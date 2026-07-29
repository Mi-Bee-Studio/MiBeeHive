package db

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// testDB creates an in-memory SQLite database, runs migrations, and returns it.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:): %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}
	return db
}

// TestDBInit verifies database opens with WAL mode and all tables are created.
func TestDBInit(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// Verify WAL mode. Note: :memory: databases report "memory" mode,
	// so we use a temp file for this specific check.
	tmpDB, err := sql.Open("sqlite", t.TempDir()+"/test.db")
	if err != nil {
		t.Fatalf("opening temp db: %v", err)
	}
	tmpDB.SetMaxOpenConns(1)
	if _, err := tmpDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("setting WAL mode: %v", err)
	}
	var mode string
	if err := tmpDB.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("reading journal_mode: %v", err)
	}
	tmpDB.Close()
	if mode != "wal" {
		t.Fatalf("expected journal_mode=wal, got %q", mode)
	}

	// Verify foreign keys enabled.
	var fk int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("reading foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Fatalf("expected foreign_keys=1, got %d", fk)
	}

	// Verify all tables exist.
	tables := []string{"projects", "files", "crawl_logs", "os_install_configs"}
	for _, tbl := range tables {
		var name string
		err := db.QueryRowContext(ctx,
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", tbl, err)
		}
	}

	// Verify indexes exist.
	indexes := []string{"idx_files_project_id", "idx_files_filename", "idx_crawl_logs_project_id"}
	for _, idx := range indexes {
		var name string
		err := db.QueryRowContext(ctx,
			"SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&name)
		if err != nil {
			t.Errorf("index %q not found: %v", idx, err)
		}
	}
}

// TestProjectRepoCRUD tests all project repository operations.
func TestProjectRepoCRUD(t *testing.T) {
	db := testDB(t)
	repo := NewProjectRepo(db)
	ctx := context.Background()

	// Create.
	p, err := repo.Create(ctx, "prometheus", "Prometheus", "github", "https://github.com/prometheus/prometheus")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if p.Name != "prometheus" {
		t.Errorf("expected name=prometheus, got %q", p.Name)
	}
	if p.SourceType != "github" {
		t.Errorf("expected source_type=github, got %q", p.SourceType)
	}
	if p.Config != "{}" {
		t.Errorf("expected default config={}, got %q", p.Config)
	}

	// GetByID.
	got, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.Name != "prometheus" {
		t.Errorf("GetByID: expected prometheus, got %v", got)
	}

	// GetByName.
	got, err = repo.GetByName(ctx, "prometheus")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got == nil || got.DisplayName != "Prometheus" {
		t.Errorf("GetByName: expected Prometheus, got %v", got)
	}

	// GetByName non-existent.
	got, err = repo.GetByName(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetByName(nonexistent): %v", err)
	}
	if got != nil {
		t.Errorf("GetByName(nonexistent): expected nil, got %v", got)
	}

	// Create second project.
	_, err = repo.Create(ctx, "consul", "Consul", "hashicorp", "https://releases.hashicorp.com/consul/")
	if err != nil {
		t.Fatalf("Create consul: %v", err)
	}

	// List.
	projects, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("List: expected 2 projects, got %d", len(projects))
	}
	// Ordered by name alphabetically.
	if projects[0].Name != "consul" || projects[1].Name != "prometheus" {
		t.Errorf("List: expected [consul, prometheus], got [%s, %s]",
			projects[0].Name, projects[1].Name)
	}

	// UpdateLatestVersion.
	err = repo.UpdateLatestVersion(ctx, p.ID, "v2.50.0")
	if err != nil {
		t.Fatalf("UpdateLatestVersion: %v", err)
	}
	got, err = repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if got.LatestVersion != "v2.50.0" {
		t.Errorf("expected latest_version=v2.50.0, got %q", got.LatestVersion)
	}

	// UpdateLastCrawledAt.
	err = repo.UpdateLastCrawledAt(ctx, p.ID)
	if err != nil {
		t.Fatalf("UpdateLastCrawledAt: %v", err)
	}
	got, err = repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID after crawl update: %v", err)
	}
	if got.LastCrawledAt == nil {
		t.Error("expected last_crawled_at to be set")
	}

	// Duplicate name should fail.
	_, err = repo.Create(ctx, "prometheus", "Dup", "github", "https://example.com")
	if err == nil {
		t.Error("expected error on duplicate project name")
	}

	// Invalid source_type should fail (CHECK constraint).
	_, err = repo.Create(ctx, "bad", "Bad", "invalid_type", "https://example.com")
	if err == nil {
		t.Error("expected error on invalid source_type")
	}
}

// TestFileRepoCRUD tests all file repository operations.
func TestFileRepoCRUD(t *testing.T) {
	db := testDB(t)
	pRepo := NewProjectRepo(db)
	fRepo := NewFileRepo(db)
	ctx := context.Background()

	// Create a project for FK.
	p, err := pRepo.Create(ctx, "grafana", "Grafana", "grafana", "https://github.com/grafana/grafana")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	// Create a file.
	f := &File{
		ProjectID:   p.ID,
		Version:     "v10.2.0",
		Filename:    "grafana-10.2.0.linux-amd64.tar.gz",
		OS:          "linux",
		Arch:        "amd64",
		Ext:         ".tar.gz",
		SizeBytes:   102400,
		DownloadURL: "https://example.com/grafana.tar.gz",
		LocalPath:   "/var/lib/mibeehive/grafana/grafana-10.2.0.linux-amd64.tar.gz",
		Status:      "pending",
	}
	id, err := fRepo.Create(ctx, f)
	if err != nil {
		t.Fatalf("Create file: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero file ID")
	}

	// GetByID.
	got, err := fRepo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.Filename != f.Filename {
		t.Errorf("GetByID: expected %q, got %v", f.Filename, got)
	}
	if got.OS != "linux" || got.Arch != "amd64" {
		t.Errorf("GetByID: os=%q arch=%q", got.OS, got.Arch)
	}

	// Create another file for same project.
	f2 := &File{
		ProjectID:   p.ID,
		Version:     "v10.1.0",
		Filename:    "grafana-10.1.0.linux-arm64.tar.gz",
		OS:          "linux",
		Arch:        "arm64",
		DownloadURL: "https://example.com/grafana-arm64.tar.gz",
		LocalPath:   "/var/lib/mibeehive/grafana/grafana-10.1.0.linux-arm64.tar.gz",
		Status:      "complete",
	}
	_, err = fRepo.Create(ctx, f2)
	if err != nil {
		t.Fatalf("Create file2: %v", err)
	}

	// ListByProject.
	files, err := fRepo.ListByProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("ListByProject: expected 2 files, got %d", len(files))
	}
	// Ordered by version DESC, so v10.2.0 first.
	if files[0].Version != "v10.2.0" {
		t.Errorf("expected first file version=v10.2.0, got %q", files[0].Version)
	}

	// FindExisting — should find the first file.
	got, err = fRepo.FindExisting(ctx, p.ID, "grafana-10.2.0.linux-amd64.tar.gz")
	if err != nil {
		t.Fatalf("FindExisting: %v", err)
	}
	if got == nil {
		t.Fatal("FindExisting: expected file, got nil")
	}

	// FindExisting — should not find a non-existent file.
	got, err = fRepo.FindExisting(ctx, p.ID, "nonexistent.tar.gz")
	if err != nil {
		t.Fatalf("FindExisting(nonexistent): %v", err)
	}
	if got != nil {
		t.Errorf("FindExisting(nonexistent): expected nil, got %v", got)
	}

	// UpdateStatus.
	err = fRepo.UpdateStatus(ctx, id, "error", "download failed")
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, err = fRepo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID after status update: %v", err)
	}
	if got.Status != "error" {
		t.Errorf("expected status=error, got %q", got.Status)
	}
	if got.ErrorMessage != "download failed" {
		t.Errorf("expected error_message=download failed, got %q", got.ErrorMessage)
	}

	// CountByProject.
	count, err := fRepo.CountByProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("CountByProject: %v", err)
	}
	if count != 2 {
		t.Errorf("CountByProject: expected 2, got %d", count)
	}

	// CountByProject for non-existent project.
	count, err = fRepo.CountByProject(ctx, 9999)
	if err != nil {
		t.Fatalf("CountByProject(9999): %v", err)
	}
	if count != 0 {
		t.Errorf("CountByProject(9999): expected 0, got %d", count)
	}
}

// TestCrawlLogRepoCRUD tests all crawl log repository operations.
func TestCrawlLogRepoCRUD(t *testing.T) {
	db := testDB(t)
	pRepo := NewProjectRepo(db)
	clRepo := NewCrawlLogRepo(db)
	ctx := context.Background()

	// Create a project.
	p, err := pRepo.Create(ctx, "prometheus", "Prometheus", "github", "https://github.com/prometheus/prometheus")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	// Create a crawl log.
	cl := &CrawlLog{
		ProjectID: p.ID,
		StartedAt: time.Now().UTC().Truncate(time.Second),
		Status:    "running",
	}
	id, err := clRepo.Create(ctx, cl)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero ID")
	}

	// ListByProject — should have 1 entry.
	logs, err := clRepo.ListByProject(ctx, p.ID, 10)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("ListByProject: expected 1 log, got %d", len(logs))
	}
	if logs[0].Status != "running" {
		t.Errorf("expected status=running, got %q", logs[0].Status)
	}
	if logs[0].FinishedAt != nil {
		t.Error("expected finished_at=nil for running log")
	}

	// UpdateFinished.
	err = clRepo.UpdateFinished(ctx, id, "success", 3, 12, "")
	if err != nil {
		t.Fatalf("UpdateFinished: %v", err)
	}

	logs, err = clRepo.ListByProject(ctx, p.ID, 10)
	if err != nil {
		t.Fatalf("ListByProject after update: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].Status != "success" {
		t.Errorf("expected status=success, got %q", logs[0].Status)
	}
	if logs[0].VersionsFound != 3 {
		t.Errorf("expected versions_found=3, got %d", logs[0].VersionsFound)
	}
	if logs[0].FilesDownloaded != 12 {
		t.Errorf("expected files_downloaded=12, got %d", logs[0].FilesDownloaded)
	}
	if logs[0].FinishedAt == nil {
		t.Error("expected finished_at to be set")
	}

	// Create more logs and test limit.
	for i := 0; i < 5; i++ {
		_, _ = clRepo.Create(ctx, &CrawlLog{
			ProjectID: p.ID,
			StartedAt: time.Now().UTC().Truncate(time.Second),
			Status:    "running",
		})
	}

	logs, err = clRepo.ListByProject(ctx, p.ID, 3)
	if err != nil {
		t.Fatalf("ListByProject with limit: %v", err)
	}
	if len(logs) != 3 {
		t.Errorf("ListByProject(limit=3): expected 3, got %d", len(logs))
	}

	// ListByProject with default limit (0).
	logs, err = clRepo.ListByProject(ctx, p.ID, 0)
	if err != nil {
		t.Fatalf("ListByProject(limit=0): %v", err)
	}
	if len(logs) != 6 {
		t.Errorf("ListByProject(limit=0): expected 6 (default 20), got %d", len(logs))
	}

	// ListByProject for non-existent project — should be empty.
	logs, err = clRepo.ListByProject(ctx, 9999, 10)
	if err != nil {
		t.Fatalf("ListByProject(9999): %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("ListByProject(9999): expected 0 logs, got %d", len(logs))
	}
}
