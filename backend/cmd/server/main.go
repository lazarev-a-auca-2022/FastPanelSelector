// Command server runs the plan-feed pipeline: it periodically parses a
// Hetzner-style pricing spreadsheet into SQLite and serves the result over
// HTTP for the frontend selector to consume.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fastpanelselector/backend/internal/config"
	"fastpanelselector/backend/internal/feed"
	"fastpanelselector/backend/internal/httpapi"
	"fastpanelselector/backend/internal/scheduler"
	"fastpanelselector/backend/internal/store/sqlite"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	db, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sched := &scheduler.Scheduler{
		Source:   feed.NewLocalFileSource(cfg.FeedPath),
		Store:    db,
		Interval: cfg.PollInterval,
		MaxBytes: cfg.MaxFeedBytes,
		Log:      log,
	}
	go sched.Run(ctx)

	handlers := &httpapi.Handlers{Store: db, Log: log}
	srv := httpapi.NewServer(httpapi.ServerConfig{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		StaticDir:         cfg.StaticDir,
	}, handlers, log)

	serveErr := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
		close(serveErr)
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down http server: %w", err)
	}

	return nil
}
