package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
)

func setupLogTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
}
	t.Cleanup(func() { database.Close() })

	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.SetMaxOpenConns(1)

	// Seed a project.
	_, err = database.Exec(`INSERT INTO projects (name, display_name, source_type, source_url)
		VALUES ('testproj', 'Test Project', 'github', 'https://github.com/test/test')`)
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}

	return database
}

func seedCrawlLog(t *testing.T, database *sql.DB, status string) int64 {
	t.Helper()
	var id int64
	err := database.QueryRow(
		`INSERT INTO crawl_logs (project_id, started_at, status, versions_found, files_downloaded, error_message)
		 VALUES (1, CURRENT_TIMESTAMP, ?, 3, 2, '') RETURNING id`, status).Scan(&id)
	if err != nil {
		t.Fatalf("seed crawl log: %v", err)
	}
	return id
}

func seedFile(t *testing.T, database *sql.DB, filename, status, errorMsg string) int64 {
	t.Helper()
	var id int64
	err := database.QueryRow(
		`INSERT INTO files (project_id, version, filename, os, arch, ext, size_bytes, download_url, local_path, checksum, status, error_message)
		 VALUES (1, '1.0.0', ?, 'linux', 'arm64', '.tar.gz', 1024, 'https://example.com/f', '/tmp/f', 'abc', ?, ?)
		 RETURNING id`, filename, status, errorMsg).Scan(&id)
	if err != nil {
		t.Fatalf("seed file: %v", err)
	}
	return id
}

func TestLogService_GetCrawlLogs(t *testing.T) {
	database := setupLogTestDB(t)
	seedCrawlLog(t, database, "success")
	seedCrawlLog(t, database, "error")

	svc := NewLogService(database, "")
	entries, err := svc.GetCrawlLogs(context.Background(), 50, 0)
	if err != nil {
		t.Fatalf("GetCrawlLogs: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Both entries should be type 'crawl' with correct source.
	for _, e := range entries {
		if e.Type != "crawl" {
			t.Fatalf("expected type 'crawl', got %q", e.Type)
		}
		if e.Source != "testproj" {
			t.Fatalf("expected source 'testproj', got %q", e.Source)
		}
	}
	// Verify at least one entry has error level.
	hasError := false
	hasInfo := false
	for _, e := range entries {
		if e.Level == "error" {
			hasError = true
		}
		if e.Level == "info" {
			hasInfo = true
		}
	}
	if !hasError {
		t.Fatal("expected at least one entry with level 'error'")
	}
	if !hasInfo {
		t.Fatal("expected at least one entry with level 'info'")
	}
}


func TestLogService_GetCrawlLogs_Empty(t *testing.T) {
	database := setupLogTestDB(t)
	svc := NewLogService(database, "")
	entries, err := svc.GetCrawlLogs(context.Background(), 50, 0)
	if err != nil {
		t.Fatalf("GetCrawlLogs: %v", err)
	}
	if entries == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestLogService_GetDownloadLogs(t *testing.T) {
	database := setupLogTestDB(t)
	seedFile(t, database, "app-1.0.0.tar.gz", "complete", "")
	seedFile(t, database, "app-1.0.1.tar.gz", "error", "network timeout")

	svc := NewLogService(database, "")
	entries, err := svc.GetDownloadLogs(context.Background(), 50, 0)
	if err != nil {
		t.Fatalf("GetDownloadLogs: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Type != "download" {
		t.Fatalf("expected type 'download', got %q", entries[0].Type)
	}
	for _, e := range entries {
		if e.Type != "download" {
			t.Fatalf("expected type 'download', got %q", e.Type)
		}
	}
	// Verify at least one entry has error level.
	hasError := false
	for _, e := range entries {
		if e.Level == "error" {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Fatal("expected at least one entry with level 'error'")
	}
	if entries[0].Source != "testproj" {
		t.Fatalf("expected source 'testproj', got %q", entries[0].Source)
	}
}


func TestLogService_GetDownloadLogs_Empty(t *testing.T) {
	database := setupLogTestDB(t)
	svc := NewLogService(database, "")
	entries, err := svc.GetDownloadLogs(context.Background(), 50, 0)
	if err != nil {
		t.Fatalf("GetDownloadLogs: %v", err)
	}
	if entries == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestLogService_GetAppLogs_NoLogFile(t *testing.T) {
	database := setupLogTestDB(t)
	svc := NewLogService(database, "")
	entries, err := svc.GetAppLogs(context.Background(), 50)
	if err != nil {
		t.Fatalf("GetAppLogs: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries with no log file, got %d", len(entries))
	}
}

func TestLogService_GetAppLogs_NonexistentPath(t *testing.T) {
	database := setupLogTestDB(t)
	svc := NewLogService(database, "/nonexistent/path/to/logfile.log")
	entries, err := svc.GetAppLogs(context.Background(), 50)
	if err != nil {
		t.Fatalf("GetAppLogs: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for nonexistent path, got %d", len(entries))
	}
}

func TestLogService_GetAppLogs_WithFile(t *testing.T) {
	database := setupLogTestDB(t)
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")
	content := ""
	for i := 0; i < 5; i++ {
		content += fmt.Sprintf("time=2025-01-01T00:00:0%dZ level=INFO msg=test-message-%d\n", i, i)
	}
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	svc := NewLogService(database, logFile)
	entries, err := svc.GetAppLogs(context.Background(), 50)
	if err != nil {
		t.Fatalf("GetAppLogs: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}
	if entries[0].Type != "app" {
		t.Fatalf("expected type 'app', got %q", entries[0].Type)
	}
	if entries[0].Level != "info" {
		t.Fatalf("expected level 'info', got %q", entries[0].Level)
	}
}

func TestLogService_GetAppLogs_LimitTail(t *testing.T) {
	database := setupLogTestDB(t)
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")
	content := ""
	for i := 0; i < 10; i++ {
		content += fmt.Sprintf("time=2025-01-01T00:00:0%dZ level=INFO msg=line-%d\n", i, i)
	}
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	svc := NewLogService(database, logFile)
	entries, err := svc.GetAppLogs(context.Background(), 3)
	if err != nil {
		t.Fatalf("GetAppLogs: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries (tail), got %d", len(entries))
	}
	// Should get the last 3 lines: line-7, line-8, line-9.
	if entries[2].Message != "line-9" {
		t.Fatalf("expected last entry message 'line-9', got %q", entries[2].Message)
	}
}

func TestLogService_GetRecentLogs_AllTypes(t *testing.T) {
	database := setupLogTestDB(t)
	seedCrawlLog(t, database, "success")
	seedFile(t, database, "app-1.0.0.tar.gz", "complete", "")

	svc := NewLogService(database, "")

	// Crawl type.
	entries, err := svc.GetRecentLogs(context.Background(), "crawl", 50, 0)
	if err != nil {
		t.Fatalf("GetRecentLogs crawl: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 crawl entry, got %d", len(entries))
	}

	// Download type.
	entries, err = svc.GetRecentLogs(context.Background(), "download", 50, 0)
	if err != nil {
		t.Fatalf("GetRecentLogs download: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 download entry, got %d", len(entries))
	}

	// App type (no log file).
	entries, err = svc.GetRecentLogs(context.Background(), "app", 50, 0)
	if err != nil {
		t.Fatalf("GetRecentLogs app: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 app entries, got %d", len(entries))
	}

	// Invalid type.
	_, err = svc.GetRecentLogs(context.Background(), "invalid", 50, 0)
	if err == nil {
		t.Fatal("expected error for invalid log type")
	}
}

func TestLogService_GetCrawlLogs_DefaultLimit(t *testing.T) {
	database := setupLogTestDB(t)
	seedCrawlLog(t, database, "success")

	svc := NewLogService(database, "")
	// Pass limit=0, should default to 50.
	entries, err := svc.GetCrawlLogs(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("GetCrawlLogs with default limit: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestLogService_GetDownloadLogs_Offset(t *testing.T) {
	database := setupLogTestDB(t)
	seedFile(t, database, "file-a.tar.gz", "complete", "")
	seedFile(t, database, "file-b.tar.gz", "complete", "")
	seedFile(t, database, "file-c.tar.gz", "complete", "")

	svc := NewLogService(database, "")
	entries, err := svc.GetDownloadLogs(context.Background(), 50, 2)
	if err != nil {
		t.Fatalf("GetDownloadLogs with offset: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry with offset=2, got %d", len(entries))
	}
}
