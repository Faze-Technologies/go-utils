package geoip

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"go.uber.org/zap"
	"google.golang.org/api/iterator"
)

type objectState struct {
	generation int64
	updated    time.Time
}

type refresher struct {
	bucket   string
	prefix   string
	dbPath   string
	interval time.Duration
	logger   *zap.Logger
	onUpdate func(context.Context) error

	client   *storage.Client
	known    map[string]objectState
	mu       sync.Mutex
	stopCh   chan struct{}
	stopOnce sync.Once
}

func newRefresher(
	ctx context.Context,
	bucket, prefix, dbPath string,
	interval time.Duration,
	logger *zap.Logger,
	onUpdate func(context.Context) error,
) (*refresher, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %w", err)
	}
	if err := os.MkdirAll(dbPath, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dbPath, err)
	}
	return &refresher{
		bucket:   bucket,
		prefix:   prefix,
		dbPath:   dbPath,
		interval: interval,
		logger:   logger,
		onUpdate: onUpdate,
		client:   client,
		known:    map[string]objectState{},
		stopCh:   make(chan struct{}),
	}, nil
}

func (r *refresher) syncOnce(ctx context.Context) (bool, error) {
	bucket := r.client.Bucket(r.bucket)
	it := bucket.Objects(ctx, &storage.Query{Prefix: r.prefix})

	var seenAny bool
	updated := false
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return updated, fmt.Errorf("list: %w", err)
		}
		seenAny = true
		if !strings.HasSuffix(attrs.Name, ".mmdb") {
			continue
		}

		baseName := filepath.Base(attrs.Name)
		localFinal := filepath.Join(r.dbPath, baseName)

		r.mu.Lock()
		prev, seen := r.known[baseName]
		r.mu.Unlock()
		if seen && prev.generation == attrs.Generation && prev.updated.Equal(attrs.Updated) {
			if _, err := os.Stat(localFinal); err == nil {
				continue
			}
		}

		localTmp := fmt.Sprintf("%s.tmp-%d", localFinal, time.Now().UnixNano())
		if err := r.downloadObject(ctx, bucket.Object(attrs.Name), localTmp); err != nil {
			_ = os.Remove(localTmp)
			return updated, fmt.Errorf("download %s: %w", attrs.Name, err)
		}
		if err := os.Rename(localTmp, localFinal); err != nil {
			_ = os.Remove(localTmp)
			return updated, fmt.Errorf("rename %s: %w", baseName, err)
		}

		r.mu.Lock()
		r.known[baseName] = objectState{generation: attrs.Generation, updated: attrs.Updated}
		r.mu.Unlock()

		updated = true
		r.logger.Info("[GEOIP] refreshed db file",
			zap.String("file", baseName),
			zap.Int64("generation", attrs.Generation))
	}

	if !seenAny {
		r.logger.Warn("[GEOIP] no objects found in bucket prefix",
			zap.String("bucket", r.bucket),
			zap.String("prefix", r.prefix))
	}
	return updated, nil
}

func (r *refresher) downloadObject(ctx context.Context, obj *storage.ObjectHandle, dst string) error {
	rc, err := obj.NewReader(ctx)
	if err != nil {
		return err
	}
	defer rc.Close()
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, rc); err != nil {
		return err
	}
	return nil
}

func (r *refresher) start(ctx context.Context) error {
	// Bound the boot sync so a flaky GCS connection can't block pod startup
	// indefinitely. The recurring loop uses its own 5-minute timeout.
	bootCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	changed, err := r.syncOnce(bootCtx)
	if err != nil {
		return err
	}
	if changed && r.onUpdate != nil {
		if err := r.onUpdate(bootCtx); err != nil {
			return fmt.Errorf("onUpdate: %w", err)
		}
	}
	go r.loop()
	return nil
}

func (r *refresher) loop() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			changed, err := r.syncOnce(ctx)
			if err != nil {
				r.logger.Error("[GEOIP] refresh cycle failed", zap.Error(err))
				cancel()
				continue
			}
			if changed && r.onUpdate != nil {
				if err := r.onUpdate(ctx); err != nil {
					r.logger.Error("[GEOIP] onUpdate failed", zap.Error(err))
				}
			}
			cancel()
		}
	}
}

func (r *refresher) stop() {
	r.stopOnce.Do(func() {
		close(r.stopCh)
		_ = r.client.Close()
	})
}
