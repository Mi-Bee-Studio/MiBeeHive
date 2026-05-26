package db

import (
	"context"
	"database/sql"
	"testing"
)

// TestMigration018FreshInstall verifies that a fresh DB with all 18 migrations
// has the storage_migrations table with correct columns and index.
func TestMigration018FreshInstall(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// Verify storage_migrations table exists.
	var name string
	err := db.QueryRowContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name='storage_migrations'",
	).Scan(&name)
	if err != nil {
		t.Fatalf("storage_migrations table not found: %v", err)
	}

	// Verify index exists.
	err = db.QueryRowContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='index' AND name='idx_storage_migrations_status'",
	).Scan(&name)
	if err != nil {
		t.Errorf("idx_storage_migrations_status index not found: %v", err)
	}

	// Verify columns via PRAGMA table_info.
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(storage_migrations)")
	if err != nil {
		t.Fatalf("PRAGMA table_info(storage_migrations): %v", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var colName, colType string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &colName, &colType, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scanning table_info: %v", err)
		}
		columns[colName] = true
	}

	expectedCols := []string{
		"id", "module", "old_path", "new_path", "status",
		"progress", "total_files", "migrated_files", "total_bytes", "migrated_bytes",
		"started_at", "completed_at", "error_message", "created_at", "updated_at",
	}
	for _, col := range expectedCols {
		if !columns[col] {
			t.Errorf("column %q not found in storage_migrations", col)
		}
	}

	// Verify CHECK constraint on status by testing valid insert.
	_, err = db.ExecContext(ctx,
		`INSERT INTO storage_migrations (module, old_path, new_path, status) VALUES (?, ?, ?, ?)`,
		"oss", "/old", "/new", "pending")
	if err != nil {
		t.Fatalf("insert valid migration task: %v", err)
	}

	// Verify invalid status is rejected.
	_, err = db.ExecContext(ctx,
		`INSERT INTO storage_migrations (module, old_path, new_path, status) VALUES (?, ?, ?, ?)`,
		"oss", "/old", "/new", "invalid")
	if err == nil {
		t.Error("expected CHECK constraint to reject invalid status 'invalid'")
	}
}

// TestMigration018UpgradePath verifies that applying migration 018 after 001-017
// creates the storage_migrations table correctly.
func TestMigration018UpgradePath(t *testing.T) {
	ctx := context.Background()

	// Create a fresh DB and manually apply only up to migration 017.
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:): %v", err)
	}
	defer db.Close()

	// Disable foreign keys before transaction.
	if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable foreign_keys: %v", err)
	}

	// Apply migrations 001-017 only.
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin tx: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at DATETIME DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}

	allMigrations := []string{
		"001", "002", "003", "004", "005", "006", "007",
		"008", "009", "010", "011", "012", "013", "014",
		"015", "016", "017",
	}

	for _, name := range allMigrations {
		entries, err := migrationsFS.ReadDir("migrations")
		if err != nil {
			t.Fatalf("ReadDir migrations: %v", err)
		}
		var filePath string
		for _, e := range entries {
			if len(e.Name()) > 7 && e.Name()[:3] == name {
				filePath = "migrations/" + e.Name()
				break
			}
		}
		if filePath == "" {
			t.Fatalf("migration %s file not found", name)
		}

		data, err := migrationsFS.ReadFile(filePath)
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if err := execMigration(tx, string(data)); err != nil {
			t.Fatalf("exec migration %s: %v", name, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", name); err != nil {
			t.Fatalf("record migration %s: %v", name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit partial migrations: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("re-enable foreign_keys: %v", err)
	}

	// Verify storage_migrations does NOT exist before migration 018.
	var tblName string
	err = db.QueryRowContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name='storage_migrations'",
	).Scan(&tblName)
	if err == nil {
		t.Fatal("storage_migrations table should not exist before migration 018")
	}

	tx, err = db.Begin()
	if err != nil {
		t.Fatalf("Begin tx for 018: %v", err)
	}
	defer tx.Rollback()

	// Find and apply migration 018.
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var filePath string
	for _, e := range entries {
		if len(e.Name()) > 7 && e.Name()[:3] == "018" {
			filePath = "migrations/" + e.Name()
			break
		}
	}

	data, err := migrationsFS.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read migration 018: %v", err)
	}
	if err := execMigration(tx, string(data)); err != nil {
		t.Fatalf("exec migration 018: %v", err)
	}
	if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES ('018')"); err != nil {
		t.Fatalf("record migration 018: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit 018: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("re-enable foreign_keys: %v", err)
	}

	// Verify storage_migrations table now exists after migration 018.
	err = db.QueryRowContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name='storage_migrations'",
	).Scan(&tblName)
	if err != nil {
		t.Fatalf("storage_migrations should exist after migration 018: %v", err)
	}

	// Verify we can CRUD on the new table.
	_, err = db.ExecContext(ctx,
		`INSERT INTO storage_migrations (module, old_path, new_path, status) VALUES (?, ?, ?, ?)`,
		"oss", "/mnt/old/oss", "/mnt/new/oss", "pending")
	if err != nil {
		t.Fatalf("insert into storage_migrations after upgrade: %v", err)
	}

	var count int
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM storage_migrations").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 row in storage_migrations, got %d", count)
	}

	// Verify migration 018 is idempotent.
	// Re-applying should not fail.
	tx, err = db.Begin()
	if err != nil {
		t.Fatalf("Begin tx for re-apply 018: %v", err)
	}
	defer tx.Rollback()

	if err := execMigration(tx, string(data)); err != nil {
		t.Fatalf("re-exec migration 018: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit re-apply 018: %v", err)
	}

	t.Log("upgrade path test passed: storage_migrations created, CRUD works, idempotent")
}
