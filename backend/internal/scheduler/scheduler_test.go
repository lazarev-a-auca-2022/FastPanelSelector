package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"fastpanelselector/backend/internal/domain"
)

const realFeedFixture = "../parse/testdata/cloud_07_07_2026.xlsx"
const emptyFeedFixture = "../parse/testdata/empty.xlsx"

type fakeSource struct {
	path string
	err  error
}

func (f *fakeSource) Fetch(_ context.Context) (string, error) {
	return f.path, f.err
}

type fakeStore struct {
	plans        []domain.Plan
	replaceErr   error
	replaceCalls int
}

func (s *fakeStore) ReplacePlans(_ context.Context, plans []domain.Plan) error {
	s.replaceCalls++
	if s.replaceErr != nil {
		return s.replaceErr
	}
	s.plans = plans
	return nil
}

func (s *fakeStore) ListPlans(_ context.Context) ([]domain.Plan, error) {
	return s.plans, nil
}

func (s *fakeStore) Close() error { return nil }

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRunOnce_SuccessfulCycleStoresPlans(t *testing.T) {
	fs := &fakeStore{}
	s := &Scheduler{
		Source:   &fakeSource{path: realFeedFixture},
		Store:    fs,
		MaxBytes: 10 * 1024 * 1024,
		Log:      testLogger(),
	}

	s.runOnce(context.Background())

	if fs.replaceCalls != 1 {
		t.Fatalf("replaceCalls = %d, want 1", fs.replaceCalls)
	}
	if len(fs.plans) != 112 {
		t.Errorf("stored %d plans, want 112", len(fs.plans))
	}
}

func TestRunOnce_FetchFailureKeepsLastKnownGood(t *testing.T) {
	fs := &fakeStore{plans: []domain.Plan{{ID: "existing"}}}
	s := &Scheduler{
		Source:   &fakeSource{err: errors.New("no file yet")},
		Store:    fs,
		MaxBytes: 10 * 1024 * 1024,
		Log:      testLogger(),
	}

	s.runOnce(context.Background())

	if fs.replaceCalls != 0 {
		t.Errorf("replaceCalls = %d, want 0 (fetch failed, store must not be touched)", fs.replaceCalls)
	}
	if len(fs.plans) != 1 || fs.plans[0].ID != "existing" {
		t.Errorf("last-known-good was overwritten: %+v", fs.plans)
	}
}

func TestRunOnce_MissingFileKeepsLastKnownGood(t *testing.T) {
	fs := &fakeStore{plans: []domain.Plan{{ID: "existing"}}}
	s := &Scheduler{
		Source:   &fakeSource{path: "/does/not/exist.xlsx"},
		Store:    fs,
		MaxBytes: 10 * 1024 * 1024,
		Log:      testLogger(),
	}

	s.runOnce(context.Background())

	if fs.replaceCalls != 0 {
		t.Errorf("replaceCalls = %d, want 0", fs.replaceCalls)
	}
}

func TestRunOnce_EmptyParseKeepsLastKnownGood(t *testing.T) {
	fs := &fakeStore{plans: []domain.Plan{{ID: "existing"}}}
	s := &Scheduler{
		Source:   &fakeSource{path: emptyFeedFixture},
		Store:    fs,
		MaxBytes: 10 * 1024 * 1024,
		Log:      testLogger(),
	}

	s.runOnce(context.Background())

	if fs.replaceCalls != 0 {
		t.Errorf("replaceCalls = %d, want 0 (zero-row parse is treated as suspicious)", fs.replaceCalls)
	}
	if len(fs.plans) != 1 {
		t.Errorf("last-known-good was overwritten: %+v", fs.plans)
	}
}

func TestRunOnce_StoreFailureDoesNotPanic(t *testing.T) {
	fs := &fakeStore{replaceErr: errors.New("disk full")}
	s := &Scheduler{
		Source:   &fakeSource{path: realFeedFixture},
		Store:    fs,
		MaxBytes: 10 * 1024 * 1024,
		Log:      testLogger(),
	}

	s.runOnce(context.Background()) // must not panic
}

func TestRun_PerformsImmediateCycleThenStopsOnCancel(t *testing.T) {
	fs := &fakeStore{}
	s := &Scheduler{
		Source:   &fakeSource{path: realFeedFixture},
		Store:    fs,
		Interval: time.Hour, // long enough that only the immediate cycle should fire
		MaxBytes: 10 * 1024 * 1024,
		Log:      testLogger(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	// Give the immediate cycle a moment to run, then stop the loop.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}

	if fs.replaceCalls != 1 {
		t.Errorf("replaceCalls = %d, want exactly 1 (only the immediate startup cycle)", fs.replaceCalls)
	}
}
