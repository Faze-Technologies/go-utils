package utils

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
)

// newTestUploader connects to an S3-compatible emulator with a bucket "test-bucket", e.g.
// docker run -p 9190:9090 -e COM_ADOBE_TESTING_S3MOCK_STORE_INITIAL_BUCKETS=test-bucket adobe/s3mock:5.2.3
// and S3_TEST_ENDPOINT=http://localhost:9190.
func newTestUploader(t *testing.T) *GCSUploader {
	t.Helper()
	endpoint := os.Getenv("S3_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_TEST_ENDPOINT not set: skipping a test that needs an S3-compatible emulator")
	}
	u, err := NewGCSUploader(GCSOptions{Enable: "true", AccessKeyID: "test", SecretAccessKey: "test", Endpoint: endpoint}, false)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestReadFromBucket(t *testing.T) {
	u := newTestUploader(t)
	ctx := context.Background()
	if _, err := u.UploadDataToBucket(ctx, "test-bucket", "m1/manifest.json", []byte(`{"a":1}`), "application/json", ""); err != nil {
		t.Fatal(err)
	}
	r, err := u.ReadFromBucket(ctx, "test-bucket", "m1/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, err := io.ReadAll(r)
	if err != nil || string(got) != `{"a":1}` {
		t.Errorf("read = %q, %v; want the uploaded bytes", got, err)
	}
}

func TestReadFromBucketMissing(t *testing.T) {
	u := newTestUploader(t)
	_, err := u.ReadFromBucket(context.Background(), "test-bucket", "m1/missing.csv.gz")
	if !errors.Is(err, ErrObjectNotFound) {
		t.Errorf("read missing = %v; want ErrObjectNotFound", err)
	}
}
