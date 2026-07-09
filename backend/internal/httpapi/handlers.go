// Package httpapi exposes the plan catalog over HTTP.
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"fastpanelselector/backend/internal/domain"
)

// planLister is the narrow read interface these handlers need — satisfied
// by store.Store, kept separate here so handler tests can use a trivial fake
// without depending on the store package.
type planLister interface {
	ListPlans(ctx context.Context) ([]domain.Plan, error)
}

type Handlers struct {
	Store planLister
	Log   *slog.Logger
}

// GetPlans returns the full catalog (enabled and disabled alike) — the
// frontend already does its own enabled-filtering client-side, so the API
// mirrors exactly what plans-data.js used to hold statically.
func (h *Handlers) GetPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := h.Store.ListPlans(r.Context())
	if err != nil {
		h.Log.Error("list plans failed", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(plans); err != nil {
		// Headers are already sent at this point; nothing more to do but log.
		h.Log.Error("encoding plans response failed", "err", err)
	}
}

func (h *Handlers) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
