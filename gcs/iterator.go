// Package gcs reads large Cloud Storage objects one partition at a time.
package gcs

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
)

// Format selects how Next splits the object.
type Format uint8

const (
	FormatBytes Format = iota + 1 // Raw bytes; partitions may split records.
	FormatCSV                     // Complete CSV records; quoted newlines stay within records.
)

// Options configures one format. Set BytesPerPartition for FormatBytes or
// RowsPerPartition for FormatCSV. CSV options have no effect in byte mode.
type Options struct {
	Format            Format
	BytesPerPartition int
	RowsPerPartition  int
	HasHeader         bool
	Comma             rune
	LazyQuotes        bool
	TrimLeadingSpace  bool
}

// Partition holds one batch. For FormatBytes, Bytes and ByteOffset are set;
// for FormatCSV, Rows and RowOffset are set. Index starts at zero. Each call to
// Next owns its returned data, so the caller may process it after another Next.
type Partition struct {
	Index      int64
	ByteOffset int64
	RowOffset  int64
	Bytes      []byte
	Rows       [][]string
}

// Result reports metadata and the number of partitions returned so far.
// Header is set only when FormatCSV and HasHeader are enabled.
type Result struct {
	Bucket        string
	Object        string
	Generation    int64
	SizeBytes     int64
	Header        []string
	BytesReturned int64
	RowsReturned  int64
	Partitions    int64
}

// Iterator reads a pinned GCS object generation. Next, Result, and Close must
// not be called concurrently. Always call Close, even if Next returns an error.
type Iterator struct {
	ctx         context.Context
	input       io.ReadCloser
	client      *storage.Client
	csvReader   *csv.Reader
	opts        Options
	result      Result
	done        bool
	closed      bool
	terminalErr error
}

// OpenPartitions opens one GCS object for streaming. bucket is the literal
// bucket name; object includes any folder prefix. The object generation is
// pinned so replacement during iteration cannot mix versions. Call Close to
// release the reader and client.
func OpenPartitions(ctx context.Context, bucket, object string, opts Options) (*Iterator, error) {
	if bucket == "" || object == "" {
		return nil, errors.New("gcs: bucket and object are required")
	}
	if err := validateOptions(opts); err != nil {
		return nil, err
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcs: create client: %w", err)
	}
	obj := client.Bucket(bucket).Object(object)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("gcs: get attributes gs://%s/%s: %w", bucket, object, err)
	}
	input, err := obj.Generation(attrs.Generation).NewReader(ctx)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("gcs: open gs://%s/%s generation=%d: %w", bucket, object, attrs.Generation, err)
	}
	result := Result{Bucket: bucket, Object: object, Generation: attrs.Generation, SizeBytes: attrs.Size}
	it, err := newIterator(ctx, input, opts, result)
	if err != nil {
		_ = input.Close()
		_ = client.Close()
		return nil, err
	}
	it.client = client
	return it, nil
}

func validateOptions(opts Options) error {
	switch opts.Format {
	case FormatBytes:
		if opts.BytesPerPartition <= 0 {
			return errors.New("gcs: BytesPerPartition must be positive for byte mode")
		}
	case FormatCSV:
		if opts.RowsPerPartition <= 0 {
			return errors.New("gcs: RowsPerPartition must be positive for CSV mode")
		}
	default:
		return errors.New("gcs: FormatBytes or FormatCSV is required")
	}
	return nil
}

func newIterator(ctx context.Context, input io.ReadCloser, opts Options, result Result) (*Iterator, error) {
	it := &Iterator{ctx: ctx, input: input, opts: opts, result: result}
	if opts.Format == FormatCSV {
		reader := csv.NewReader(input)
		if opts.Comma != 0 {
			reader.Comma = opts.Comma
		}
		reader.LazyQuotes = opts.LazyQuotes
		reader.TrimLeadingSpace = opts.TrimLeadingSpace
		if opts.HasHeader {
			header, err := reader.Read()
			if err != nil {
				return nil, fmt.Errorf("gcs: read CSV header: %w", err)
			}
			it.result.Header = header
		}
		it.csvReader = reader
	}
	return it, nil
}

// Next returns the next partition, or io.EOF after the last one. A CSV record
// is never split between partitions. Raw byte partitions can end anywhere.
func (it *Iterator) Next() (Partition, error) {
	if it.closed {
		return Partition{}, io.ErrClosedPipe
	}
	if it.terminalErr != nil {
		return Partition{}, it.terminalErr
	}
	if it.done {
		return Partition{}, io.EOF
	}
	if err := it.ctx.Err(); err != nil {
		it.terminalErr = err
		return Partition{}, err
	}
	if it.opts.Format == FormatBytes {
		return it.nextBytes()
	}
	return it.nextCSV()
}

func (it *Iterator) nextBytes() (Partition, error) {
	buf := make([]byte, it.opts.BytesPerPartition)
	n, err := io.ReadFull(it.input, buf)
	if errors.Is(err, io.EOF) {
		it.done = true
		return Partition{}, io.EOF
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		it.done = true
	} else if err != nil {
		it.terminalErr = fmt.Errorf("gcs: read bytes at offset %d: %w", it.result.BytesReturned, err)
		return Partition{}, it.terminalErr
	}
	part := Partition{Index: it.result.Partitions, ByteOffset: it.result.BytesReturned, Bytes: buf[:n]}
	it.result.BytesReturned += int64(n)
	it.result.Partitions++
	return part, nil
}

func (it *Iterator) nextCSV() (Partition, error) {
	rows := make([][]string, 0, it.opts.RowsPerPartition)
	for len(rows) < it.opts.RowsPerPartition {
		if err := it.ctx.Err(); err != nil {
			it.terminalErr = err
			return Partition{}, err
		}
		row, err := it.csvReader.Read()
		if errors.Is(err, io.EOF) {
			it.done = true
			break
		}
		if err != nil {
			it.terminalErr = fmt.Errorf("gcs: read CSV after %d returned rows: %w", it.result.RowsReturned, err)
			return Partition{}, it.terminalErr
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return Partition{}, io.EOF
	}
	part := Partition{Index: it.result.Partitions, RowOffset: it.result.RowsReturned, Rows: rows}
	it.result.RowsReturned += int64(len(rows))
	it.result.Partitions++
	return part, nil
}

// Result returns a snapshot of metadata and progress. Its Header is copied.
func (it *Iterator) Result() Result {
	result := it.result
	result.Header = append([]string(nil), result.Header...)
	return result
}

// Close releases the GCS reader and client. It is safe to call more than once.
func (it *Iterator) Close() error {
	if it.closed {
		return nil
	}
	it.closed = true
	err := it.input.Close()
	if it.client != nil {
		err = errors.Join(err, it.client.Close())
	}
	return err
}
