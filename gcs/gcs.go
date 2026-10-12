// Package gcs reads and writes Google Cloud Storage objects as streams, with Google's own client and the
// pod's identity (Application Default Credentials), the way geoip reads its database. With
// STORAGE_EMULATOR_HOST set it talks to an emulator such as fake-gcs-server.
//
// It is built for big files split into parts, such as contest-service's teams export:
//   - Write streams an object and commits it only when the caller's write function succeeds, so a failed
//     write never leaves a partial object and never replaces the object that was there.
//   - ReadEach reads many objects at the same time, one goroutine each, and streams each one to a callback;
//     the first failure cancels the rest.
//
// Bytes are passed through untouched: an object stored as application/gzip with no Content-Encoding comes
// back as the gzip bytes, and the caller unzips it.
package gcs

import (
	"context"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	"golang.org/x/sync/errgroup"
)

// ErrNotFound is wrapped, with the object's path, in the error for an object that does not exist.
var ErrNotFound = storage.ErrObjectNotExist

// Client reads and writes objects. It is safe for concurrent use; make one per process and Close it at
// shutdown.
type Client struct {
	client *storage.Client
}

// NewClient connects with Application Default Credentials. Reads use the JSON API, which returns the same
// bytes as the default XML API and is the one fake-gcs-server serves on any host.
func NewClient(ctx context.Context) (*Client, error) {
	c, err := storage.NewClient(ctx, storage.WithJSONReads())
	if err != nil {
		return nil, fmt.Errorf("gcs: create client: %w", err)
	}
	return &Client{client: c}, nil
}

// Read downloads a whole object into memory. Use it for small objects such as a manifest; stream big ones
// with ReadEach.
func (c *Client) Read(ctx context.Context, bucket, object string) ([]byte, error) {
	var data []byte
	err := c.read(ctx, bucket, object, func(r io.Reader) error {
		var err error
		data, err = io.ReadAll(r)
		return err
	})
	return data, err
}

// ReadEach reads every object in objects at the same time, one goroutine each, and calls fn with each one's
// stream. fn runs concurrently, so whatever it shares must be safe for that. It returns the first error,
// from opening, reading or fn, with the object's path; that error cancels ctx for the reads still running.
func (c *Client) ReadEach(ctx context.Context, bucket string, objects []string, fn func(ctx context.Context, object string, r io.Reader) error) error {
	g, gctx := errgroup.WithContext(ctx)
	for _, object := range objects {
		g.Go(func() error {
			return c.read(gctx, bucket, object, func(r io.Reader) error { return fn(gctx, object, r) })
		})
	}
	return g.Wait()
}

// read opens one object and passes its stream to fn.
func (c *Client) read(ctx context.Context, bucket, object string, fn func(io.Reader) error) error {
	r, err := c.client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("gcs: open gs://%s/%s: %w", bucket, object, err)
	}
	defer r.Close()
	if err := fn(r); err != nil {
		return fmt.Errorf("gcs: read gs://%s/%s: %w", bucket, object, err)
	}
	return nil
}

// Write streams a new object: fn writes the content to w, and the object is committed only when fn returns
// nil. When fn or the upload fails, the upload is aborted: nothing is committed and an object already at
// that name is left as it was. contentType is stored on the object, with no Content-Encoding, so readers
// get back exactly the bytes fn wrote.
func (c *Client) Write(ctx context.Context, bucket, object, contentType string, fn func(w io.Writer) error) error {
	// Only a cancelled context aborts an upload: closing the writer would commit what was written so far.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	w := c.client.Bucket(bucket).Object(object).NewWriter(ctx)
	w.ContentType = contentType
	if err := fn(w); err != nil {
		cancel()
		_ = w.Close() // returns the cancellation; the callback's error is the one to report
		return fmt.Errorf("gcs: write gs://%s/%s: %w", bucket, object, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("gcs: commit gs://%s/%s: %w", bucket, object, err)
	}
	return nil
}

// Close releases the client's connections.
func (c *Client) Close() error {
	return c.client.Close()
}
