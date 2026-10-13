package csvpartition

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestProcessReaderProcessesEveryRecordOnce(t *testing.T) {
	var csvText strings.Builder
	csvText.WriteString("id,value\r\n")
	for i := range 1001 {
		fmt.Fprintf(&csvText, "%d,\"value %d with\r\na newline\"\r\n", i, i)
	}

	var mu sync.Mutex
	seen := make(map[string]int)
	result, err := ProcessReader(
		context.Background(), strings.NewReader(csvText.String()),
		Options{RowsPerPartition: 17, Workers: 4, HasHeader: true},
		func(_ context.Context, header []string, rows [][]string) error {
			if len(header) != 2 || header[0] != "id" || header[1] != "value" {
				return fmt.Errorf("unexpected header: %v", header)
			}
			mu.Lock()
			defer mu.Unlock()
			for _, row := range rows {
				if len(row) != 2 || !strings.Contains(row[1], "with\na newline") {
					return fmt.Errorf("unexpected row: %v", row)
				}
				seen[row[0]]++
			}
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsRead != 1001 || result.RowsProcessed != 1001 || result.Partitions != 59 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(seen) != 1001 {
		t.Fatalf("got %d unique rows, want 1001", len(seen))
	}
	for i := range 1001 {
		if seen[fmt.Sprint(i)] != 1 {
			t.Fatalf("row %d processed %d times", i, seen[fmt.Sprint(i)])
		}
	}
}

func TestProcessReaderReportsMalformedCSV(t *testing.T) {
	result, err := ProcessReader(
		context.Background(), strings.NewReader("id,value\n1,ok\n2,\"unfinished"),
		Options{RowsPerPartition: 1, Workers: 2, HasHeader: true},
		func(context.Context, []string, [][]string) error { return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "read CSV record") {
		t.Fatalf("expected CSV parse error, got %v", err)
	}
	if result.RowsRead != 1 {
		t.Fatalf("read %d valid rows, want 1", result.RowsRead)
	}
}

func TestProcessReaderReportsProcessorError(t *testing.T) {
	want := errors.New("database write failed")
	result, err := ProcessReader(
		context.Background(), strings.NewReader("id\n1\n2\n3\n"),
		Options{RowsPerPartition: 1, Workers: 2, HasHeader: true},
		func(context.Context, []string, [][]string) error { return want },
	)
	if !errors.Is(err, want) {
		t.Fatalf("expected processor error, got %v", err)
	}
	if result.RowsProcessed != 0 {
		t.Fatalf("reported %d processed rows after callback error", result.RowsProcessed)
	}
}
