package db

import (
	"context"
	"testing"
)

// TestMigration027BackfillFilesSourceType verifies the backfill UPDATE
// repairs pre-027 rows (files created by the crawler, which did not write
// source_type) from the owning project (issue #63).
func TestMigration027BackfillFilesSourceType(t *testing.T) {
	db := testDB(t) // applies all migrations including 027 on a fresh in-memory DB
	ctx := context.Background()

	res, err := db.ExecContext(ctx,
		`INSERT INTO projects (name, display_name, source_type, source_url) VALUES ('prom', 'Prom', 'github', 'https://github.com/prometheus/prometheus')`)
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	pid, _ := res.LastInsertId()

	// Simulate a pre-027 crawler-created row: source_type NULL.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO files (project_id, version, filename, download_url, local_path, status, source_type)
		 VALUES (?, 'v2.0.0', 'prometheus-2.0.0.tar.gz', 'https://example.com/f', '/tmp/f', 'complete', NULL)`,
		pid); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	// Re-run the migration's backfill statement (idempotent).
	if _, err := db.ExecContext(ctx,
		`UPDATE files
		 SET source_type = (SELECT p.source_type FROM projects p WHERE p.id = files.project_id)
		 WHERE (source_type IS NULL OR source_type = '')
		   AND project_id IS NOT NULL`); err != nil {
		t.Fatalf("backfill update: %v", err)
	}

	var st string
	if err := db.QueryRowContext(ctx,
		`SELECT source_type FROM files WHERE project_id = ?`, pid).Scan(&st); err != nil {
		t.Fatalf("query file: %v", err)
	}
	if st != "github" {
		t.Errorf("files.source_type = %q, want %q", st, "github")
	}

	// The FileRepo must round-trip source_type on new rows too.
	repo := NewFileRepo(db)
	id, err := repo.Create(ctx, &File{
		ProjectID: pid, Version: "v2.1.0", Filename: "prometheus-2.1.0.tar.gz",
		DownloadURL: "https://example.com/f2", LocalPath: "/tmp/f2",
		Status: "pending", SourceType: "github",
	})
	if err != nil {
		t.Fatalf("FileRepo.Create: %v", err)
	}
	got, err := repo.GetByID(ctx, id)
	if err != nil || got == nil {
		t.Fatalf("FileRepo.GetByID: %v", err)
	}
	if got.SourceType != "github" {
		t.Errorf("round-tripped SourceType = %q, want %q", got.SourceType, "github")
	}
}
