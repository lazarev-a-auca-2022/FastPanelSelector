// Package store defines the persistence boundary for the plan catalog.
package store

import (
	"context"

	"fastpanelselector/backend/internal/domain"
)

// Store persists the plan catalog. Implementations must make ReplacePlans
// atomic: callers rely on a failed call leaving the previous snapshot fully
// intact (see scheduler.Scheduler for why that matters).
type Store interface {
	// ReplacePlans atomically replaces the entire catalog with plans. Each
	// feed cycle represents a full snapshot, not a delta, so this is a
	// replace rather than a merge/upsert.
	ReplacePlans(ctx context.Context, plans []domain.Plan) error

	// ListPlans returns the current catalog (enabled and disabled plans
	// alike — filtering is a frontend concern).
	ListPlans(ctx context.Context) ([]domain.Plan, error)

	Close() error
}
