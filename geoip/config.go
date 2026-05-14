package geoip

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDBPath          = "./.geoip"
	defaultGCSPrefix       = "current/"
	defaultRefreshInterval = 6 * time.Hour
	defaultHeaderPrefix    = "X-Fc-Geo-"
)

func defaultConfig() Config {
	return Config{
		Enabled:         true,
		DBPath:          defaultDBPath,
		GCSPrefix:       defaultGCSPrefix,
		RefreshInterval: defaultRefreshInterval,
		HeaderPrefix:    defaultHeaderPrefix,
	}
}

func parseBool(raw string, fallback bool) bool {
	if raw == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}

func parseDurationMs(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return time.Duration(n) * time.Millisecond
}

func mergeFromEnv(cfg Config) Config {
	cfg.Enabled = parseBool(os.Getenv("GEOIP_ENABLED"), cfg.Enabled)
	if v := os.Getenv("GEOIP_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("GEOIP_GCS_BUCKET"); v != "" {
		cfg.GCSBucket = v
	}
	if v := os.Getenv("GEOIP_GCS_PREFIX"); v != "" {
		cfg.GCSPrefix = v
	}
	cfg.RefreshInterval = parseDurationMs(os.Getenv("GEOIP_REFRESH_INTERVAL_MS"), cfg.RefreshInterval)
	if v := os.Getenv("GEOIP_HEADER_PREFIX"); v != "" {
		cfg.HeaderPrefix = v
	}
	return cfg
}
