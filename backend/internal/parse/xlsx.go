// Package parse turns a Hetzner-style plan feed spreadsheet into domain.Plan
// records. The feed arrives from a semi-trusted external process on a timer,
// so every entry point here is defensive: no path here may panic or block
// the scheduler that calls it indefinitely.
package parse

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"fastpanelselector/backend/internal/domain"
)

var (
	ErrFileTooLarge   = errors.New("parse: feed file exceeds maximum allowed size")
	ErrHeaderNotFound = errors.New("parse: could not locate a recognizable header row")
	ErrNoSheets       = errors.New("parse: workbook has no sheets")
	ErrTooManyBadRows = errors.New("parse: too many rows failed to parse, feed likely malformed")
)

// headerScanWindow bounds how many leading rows we'll inspect while looking
// for the header row. The real feed has one blank spacer row before it.
const headerScanWindow = 10

// minRowsForAbortCheck: below this many candidate rows, don't apply the
// "too many bad rows" abort — a couple of dirty rows in a small file isn't
// evidence of structural drift, it's just noise.
const minRowsForAbortCheck = 5

var requiredColumns = []string{
	"Location", "City", "ID", "Package (billing product)", "Arch",
	"CPU type", "Cores", "RAM (GB)", "Disk (GB)", "Enabled?", "Price",
}

// ParseXLSX reads a plan feed workbook and returns the plans it describes.
// r is fully buffered up to maxBytes+1 before excelize ever sees it, so an
// oversized or unbounded stream is rejected up front rather than read into
// memory in full.
func ParseXLSX(r io.Reader, maxBytes int64) (plans []domain.Plan, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			// excelize parses untrusted binary (zip+XML) input; a panic deep
			// inside a third-party dependency must not take the service down.
			err = fmt.Errorf("parse: recovered from panic while parsing workbook: %v", rec)
		}
	}()

	buf, err := readLimited(r, maxBytes)
	if err != nil {
		return nil, err
	}

	f, err := excelize.OpenReader(bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("parse: opening workbook: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, ErrNoSheets
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("parse: reading sheet %q: %w", sheets[0], err)
	}

	col, headerRow, err := findHeader(rows)
	if err != nil {
		return nil, err
	}

	var out []domain.Plan
	seen, failed := 0, 0
	for _, row := range rows[headerRow+1:] {
		plan, ok, rowErr := parseRow(row, col)
		if !ok {
			continue // blank ID: not a real offering at this location, expected/normal
		}
		seen++
		if rowErr != nil {
			failed++
			continue
		}
		out = append(out, plan)
	}
	if seen >= minRowsForAbortCheck && failed*2 > seen {
		return nil, fmt.Errorf("%w: %d/%d candidate rows failed to parse", ErrTooManyBadRows, failed, seen)
	}

	if out == nil {
		out = []domain.Plan{}
	}
	return out, nil
}

func readLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	buf, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("parse: reading feed: %w", err)
	}
	if int64(len(buf)) > maxBytes {
		return nil, ErrFileTooLarge
	}
	return buf, nil
}

// findHeader scans the first rows of the sheet for one that contains every
// required column label, and returns a column-name -> index map plus the
// 0-based row index the header was found on.
func findHeader(rows [][]string) (map[string]int, int, error) {
	window := len(rows)
	if window > headerScanWindow {
		window = headerScanWindow
	}
	for i := 0; i < window; i++ {
		idx := indexColumns(rows[i])
		if hasAllRequired(idx) {
			return idx, i, nil
		}
	}
	return nil, 0, ErrHeaderNotFound
}

func indexColumns(row []string) map[string]int {
	idx := make(map[string]int, len(row))
	for i, c := range row {
		name := strings.TrimSpace(c)
		if name == "" {
			continue
		}
		idx[name] = i
	}
	return idx
}

func hasAllRequired(idx map[string]int) bool {
	for _, name := range requiredColumns {
		if _, ok := idx[name]; !ok {
			return false
		}
	}
	return true
}

func cell(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// parseRow converts one spreadsheet row into a Plan. ok is false when the
// row has no ID — that's the normal signal for "this location/spec
// combination isn't offered," not a parse failure. err is non-nil when the
// row does claim to be a real offering (has an ID) but a required field
// couldn't be parsed.
func parseRow(row []string, col map[string]int) (domain.Plan, bool, error) {
	id := cell(row, col["ID"])
	if id == "" {
		return domain.Plan{}, false, nil
	}

	cores, err := strconv.Atoi(cell(row, col["Cores"]))
	if err != nil {
		return domain.Plan{}, true, fmt.Errorf("row id=%s: bad Cores: %w", id, err)
	}
	ram, err := strconv.Atoi(cell(row, col["RAM (GB)"]))
	if err != nil {
		return domain.Plan{}, true, fmt.Errorf("row id=%s: bad RAM: %w", id, err)
	}
	disk, err := strconv.Atoi(cell(row, col["Disk (GB)"]))
	if err != nil {
		return domain.Plan{}, true, fmt.Errorf("row id=%s: bad Disk: %w", id, err)
	}

	enabled := parseEnabled(cell(row, col["Enabled?"]))

	// Defensive invariant, not a workaround for an observed bug: a disabled
	// plan never carries a price, regardless of what the cell holds.
	var price *float64
	if enabled {
		if raw := cell(row, col["Price"]); raw != "" {
			v, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return domain.Plan{}, true, fmt.Errorf("row id=%s: bad Price: %w", id, err)
			}
			rounded := math.Round(v*100) / 100
			price = &rounded
		}
	}

	return domain.Plan{
		Location: cell(row, col["Location"]),
		City:     cell(row, col["City"]),
		ID:       id,
		Package:  cell(row, col["Package (billing product)"]),
		Arch:     cell(row, col["Arch"]),
		CPUType:  cell(row, col["CPU type"]),
		Cores:    cores,
		RAM:      ram,
		Disk:     disk,
		Enabled:  enabled,
		Price:    price,
	}, true, nil
}

// parseEnabled treats the column as numeric (1 = enabled), per the feed's
// actual convention, not as a free-form truthy string.
func parseEnabled(raw string) bool {
	if raw == "" {
		return false
	}
	v, err := strconv.ParseFloat(raw, 64)
	return err == nil && v != 0
}
