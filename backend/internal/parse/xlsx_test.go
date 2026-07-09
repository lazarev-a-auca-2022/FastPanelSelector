package parse

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"fastpanelselector/backend/internal/domain"
)

const defaultMaxBytes = 10 * 1024 * 1024

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("reading testdata/%s: %v", name, err)
	}
	return b
}

func findPlan(plans []domain.Plan, id string) (domain.Plan, bool) {
	for _, p := range plans {
		if p.ID == id {
			return p, true
		}
	}
	return domain.Plan{}, false
}

func TestParseXLSX_RealFeed(t *testing.T) {
	data := readTestdata(t, "cloud_07_07_2026.xlsx")

	plans, err := ParseXLSX(bytes.NewReader(data), defaultMaxBytes)
	if err != nil {
		t.Fatalf("ParseXLSX: %v", err)
	}

	if got, want := len(plans), 112; got != want {
		t.Errorf("total plans = %d, want %d", got, want)
	}

	enabledCount := 0
	for _, p := range plans {
		if p.Enabled {
			enabledCount++
		}
	}
	if got, want := enabledCount, 92; got != want {
		t.Errorf("enabled plans = %d, want %d", got, want)
	}

	p947, ok := findPlan(plans, "947")
	if !ok {
		t.Fatal("plan id=947 not found")
	}
	if p947.Location != "DE" || p947.Cores != 2 || p947.RAM != 4 || p947.Disk != 80 {
		t.Errorf("plan 947 = %+v, want DE/2/4/80", p947)
	}
	if !p947.Enabled {
		t.Errorf("plan 947 should be enabled")
	}
	if p947.Price == nil || *p947.Price != 23.00 {
		t.Errorf("plan 947 price = %v, want 23.00", p947.Price)
	}

	p817, ok := findPlan(plans, "817")
	if !ok {
		t.Fatal("plan id=817 not found")
	}
	if p817.Price == nil || *p817.Price != 6.48 {
		t.Errorf("plan 817 price = %v, want 6.48", p817.Price)
	}

	p930, ok := findPlan(plans, "930")
	if !ok {
		t.Fatal("plan id=930 not found")
	}
	if p930.Enabled {
		t.Errorf("plan 930 should be disabled")
	}
	if p930.Price != nil {
		t.Errorf("plan 930 price = %v, want nil (disabled plans never carry a price)", *p930.Price)
	}
}

func TestParseXLSX_MalformedHeaders(t *testing.T) {
	data := readTestdata(t, "malformed_headers.xlsx")

	_, err := ParseXLSX(bytes.NewReader(data), defaultMaxBytes)
	if err == nil {
		t.Fatal("expected an error for a workbook with no recognizable header row")
	}
	if !strings.Contains(err.Error(), ErrHeaderNotFound.Error()) {
		t.Errorf("error = %v, want it to wrap ErrHeaderNotFound", err)
	}
}

func TestParseXLSX_Empty(t *testing.T) {
	data := readTestdata(t, "empty.xlsx")

	plans, err := ParseXLSX(bytes.NewReader(data), defaultMaxBytes)
	if err != nil {
		t.Fatalf("ParseXLSX: %v", err)
	}
	if plans == nil {
		t.Error("plans should be an empty slice, not nil")
	}
	if len(plans) != 0 {
		t.Errorf("len(plans) = %d, want 0", len(plans))
	}
}

func TestParseXLSX_OversizedInput(t *testing.T) {
	data := readTestdata(t, "cloud_07_07_2026.xlsx")

	_, err := ParseXLSX(bytes.NewReader(data), int64(len(data)-1))
	if err == nil {
		t.Fatal("expected an error when input exceeds maxBytes")
	}
	if err != ErrFileTooLarge {
		t.Errorf("error = %v, want ErrFileTooLarge", err)
	}
}

func TestParseXLSX_NotAWorkbook(t *testing.T) {
	_, err := ParseXLSX(strings.NewReader("this is not a zip file at all"), defaultMaxBytes)
	if err == nil {
		t.Fatal("expected an error for garbage input, got nil")
	}
}
