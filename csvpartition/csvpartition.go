package csvpartition

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"cloud.google.com/go/storage"
	"github.com/Faze-Technologies/go-utils/logs"
	"github.com/Faze-Technologies/go-utils/utils"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const csvProgressRows int64 = 5_000_000

// Options bounds both the number of rows held in memory and the
// number of partitions being processed at once. HasHeader excludes the first
// CSV record from the data row count and passes it to every callback.
type Options struct {
	RowsPerPartition int
	Workers          int
	HasHeader        bool
	Logger           *zap.Logger // Optional; defaults to the service logger when initialized.
}

// Result describes a complete run. On error, RowsRead and
// RowsProcessed are partial counts and must not be treated as a complete run.
type Result struct {
	Bucket        string
	Object        string
	Generation    int64
	SizeBytes     int64
	RowsRead      int64
	RowsProcessed int64
	Partitions    int64
	Duration      time.Duration
}

// Processor must return an error if any row in its partition was
// not processed. Calls can run concurrently and in a different order from the
// file. The processor owns retry and idempotency for external side effects.
type Processor func(ctx context.Context, header []string, rows [][]string) error

// ProcessGCS reads one immutable GCS object generation and processes every
// parsed CSV record in bounded partitions. GetBucket resolves the bucket name
// for the configured environment. R2 buckets are not supported.
func ProcessGCS(
	ctx context.Context,
	bucketType utils.BucketType,
	objectName string,
	opts Options,
	process Processor,
) (result Result, err error) {
	started := time.Now()
	logger := csvLogger(opts)
	defer func() {
		result.Duration = time.Since(started)
		logCSVCompletion(logger, result, err)
	}()

	if objectName == "" {
		return result, errors.New("CSV object name is required")
	}
	if bucketType == utils.BucketNFTAssets {
		return result, errors.New("NFT assets bucket is R2, not GCS")
	}
	if err := validateOptions(opts, process); err != nil {
		return result, err
	}

	bucket, err := utils.GetBucket(bucketType)
	if err != nil {
		return result, err
	}
	result.Bucket, result.Object = bucket, objectName
	logger.Info("CSV processing started",
		zap.String("bucket", bucket), zap.String("object", objectName),
		zap.Int("rows_per_partition", opts.RowsPerPartition), zap.Int("workers", opts.Workers))

	client, err := storage.NewClient(ctx)
	if err != nil {
		return result, fmt.Errorf("create GCS client: %w", err)
	}
	defer client.Close()

	object := client.Bucket(bucket).Object(objectName)
	attrs, err := object.Attrs(ctx)
	if err != nil {
		return result, fmt.Errorf("get CSV object attributes: %w", err)
	}
	result.Generation, result.SizeBytes = attrs.Generation, attrs.Size
	logger.Info("CSV object opened",
		zap.String("bucket", bucket), zap.String("object", objectName),
		zap.Int64("generation", attrs.Generation), zap.Int64("size_bytes", attrs.Size))
	reader, err := object.Generation(attrs.Generation).NewReader(ctx)
	if err != nil {
		return result, fmt.Errorf("open CSV object generation %d: %w", attrs.Generation, err)
	}
	defer reader.Close()

	return processPartitions(ctx, reader, opts, process, result, started, logger)
}

func csvLogger(opts Options) *zap.Logger {
	if opts.Logger != nil {
		return opts.Logger
	}
	if logger := logs.GetLogger(); logger != nil {
		return logger
	}
	return zap.NewNop()
}

func logCSVCompletion(logger *zap.Logger, result Result, err error) {
	fields := []zap.Field{
		zap.Int64("rows_read", result.RowsRead), zap.Int64("rows_processed", result.RowsProcessed),
		zap.Int64("partitions_completed", result.Partitions), zap.Duration("duration", result.Duration),
	}
	if result.Bucket != "" {
		fields = append(fields,
			zap.String("bucket", result.Bucket), zap.String("object", result.Object),
			zap.Int64("generation", result.Generation))
	}
	if err != nil {
		logger.Error("CSV processing failed", append(fields, zap.Error(err))...)
		return
	}
	logger.Info("CSV processing completed", fields...)
}

func validateOptions(opts Options, process Processor) error {
	if opts.RowsPerPartition < 1 || opts.Workers < 1 {
		return errors.New("RowsPerPartition and Workers must be positive")
	}
	if process == nil {
		return errors.New("CSV partition processor is required")
	}
	return nil
}

// ProcessReader processes a CSV from any reader using the same partitioning
// logic as ProcessGCS. It is useful for local files and other stream sources.
func ProcessReader(
	ctx context.Context,
	input io.Reader,
	opts Options,
	process Processor,
) (result Result, err error) {
	started := time.Now()
	logger := csvLogger(opts)
	logger.Info("CSV processing started",
		zap.Int("rows_per_partition", opts.RowsPerPartition), zap.Int("workers", opts.Workers))
	defer func() {
		result.Duration = time.Since(started)
		logCSVCompletion(logger, result, err)
	}()
	return processPartitions(ctx, input, opts, process, Result{}, started, logger)
}

func processPartitions(
	ctx context.Context,
	input io.Reader,
	opts Options,
	process Processor,
	result Result,
	started time.Time,
	logger *zap.Logger,
) (Result, error) {
	if err := validateOptions(opts, process); err != nil {
		return result, err
	}

	reader := csv.NewReader(input)
	var header []string
	if opts.HasHeader {
		var err error
		header, err = reader.Read()
		if err != nil {
			return result, fmt.Errorf("read CSV header: %w", err)
		}
		reader.FieldsPerRecord = len(header)
	}

	group, groupCtx := errgroup.WithContext(ctx)
	partitions := make(chan [][]string, opts.Workers)
	var processed, completed atomic.Int64
	var nextProgress atomic.Int64
	nextProgress.Store(csvProgressRows)
	for range opts.Workers {
		group.Go(func() error {
			for {
				select {
				case <-groupCtx.Done():
					return groupCtx.Err()
				case rows, ok := <-partitions:
					if !ok {
						return nil
					}
					if err := process(groupCtx, header, rows); err != nil {
						return fmt.Errorf("process CSV partition: %w", err)
					}
					count := processed.Add(int64(len(rows)))
					completed.Add(1)
					for {
						threshold := nextProgress.Load()
						if count < threshold {
							break
						}
						if !nextProgress.CompareAndSwap(threshold, threshold+csvProgressRows) {
							continue
						}
						logger.Info("CSV processing progress",
							zap.Int64("rows_processed", count),
							zap.Int64("partitions_completed", completed.Load()),
							zap.Duration("elapsed", time.Since(started)))
					}
				}
			}
		})
	}

	group.Go(func() error {
		defer close(partitions)
		rows := make([][]string, 0, opts.RowsPerPartition)
		for {
			record, err := reader.Read()
			if errors.Is(err, io.EOF) {
				if len(rows) == 0 {
					return nil
				}
				select {
				case partitions <- rows:
					return nil
				case <-groupCtx.Done():
					return groupCtx.Err()
				}
			}
			if err != nil {
				return fmt.Errorf("read CSV record after %d data rows: %w", result.RowsRead, err)
			}
			result.RowsRead++
			rows = append(rows, record)
			if len(rows) == opts.RowsPerPartition {
				select {
				case partitions <- rows:
					rows = make([][]string, 0, opts.RowsPerPartition)
				case <-groupCtx.Done():
					return groupCtx.Err()
				}
			}
		}
	})

	err := group.Wait()
	result.RowsProcessed = processed.Load()
	result.Partitions = completed.Load()
	result.Duration = time.Since(started)
	if err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if result.RowsRead != result.RowsProcessed {
		return result, fmt.Errorf("CSV row count mismatch: read %d, processed %d", result.RowsRead, result.RowsProcessed)
	}
	return result, nil
}
