// Package config loads and validates the service's environment-driven
// configuration. There are no secrets in this service's config surface
// today (SQLite is a local file, no external calls are made) — if a future
// datastore needs credentials, they must come from env/secret store here,
// never a literal in code.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPPort     int
	DBPath       string
	FeedPath     string
	PollInterval time.Duration
	MaxFeedBytes int64
	LogLevel     slog.Level
}

func Load() (Config, error) {
	cfg := Config{
		HTTPPort:     8080,
		DBPath:       "./data/plans.db",
		FeedPath:     "./data/feed.xlsx",
		PollInterval: 15 * time.Minute,
		MaxFeedBytes: 10 * 1024 * 1024,
		LogLevel:     slog.LevelInfo,
	}

	if v, ok := os.LookupEnv("HTTP_PORT"); ok {
		port, err := strconv.Atoi(v)
		if err != nil || port <= 0 || port > 65535 {
			return Config{}, fmt.Errorf("config: invalid HTTP_PORT %q", v)
		}
		cfg.HTTPPort = port
	}

	if v, ok := os.LookupEnv("DB_PATH"); ok && v != "" {
		cfg.DBPath = v
	}

	if v, ok := os.LookupEnv("FEED_PATH"); ok && v != "" {
		cfg.FeedPath = v
	}

	if v, ok := os.LookupEnv("POLL_INTERVAL"); ok {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("config: invalid POLL_INTERVAL %q: %w", v, err)
		}
		cfg.PollInterval = d
	}

	if v, ok := os.LookupEnv("MAX_FEED_BYTES"); ok {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return Config{}, fmt.Errorf("config: invalid MAX_FEED_BYTES %q", v)
		}
		cfg.MaxFeedBytes = n
	}

	if v, ok := os.LookupEnv("LOG_LEVEL"); ok && v != "" {
		var lvl slog.Level
		if err := lvl.UnmarshalText([]byte(v)); err != nil {
			return Config{}, fmt.Errorf("config: invalid LOG_LEVEL %q: %w", v, err)
		}
		cfg.LogLevel = lvl
	}

	return cfg, nil
}
