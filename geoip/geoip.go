// Package geoip provides MaxMind-backed IP geolocation as a Gin middleware
// plus an outbound resty interceptor. It is opt-in: importing the package
// has no side effects until Init is called.
//
// The .mmdb files are loaded from a local directory. If GCSBucket is set the
// package will pull the latest .mmdb files from gs://<bucket>/<prefix>* on
// startup and refresh them periodically. Atomic file rename + atomic.Pointer
// reader swap means lookups are never served against a half-written database.
package geoip

import (
	"context"

	"go.uber.org/zap"
)

// Service is the package handle returned by Init. It exposes the middleware,
// transport hook, and direct lookup methods.
type Service struct {
	config    Config
	engine    *lookupEngine
	refresher *refresher
	logger    *zap.Logger
}

// Init constructs and starts a Service. Pass overrides via opts; any field
// left at the zero value falls back to env vars (GEOIP_*) and then to the
// package defaults.
//
// Init is non-fatal: if the .mmdb files cannot be loaded or the bucket cannot
// be reached, the returned service still works — lookups just return
// {LookupStatus: disabled} until the next successful refresh.
func Init(ctx context.Context, logger *zap.Logger, opts Config) (*Service, error) {
	cfg := defaultConfig()
	if opts.DBPath != "" {
		cfg.DBPath = opts.DBPath
	}
	if opts.GCSBucket != "" {
		cfg.GCSBucket = opts.GCSBucket
	}
	if opts.GCSPrefix != "" {
		cfg.GCSPrefix = opts.GCSPrefix
	}
	if opts.RefreshInterval > 0 {
		cfg.RefreshInterval = opts.RefreshInterval
	}
	if opts.HeaderPrefix != "" {
		cfg.HeaderPrefix = opts.HeaderPrefix
	}
	cfg = mergeFromEnv(cfg)

	s := &Service{config: cfg, engine: newLookupEngine(logger), logger: logger}

	if !cfg.Enabled {
		logger.Info("[GEOIP] disabled via config")
		return s, nil
	}

	if cfg.GCSBucket != "" {
		r, err := newRefresher(ctx, cfg.GCSBucket, cfg.GCSPrefix, cfg.DBPath, cfg.RefreshInterval, logger,
			func(c context.Context) error { return s.engine.loadFromDirectory(cfg.DBPath) })
		if err != nil {
			logger.Error("[GEOIP] refresher init failed; continuing without GCS sync", zap.Error(err))
		} else {
			s.refresher = r
			if err := r.start(ctx); err != nil {
				logger.Error("[GEOIP] refresher start failed", zap.Error(err))
			}
		}
	} else {
		if err := s.engine.loadFromDirectory(cfg.DBPath); err != nil {
			logger.Error("[GEOIP] initial load failed", zap.Error(err))
		}
	}

	return s, nil
}

// Lookup performs a direct lookup against the loaded readers, bypassing the
// middleware. Useful for background jobs that need to enrich data by IP.
func (s *Service) Lookup(ip string) GeoResult {
	return s.engine.lookup(ip)
}

// Ready reports whether at least one .mmdb file is currently loaded.
func (s *Service) Ready() bool {
	if !s.config.Enabled {
		return false
	}
	return s.engine.ready()
}

// Stop halts the background refresher and releases the storage client.
func (s *Service) Stop() {
	if s.refresher != nil {
		s.refresher.stop()
	}
}
