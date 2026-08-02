package db

import (
	"context"
	"testing"
)

// TestMigration022CrawlLogsNetworkError verifies migration 022 adds
// 'network_error' to crawl_logs.status CHECK constraint. Before this migration,
// inserting a 'network_error' status would fail the CHECK (#23: transient
// fetch failures need a distinct status so they're separable from genuine
// upstream/config errors).
func TestMigration022CrawlLogsNetworkError(t *testing.T) {
	db := testDB(t) // applies all migrations including 022 on a fresh in-memory DB
	ctx := context.Background()

	// Need a project row to satisfy the foreign key.
	res, err := db.ExecContext(ctx,
		`INSERT INTO projects (name, display_name, source_type, source_url) VALUES (?, ?, ?, ?)`,
		"netproj", "Net Proj", "github", "https://example.com")
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	pid, _ := res.LastInsertId()

	// Every status value allowed by the post-022 CHECK must insert.
	for _, st := range []string{"running", "success", "error", "rate_limited", "network_error"} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO crawl_logs (project_id, started_at, status) VALUES (?, datetime('now'), ?)`,
			pid, st)
		if err != nil {
			t.Errorf("insert crawl_logs status=%q failed after migration 022: %v", st, err)
		}
	}

	// An unknown status must still be rejected (the CHECK is still enforced).
	_, err = db.ExecContext(ctx,
		`INSERT INTO crawl_logs (project_id, started_at, status) VALUES (?, datetime('now'), ?)`,
		pid, "bogus_status")
	if err == nil {
		t.Error("expected CHECK constraint to reject an unknown crawl_logs status, but insert succeeded")
	}
}
