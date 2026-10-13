// Package csvpartition streams CSV records into bounded partitions for
// concurrent processing. It parses records before partitioning, so quoted
// newlines never split a record. A successful call means every parsed data
// record was delivered to a processor whose call returned nil.
//
// ProcessReader accepts any io.Reader. ProcessGCS resolves the bucket with
// utils.GetBucket for the configured environment and pins the object generation
// before reading. Both return row counts and elapsed time. A failed call may
// have completed some partitions; processors with external side effects should
// be idempotent if a caller plans to retry the run.
//
// For example:
//
//	result, err := csvpartition.ProcessGCS(ctx, utils.BucketContent, "ranks.csv",
//		csvpartition.Options{RowsPerPartition: 10_000, Workers: 4, HasHeader: true},
//		func(ctx context.Context, header []string, rows [][]string) error {
//			// Validate or process all rows in this partition.
//			return nil
//		})
//	_ = result
//	_ = err
package csvpartition
