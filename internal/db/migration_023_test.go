package db

import (
	"context"
	"testing"
)

// TestMigration023VirtualIndex verifies migration 023 creates the six virtual
// index tables, extends files with the four supply-layer metadata columns, and
// records version '023' in schema_migrations (#34: virtual directory trees for
// the supply layer).
func TestMigration023VirtualIndex(t *testing.T) {
	db := testDB(t) // applies all migrations including 023 on a fresh in-memory DB
	ctx := context.Background()

	// schema_migrations must contain version '023'.
	var applied int
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM schema_migrations WHERE version = '023'").Scan(&applied)
	if err != nil {
		t.Fatalf("query schema_migrations for 023: %v", err)
	}
	if applied != 1 {
		t.Fatalf("expected schema_migrations to contain version '023', got count=%d", applied)
	}

	// All six virtual index tables must exist.
	tables := []string{
		"channels",
		"virtual_views",
		"virtual_nodes",
		"share_links",
		"virtual_node_paths",
		"virtual_rule_entries",
	}
	for _, tbl := range tables {
		var name string
		err := db.QueryRowContext(ctx,
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found after migration 023: %v", tbl, err)
		}
	}

	// files must have the four new columns.
	cols := []string{"source_type", "category", "storage_subdir", "public_token"}
	for _, col := range cols {
		var name string
		err := db.QueryRowContext(ctx,
			"SELECT name FROM pragma_table_info('files') WHERE name=?", col).Scan(&name)
		if err != nil {
			t.Errorf("column %q not found on files after migration 023: %v", col, err)
		}
	}

	// The unique partial index on files.public_token must exist.
	var idxName string
	err = db.QueryRowContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='index' AND name='idx_files_public_token'").Scan(&idxName)
	if err != nil {
		t.Errorf("index idx_files_public_token not found after migration 023: %v", err)
	}
}

// TestMigration023Idempotent verifies running Migrate twice is safe: the
// ALTER TABLE ADD COLUMN statements are skipped on the second run (duplicate
// column), and no tables are recreated with errors.
func TestMigration023Idempotent(t *testing.T) {
	db := testDB(t) // first run applies all migrations
	ctx := context.Background()

	// Second run must succeed without error.
	if err := Migrate(db); err != nil {
		t.Fatalf("second Migrate() run failed: %v", err)
	}

	// The four columns must still be present exactly once.
	for _, col := range []string{"source_type", "category", "storage_subdir", "public_token"} {
		var cnt int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM pragma_table_info('files') WHERE name=?", col).Scan(&cnt)
		if err != nil {
			t.Fatalf("counting column %q: %v", col, err)
		}
		if cnt != 1 {
			t.Errorf("expected exactly one %q column after idempotent re-run, got %d", col, cnt)
		}
	}
}