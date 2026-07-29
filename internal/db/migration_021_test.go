package db

import (
	"context"
	"testing"
)

// TestMigration021SourceTypeAny verifies migration 021 drops the closed CHECK
// constraint on projects.source_type, so npm/pypi/crates/rulesrc (and any future
// type) can be inserted. This was a latent bug: the app registered 7 crawlers but
// the schema only allowed 4 source types.
func TestMigration021SourceTypeAny(t *testing.T) {
	db := testDB(t) // applies all migrations including 021 on a fresh in-memory DB
	ctx := context.Background()

	// Every source type the app uses — including the previously-blocked ones —
	// must now insert successfully.
	for _, st := range []string{"github", "go", "hashicorp", "grafana", "npm", "pypi", "crates", "rulesrc"} {
		name := "proj-" + st
		_, err := db.ExecContext(ctx,
			`INSERT INTO projects (name, display_name, source_type, source_url) VALUES (?, ?, ?, ?)`,
			name, name, st, "https://example.com")
		if err != nil {
			t.Errorf("insert source_type=%q failed after migration 021: %v", st, err)
		}
	}

	// Sanity: data actually persisted.
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM projects").Scan(&count); err != nil {
		t.Fatalf("count projects: %v", err)
	}
	if count != 8 {
		t.Errorf("expected 8 projects, got %d", count)
	}
}
