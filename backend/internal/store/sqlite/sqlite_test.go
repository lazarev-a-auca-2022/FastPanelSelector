package sqlite

import (
	"context"
	"testing"

	"fastpanelselector/backend/internal/domain"
)

func f(v float64) *float64 { return &v }

func TestReplaceAndListRoundTrip(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	initial, err := s.ListPlans(ctx)
	if err != nil {
		t.Fatalf("ListPlans (empty): %v", err)
	}
	if len(initial) != 0 {
		t.Fatalf("expected empty store, got %d plans", len(initial))
	}

	plans := []domain.Plan{
		{ID: "1", Location: "DE", City: "Falkenstein", Package: "A", Arch: "x86", CPUType: "shared", Cores: 2, RAM: 4, Disk: 40, Enabled: true, Price: f(6.48)},
		{ID: "2", Location: "DE", City: "Falkenstein", Package: "B", Arch: "x86", CPUType: "shared", Cores: 2, RAM: 2, Disk: 40, Enabled: false, Price: nil},
	}
	if err := s.ReplacePlans(ctx, plans); err != nil {
		t.Fatalf("ReplacePlans: %v", err)
	}

	got, err := s.ListPlans(ctx)
	if err != nil {
		t.Fatalf("ListPlans: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	byID := map[string]domain.Plan{}
	for _, p := range got {
		byID[p.ID] = p
	}

	p1 := byID["1"]
	if !p1.Enabled || p1.Price == nil || *p1.Price != 6.48 {
		t.Errorf("plan 1 = %+v, want enabled with price 6.48", p1)
	}
	p2 := byID["2"]
	if p2.Enabled || p2.Price != nil {
		t.Errorf("plan 2 = %+v, want disabled with nil price", p2)
	}
}

func TestReplacePlansFullyReplacesSnapshot(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	first := []domain.Plan{
		{ID: "1", Location: "DE", City: "Falkenstein", Package: "A", Arch: "x86", CPUType: "shared", Cores: 2, RAM: 4, Disk: 40, Enabled: true, Price: f(6.48)},
	}
	if err := s.ReplacePlans(ctx, first); err != nil {
		t.Fatalf("ReplacePlans(first): %v", err)
	}

	// Second snapshot drops id=1 entirely and introduces id=2 — simulating a
	// Hetzner SKU disappearing between polls.
	second := []domain.Plan{
		{ID: "2", Location: "FI", City: "Helsinki", Package: "B", Arch: "arm", CPUType: "shared", Cores: 4, RAM: 8, Disk: 80, Enabled: true, Price: f(12.38)},
	}
	if err := s.ReplacePlans(ctx, second); err != nil {
		t.Fatalf("ReplacePlans(second): %v", err)
	}

	got, err := s.ListPlans(ctx)
	if err != nil {
		t.Fatalf("ListPlans: %v", err)
	}
	if len(got) != 1 || got[0].ID != "2" {
		t.Fatalf("got %+v, want only plan id=2", got)
	}
}

func TestListPlansSurvivesEmptyReplace(t *testing.T) {
	// Documents the store's own behavior in isolation: calling ReplacePlans
	// with an empty slice does wipe the table. The scheduler is the layer
	// responsible for deciding *not* to call ReplacePlans on a suspicious
	// empty parse — the store itself just does what it's told.
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	seed := []domain.Plan{
		{ID: "1", Location: "DE", City: "Falkenstein", Package: "A", Arch: "x86", CPUType: "shared", Cores: 2, RAM: 4, Disk: 40, Enabled: true, Price: f(6.48)},
	}
	if err := s.ReplacePlans(ctx, seed); err != nil {
		t.Fatalf("ReplacePlans(seed): %v", err)
	}
	if err := s.ReplacePlans(ctx, []domain.Plan{}); err != nil {
		t.Fatalf("ReplacePlans(empty): %v", err)
	}

	got, err := s.ListPlans(ctx)
	if err != nil {
		t.Fatalf("ListPlans: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}
}
