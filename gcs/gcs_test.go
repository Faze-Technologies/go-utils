package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient connects to fake-gcs-server and creates a fresh bucket. Run one with
// docker run -p 4443:4443 fsouza/fake-gcs-server -scheme http, then STORAGE_EMULATOR_HOST=localhost:4443.
func newTestClient(t *testing.T) (*Client, string) {
	t.Helper()
	if os.Getenv("STORAGE_EMULATOR_HOST") == "" {
		t.Skip("STORAGE_EMULATOR_HOST not set: skipping a test that needs fake GCS")
	}
	ctx := context.Background()
	c, err := NewClient(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	bucket := fmt.Sprintf("gcs-test-%d", time.Now().UnixNano())
	if err := c.client.Bucket(bucket).Create(ctx, "test-project", nil); err != nil {
		t.Fatal(err)
	}
	return c, bucket
}

func write(t *testing.T, c *Client, bucket, object, body string) {
	t.Helper()
	err := c.Write(context.Background(), bucket, object, "text/plain", func(w io.Writer) error {
		_, err := io.WriteString(w, body)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWriteThenRead(t *testing.T) {
	c, bucket := newTestClient(t)
	ctx := context.Background()
	err := c.Write(ctx, bucket, "m1/part-0.csv.gz", "application/gzip", func(w io.Writer) error {
		for _, line := range []string{"line 1\n", "line 2\n"} {
			if _, err := io.WriteString(w, line); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Read(ctx, bucket, "m1/part-0.csv.gz")
	if err != nil || string(got) != "line 1\nline 2\n" {
		t.Fatalf("Read = %q, %v", got, err)
	}
	attrs, err := c.client.Bucket(bucket).Object("m1/part-0.csv.gz").Attrs(ctx)
	if err != nil || attrs.ContentType != "application/gzip" || attrs.ContentEncoding != "" {
		t.Errorf("attrs = %+v, %v; want application/gzip with no Content-Encoding", attrs, err)
	}
}

// A failed write leaves nothing behind, and an object it was replacing stays as it was.
func TestWriteErrorCommitsNothing(t *testing.T) {
	c, bucket := newTestClient(t)
	ctx := context.Background()
	write(t, c, bucket, "kept", "old")
	boom := errors.New("boom")
	for _, object := range []string{"new", "kept"} {
		err := c.Write(ctx, bucket, object, "text/plain", func(w io.Writer) error {
			if _, err := io.WriteString(w, "partial"); err != nil {
				return err
			}
			return boom
		})
		if !errors.Is(err, boom) {
			t.Errorf("Write %s = %v, want the callback's error", object, err)
		}
	}
	if _, err := c.Read(ctx, bucket, "new"); !errors.Is(err, ErrNotFound) {
		t.Errorf("failed new object: Read = %v, want ErrNotFound", err)
	}
	if got, err := c.Read(ctx, bucket, "kept"); err != nil || string(got) != "old" {
		t.Errorf("failed overwrite: Read = %q, %v; want the old object", got, err)
	}
}

func TestReadMissing(t *testing.T) {
	c, bucket := newTestClient(t)
	_, err := c.Read(context.Background(), bucket, "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Read missing = %v, want ErrNotFound", err)
	}
}

func TestReadEach(t *testing.T) {
	c, bucket := newTestClient(t)
	objects := []string{"m1/a", "m1/b", "m1/c"}
	for _, o := range objects {
		write(t, c, bucket, o, "body of "+o)
	}
	var mu sync.Mutex
	got := map[string]string{}
	err := c.ReadEach(context.Background(), bucket, objects, func(_ context.Context, object string, r io.Reader) error {
		b, err := io.ReadAll(r)
		mu.Lock()
		got[object] = string(b)
		mu.Unlock()
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range objects {
		if got[o] != "body of "+o {
			t.Errorf("%s: got %q", o, got[o])
		}
	}
}

// The objects are read at the same time: every callback is running before any returns.
func TestReadEachRunsInParallel(t *testing.T) {
	c, bucket := newTestClient(t)
	objects := []string{"p/a", "p/b", "p/c"}
	for _, o := range objects {
		write(t, c, bucket, o, "x")
	}
	var started sync.WaitGroup
	started.Add(len(objects))
	all := make(chan struct{})
	go func() { started.Wait(); close(all) }()
	err := c.ReadEach(context.Background(), bucket, objects, func(ctx context.Context, _ string, _ io.Reader) error {
		started.Done()
		select {
		case <-all:
			return nil
		case <-time.After(5 * time.Second):
			return errors.New("the other reads did not start: not parallel")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The first failure is returned, names its object, and cancels the reads still running.
func TestReadEachFirstErrorCancelsTheRest(t *testing.T) {
	c, bucket := newTestClient(t)
	write(t, c, bucket, "e/slow", "x")
	write(t, c, bucket, "e/bad", "x")
	boom := errors.New("bad row")
	slowStarted := make(chan struct{})
	cancelled := make(chan bool, 1)
	err := c.ReadEach(context.Background(), bucket, []string{"e/slow", "e/bad"}, func(ctx context.Context, object string, _ io.Reader) error {
		if object == "e/bad" {
			<-slowStarted // fail only once the slow read is running, so there is a read to cancel
			return boom
		}
		close(slowStarted)
		select {
		case <-ctx.Done():
			cancelled <- true
		case <-time.After(5 * time.Second):
			cancelled <- false
		}
		return ctx.Err()
	})
	if !errors.Is(err, boom) {
		t.Fatalf("ReadEach = %v, want the callback's error", err)
	}
	if want := "e/bad"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not name %s", err, want)
	}
	if !<-cancelled {
		t.Error("the slow read was not cancelled")
	}
}

func TestReadEachMissingObject(t *testing.T) {
	c, bucket := newTestClient(t)
	write(t, c, bucket, "n/a", "x")
	err := c.ReadEach(context.Background(), bucket, []string{"n/a", "n/missing"}, func(_ context.Context, _ string, r io.Reader) error {
		_, err := io.Copy(io.Discard, r)
		return err
	})
	if !errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "n/missing") {
		t.Errorf("ReadEach = %v, want ErrNotFound naming n/missing", err)
	}
}

// A transient failure during an upload is retried: every Write replaces the whole object, so repeating it is
// safe. A proxy in front of fake GCS answers the first upload request with 503.
func TestWriteRetriesTransientFailure(t *testing.T) {
	if os.Getenv("STORAGE_EMULATOR_HOST") == "" {
		t.Skip("STORAGE_EMULATOR_HOST not set: skipping a test that needs fake GCS")
	}
	_, bucket := newTestClient(t)
	target, err := url.Parse("http://" + os.Getenv("STORAGE_EMULATOR_HOST"))
	if err != nil {
		t.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	var failed atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/upload/") && failed.CompareAndSwap(false, true) {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	defer srv.Close()
	t.Setenv("STORAGE_EMULATOR_HOST", strings.TrimPrefix(srv.URL, "http://"))
	c, err := NewClient(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	write(t, c, bucket, "retried", "body")
	if !failed.Load() {
		t.Fatal("the proxy never failed an upload: the test did not exercise a retry")
	}
	if got, err := c.Read(context.Background(), bucket, "retried"); err != nil || string(got) != "body" {
		t.Errorf("Read = %q, %v; want the retried upload", got, err)
	}
}
