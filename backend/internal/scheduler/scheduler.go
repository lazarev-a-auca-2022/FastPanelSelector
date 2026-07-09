// Package scheduler runs the periodic feed -> parse -> store cycle.
package scheduler

import (
	"context"
	"log/slog"
	"os"
	"time"

	"fastpanelselector/backend/internal/domain"
	"fastpanelselector/backend/internal/feed"
	"fastpanelselector/backend/internal/parse"
	"fastpanelselector/backend/internal/store"
)

type Scheduler struct {
	Source   feed.Source
	Store    store.Store
	Interval time.Duration
	MaxBytes int64
	Log      *slog.Logger
}

// Run performs one cycle immediately (so the API has data as soon as
// possible rather than waiting for the first tick), then continues on
// Interval until ctx is cancelled. Every failure at every stage is logged
// and swallowed here — a bad or missing feed file must never crash the
// process or interrupt the loop; the previous snapshot simply keeps being
// served until a cycle succeeds again.
func (s *Scheduler) Run(ctx context.Context) {
	s.runOnce(ctx)

	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context) {
	path, err := s.Source.Fetch(ctx)
	if err != nil {
		s.Log.Warn("feed fetch failed, keeping last-known-good", "err", err)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		s.Log.Warn("feed file open failed, keeping last-known-good", "path", path, "err", err)
		return
	}
	defer f.Close()

	plans, err := parse.ParseXLSX(f, s.MaxBytes)
	if err != nil {
		s.Log.Warn("feed parse failed, keeping last-known-good", "path", path, "err", err)
		return
	}

	if len(plans) == 0 {
		// A structurally valid file with zero qualifying rows is far more
		// likely to be the wrong file, or a header-only stub, than a real
		// "Hetzner has zero products" event. Don't wipe a public pricing
		// catalog on a suspicious empty parse.
		s.Log.Warn("feed parsed to zero plans, treating as suspicious and keeping last-known-good", "path", path)
		return
	}

	if err := s.Store.ReplacePlans(ctx, plans); err != nil {
		s.Log.Error("storing plans failed, keeping last-known-good", "err", err)
		return
	}

	s.Log.Info("feed refresh ok", "plan_count", len(plans), "enabled_count", countEnabled(plans))
}

func countEnabled(plans []domain.Plan) int {
	n := 0
	for _, p := range plans {
		if p.Enabled {
			n++
		}
	}
	return n
}
