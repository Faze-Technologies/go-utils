package gcs

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestByteIteratorReturnsEveryByteOnce(t *testing.T) {
	data := []byte{'a', 0, '\n', 'b', 'c', 'd', 'e', 'f', 'g'}
	it, err := newIterator(context.Background(), io.NopCloser(bytes.NewReader(data)),
		Options{Format: FormatBytes, BytesPerPartition: 4},
		Result{Bucket: "bucket", Object: "folder/file", Generation: 17, SizeBytes: int64(len(data))})
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close()
	var got []byte
	for index := int64(0); ; index++ {
		part, err := it.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if part.Index != index || part.ByteOffset != int64(len(got)) || len(part.Rows) != 0 {
			t.Fatalf("unexpected partition: %+v", part)
		}
		got = append(got, part.Bytes...)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("got %v, want %v", got, data)
	}
	result := it.Result()
	if result.BytesReturned != int64(len(data)) || result.Partitions != 3 || result.Generation != 17 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := it.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected repeated EOF, got %v", err)
	}
}

func TestCSVIteratorPreservesQuotedNewlinesAndHeader(t *testing.T) {
	var input strings.Builder
	w := csv.NewWriter(&input)
	if err := w.Write([]string{"id", "value"}); err != nil {
		t.Fatal(err)
	}
	for i := range 101 {
		if err := w.Write([]string{fmt.Sprint(i), fmt.Sprintf("line one\nline two %d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		t.Fatal(err)
	}
	it, err := newIterator(context.Background(), io.NopCloser(strings.NewReader(input.String())),
		Options{Format: FormatCSV, RowsPerPartition: 9, HasHeader: true}, Result{SizeBytes: int64(input.Len())})
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close()
	header := it.Result().Header
	if len(header) != 2 || header[0] != "id" || header[1] != "value" {
		t.Fatalf("unexpected header: %v", header)
	}
	header[0] = "changed"
	if it.Result().Header[0] != "id" {
		t.Fatal("Result exposed mutable header")
	}
	var seen int64
	for {
		part, err := it.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if part.RowOffset != seen || part.Index != seen/9 || len(part.Rows) > 9 {
			t.Fatalf("unexpected partition: %+v", part)
		}
		for _, row := range part.Rows {
			if row[0] != fmt.Sprint(seen) || row[1] != fmt.Sprintf("line one\nline two %d", seen) {
				t.Fatalf("unexpected row %d: %v", seen, row)
			}
			seen++
		}
	}
	result := it.Result()
	if result.RowsReturned != 101 || result.Partitions != 12 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCSVIteratorReturnsParseError(t *testing.T) {
	it, err := newIterator(context.Background(), io.NopCloser(strings.NewReader("id,value\n1,ok\n2,\"unfinished")),
		Options{Format: FormatCSV, RowsPerPartition: 1, HasHeader: true}, Result{})
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close()
	if _, err := it.Next(); err != nil {
		t.Fatal(err)
	}
	if _, err := it.Next(); err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("expected parse error, got %v", err)
	}
	if it.Result().RowsReturned != 1 {
		t.Fatalf("unexpected result: %+v", it.Result())
	}
}

func TestIteratorEmptyInputAndClose(t *testing.T) {
	it, err := newIterator(context.Background(), io.NopCloser(strings.NewReader("")),
		Options{Format: FormatBytes, BytesPerPartition: 4}, Result{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := it.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
	if err := it.Close(); err != nil {
		t.Fatal(err)
	}
	if err := it.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := it.Next(); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("expected closed error, got %v", err)
	}
}

func TestOpenPartitionsValidatesBeforeConnecting(t *testing.T) {
	_, err := OpenPartitions(context.Background(), "bucket", "object", Options{Format: FormatBytes})
	if err == nil || !strings.Contains(err.Error(), "BytesPerPartition") {
		t.Fatalf("expected byte partition size error, got %v", err)
	}
	_, err = OpenPartitions(context.Background(), "bucket", "object", Options{Format: FormatCSV})
	if err == nil || !strings.Contains(err.Error(), "RowsPerPartition") {
		t.Fatalf("expected row partition size error, got %v", err)
	}
}
