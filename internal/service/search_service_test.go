package service

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
)

func setupSearchTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	database.SetMaxOpenConns(1)

	// Seed project
	_, err = database.Exec(`INSERT INTO projects (name, display_name, source_type, source_url, latest_version)
		VALUES ('prometheus', 'Prometheus', 'github', 'https://github.com/prometheus/prometheus', '2.50.0')`)
	if err != nil {
		t.Fatalf("failed to seed project: %v", err)
	}

	// Seed files
	files := []struct {
		version, filename, os, arch, ext, downloadURL, localPath, checksum, status string
	}{
		{"2.50.0", "prometheus-2.50.0.linux-arm64.tar.gz", "linux", "arm64", ".tar.gz", "https://example.com/p1.tar.gz", "/tmp/p1.tar.gz", "abc123", "complete"},
		{"2.50.0", "prometheus-2.50.0.darwin-amd64.tar.gz", "darwin", "amd64", ".tar.gz", "https://example.com/p2.tar.gz", "/tmp/p2.tar.gz", "def456", "complete"},
		{"2.49.0", "prometheus-2.49.0.linux-arm64.tar.gz", "linux", "arm64", ".tar.gz", "https://example.com/p3.tar.gz", "/tmp/p3.tar.gz", "ghi789", "pending"},
	}
	for _, f := range files {
		_, err := database.Exec(`INSERT INTO files (project_id, version, filename, os, arch, ext, size_bytes, download_url, local_path, checksum, status)
			VALUES (1, ?, ?, ?, ?, ?, 1024, ?, ?, ?, ?)`,
			f.version, f.filename, f.os, f.arch, f.ext, f.downloadURL, f.localPath, f.checksum, f.status)
		if err != nil {
			t.Fatalf("failed to seed file: %v", err)
		}
	}

	// Seed an OS install config
	_, err = database.Exec(`INSERT INTO os_install_configs (name, config_name, os_type, config, enabled)
		VALUES ('prometheus-server', 'prom-server', 'debian', '{}', 1)`)
	if err != nil {
		t.Fatalf("failed to seed config: %v", err)
	}

	// Seed an ISO catalog entry with 'prometheus' in name
	_, err = database.Exec(`INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, status, download_status)
		VALUES ('Prometheus OS', 'debian', 'netinst', 'arm64', 'https://example.com', 'debian-12.*-arm64.iso', 1, 'available', 'idle')`)
	if err != nil {
		t.Fatalf("failed to seed ISO catalog: %v", err)
	}

	// Seed a container app
	_, err = database.Exec(`INSERT INTO container_apps (name, image, status) VALUES ('prometheus-app', 'prom/prometheus:latest', 'stopped')`)
	if err != nil {
		t.Fatalf("failed to seed container: %v", err)
	}

	return database
}

func TestSearchService_Search_AllTypes(t *testing.T) {
	database := setupSearchTestDB(t)
	defer database.Close()

	svc := NewSearchService(database)
	resp, err := svc.Search(context.Background(), "prometheus", "all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find project
	if len(resp.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(resp.Projects))
	}
	if resp.Projects[0].Name != "prometheus" {
		t.Fatalf("expected project name 'prometheus', got %q", resp.Projects[0].Name)
	}
	if resp.Projects[0].Type != "project" {
		t.Fatalf("expected type 'project', got %q", resp.Projects[0].Type)
	}

	// Should find files
	if len(resp.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(resp.Files))
	}

	// Should find config
	if len(resp.Configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(resp.Configs))
	}
	if resp.Configs[0].Name != "prometheus-server" {
		t.Fatalf("expected config name 'prometheus-server', got %q", resp.Configs[0].Name)
	}

	// Should find ISO
	if len(resp.ISOs) != 1 {
		t.Fatalf("expected 1 ISO, got %d", len(resp.ISOs))
	}

	// Should find container
	if len(resp.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(resp.Containers))
	}

	// Total
	if resp.Total != 7 {
		t.Fatalf("expected total 7, got %d", resp.Total)
	}
}

func TestSearchService_Search_ProjectsOnly(t *testing.T) {
	database := setupSearchTestDB(t)
	defer database.Close()

	svc := NewSearchService(database)
	resp, err := svc.Search(context.Background(), "prometheus", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(resp.Projects))
	}
	// Other categories should be empty (non-nil)
	if len(resp.Files) != 0 {
		t.Fatalf("expected 0 files for project-only search, got %d", len(resp.Files))
	}
	if len(resp.Configs) != 0 {
		t.Fatalf("expected 0 configs for project-only search, got %d", len(resp.Configs))
	}
	if resp.Total != 1 {
		t.Fatalf("expected total 1, got %d", resp.Total)
	}
}

func TestSearchService_Search_FilesOnly(t *testing.T) {
	database := setupSearchTestDB(t)
	defer database.Close()

	svc := NewSearchService(database)
	resp, err := svc.Search(context.Background(), "prometheus", "file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(resp.Files))
	}
	if len(resp.Projects) != 0 {
		t.Fatalf("expected 0 projects for file-only search, got %d", len(resp.Projects))
	}
	if resp.Total != 3 {
		t.Fatalf("expected total 3, got %d", resp.Total)
	}
}

func TestSearchService_Search_NoResults(t *testing.T) {
	database := setupSearchTestDB(t)
	defer database.Close()

	svc := NewSearchService(database)
	resp, err := svc.Search(context.Background(), "zzznonexistent", "all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Total != 0 {
		t.Fatalf("expected total 0 for nonexistent query, got %d", resp.Total)
	}
	// All slices should be non-nil empty
	if resp.Projects == nil || len(resp.Projects) != 0 {
		t.Fatal("expected non-nil empty Projects slice")
	}
	if resp.Files == nil || len(resp.Files) != 0 {
		t.Fatal("expected non-nil empty Files slice")
	}
}

func TestSearchService_Search_EmptyQuery(t *testing.T) {
	database := setupSearchTestDB(t)
	defer database.Close()

	svc := NewSearchService(database)
	_, err := svc.Search(context.Background(), "", "all")
	if err == nil {
		t.Fatal("expected error for empty query, got nil")
	}
}
