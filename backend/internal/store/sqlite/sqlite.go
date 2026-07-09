// Package sqlite is the SQLite-backed implementation of store.Store, using
// a pure-Go driver (no cgo) so the service builds as a static binary.
package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"

	"fastpanelselector/backend/internal/domain"
)

//go:embed schema.sql
var schema string

type Store struct {
	db *sql.DB
}

// Open creates (or reuses) the SQLite database at dsn and ensures the
// schema exists. dsn is a file path, or ":memory:" for tests.
//
// A single open connection is used deliberately: this service has one
// writer every ~15 minutes and occasional readers, a traffic profile SQLite
// handles cleanly under a single connection without needing WAL mode or
// SQLITE_BUSY retry logic.
func Open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: applying schema: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// ReplacePlans atomically replaces the entire catalog. Each feed cycle is a
// full snapshot, so a plain delete+insert inside one transaction is both
// simpler and more correct than a merge: it naturally handles plans that
// disappeared from the feed, and a failure at any point rolls back,
// leaving the previous snapshot fully intact for readers.
func (s *Store) ReplacePlans(ctx context.Context, plans []domain.Plan) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	if _, err := tx.ExecContext(ctx, "DELETE FROM plans"); err != nil {
		return fmt.Errorf("sqlite: clearing plans: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO plans (id, location, city, package, arch, cpu_type, cores, ram, disk, enabled, price)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("sqlite: preparing insert: %w", err)
	}
	defer stmt.Close()

	for _, p := range plans {
		enabled := 0
		if p.Enabled {
			enabled = 1
		}
		if _, err := stmt.ExecContext(ctx,
			p.ID, p.Location, p.City, p.Package, p.Arch, p.CPUType,
			p.Cores, p.RAM, p.Disk, enabled, p.Price,
		); err != nil {
			return fmt.Errorf("sqlite: inserting plan id=%s: %w", p.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit: %w", err)
	}
	return nil
}

func (s *Store) ListPlans(ctx context.Context) ([]domain.Plan, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, location, city, package, arch, cpu_type, cores, ram, disk, enabled, price
		FROM plans
		ORDER BY location, package
	`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing plans: %w", err)
	}
	defer rows.Close()

	plans := []domain.Plan{}
	for rows.Next() {
		var p domain.Plan
		var enabled int
		if err := rows.Scan(
			&p.ID, &p.Location, &p.City, &p.Package, &p.Arch, &p.CPUType,
			&p.Cores, &p.RAM, &p.Disk, &enabled, &p.Price,
		); err != nil {
			return nil, fmt.Errorf("sqlite: scanning plan row: %w", err)
		}
		p.Enabled = enabled != 0
		plans = append(plans, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating plan rows: %w", err)
	}
	return plans, nil
}
