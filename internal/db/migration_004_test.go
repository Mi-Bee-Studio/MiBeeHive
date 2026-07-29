package db

import (
	"context"
	"database/sql"
	"testing"
)

func TestMigration004(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// Verify enabled column exists via table_info.

	// Verify enabled column exists via table_info.
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(projects)")
	if err != nil {
		t.Fatalf("PRAGMA table_info(projects): %v", err)
	}
	defer rows.Close()

	foundEnabled := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scanning table_info: %v", err)
		}
		if name == "enabled" {
			foundEnabled = true
			if notnull != 1 {
				t.Errorf("enabled column should be NOT NULL, got notnull=%d", notnull)
			}
			if !dflt.Valid || dflt.String != "1" {
				t.Errorf("enabled column should default to 1, got %q", dflt.String)
			}
		}
	}
	if !foundEnabled {
		t.Error("enabled column not found in projects table")
	}

	// Verify source_credentials table exists.
	var tableName string
	err = db.QueryRowContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name='source_credentials'").Scan(&tableName)
	if err != nil {
		t.Fatalf("source_credentials table not found: %v", err)
	}

	// Verify source_credentials schema.
	rows, err = db.QueryContext(ctx, "PRAGMA table_info(source_credentials)")
	if err != nil {
		t.Fatalf("PRAGMA table_info(source_credentials): %v", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scanning source_credentials table_info: %v", err)
		}
		columns[name] = true
	}

	requiredCols := []string{"id", "source_type", "token", "created_at", "updated_at"}
	for _, col := range requiredCols {
		if !columns[col] {
			t.Errorf("source_credentials missing column %q", col)
		}
	}

	// Verify UNIQUE constraint on source_type.
	// Insert a credential, then try duplicate — should fail.
	_, err = db.ExecContext(ctx,
		"INSERT INTO source_credentials (source_type, token) VALUES (?, ?)",
		"github", "ghp_test123")
	if err != nil {
		t.Fatalf("inserting source_credential: %v", err)
	}
	_, err = db.ExecContext(ctx,
		"INSERT INTO source_credentials (source_type, token) VALUES (?, ?)",
		"github", "ghp_duplicate")
	if err == nil {
		t.Error("expected UNIQUE constraint violation on duplicate source_type")
	}

	// Verify default enabled value on new project.
	pRepo := NewProjectRepo(db)
	p, err := pRepo.Create(ctx, "test-enabled", "Test Enabled", "github", "https://github.com/test/test")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}
	if !p.Enabled {
		t.Error("new project should have enabled=true by default")
	}
}
