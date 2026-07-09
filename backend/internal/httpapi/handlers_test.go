package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fastpanelselector/backend/internal/domain"
)

type fakeLister struct {
	plans []domain.Plan
	err   error
}

func (f *fakeLister) ListPlans(_ context.Context) ([]domain.Plan, error) {
	return f.plans, f.err
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func priceOf(v float64) *float64 { return &v }

func TestGetPlans_ReturnsCatalogAsJSON(t *testing.T) {
	h := &Handlers{
		Store: &fakeLister{plans: []domain.Plan{
			{ID: "1", Location: "DE", Enabled: true, Price: priceOf(6.48)},
			{ID: "2", Location: "DE", Enabled: false, Price: nil},
		}},
		Log: testLogger(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/plans", nil)
	rec := httptest.NewRecorder()
	h.GetPlans(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got []domain.Plan
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not valid JSON: %v\nbody: %s", err, rec.Body.String())
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[1].Price != nil {
		t.Errorf("disabled plan price = %v, want null", *got[1].Price)
	}
}

func TestGetPlans_StoreErrorReturnsGenericMessage(t *testing.T) {
	h := &Handlers{
		Store: &fakeLister{err: errors.New("disk on fire, path=/secret/internal/plans.db")},
		Log:   testLogger(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/plans", nil)
	rec := httptest.NewRecorder()
	h.GetPlans(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	if got := body; got == "" {
		t.Fatal("expected a response body")
	}
	if want := "disk on fire"; strings.Contains(body, want) {
		t.Errorf("response body leaked internal error detail: %q", body)
	}
}

func TestHealthz(t *testing.T) {
	h := &Handlers{Log: testLogger()}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.Healthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != `{"status":"ok"}` {
		t.Errorf("body = %q", rec.Body.String())
	}
}
