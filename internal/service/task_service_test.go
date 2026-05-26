package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
)

func setupTaskTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	database.SetMaxOpenConns(1)
	return database
}

func TestTaskService_GetAllTasks_EmptyDB(t *testing.T) {
	database := setupTaskTestDB(t)
	defer database.Close()

	svc := NewTaskService(database)
	tasks, err := svc.GetAllTasks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks from empty DB, got %d", len(tasks))
	}
}

func TestTaskService_GetAllTasks_WithProjects(t *testing.T) {
	database := setupTaskTestDB(t)
	defer database.Close()

	// Seed an enabled project.
	_, err := database.Exec(`INSERT INTO projects (name, display_name, source_type, source_url, enabled)
		VALUES ('grafana', 'Grafana', 'github', 'https://github.com/grafana/grafana', 1)`)
	if err != nil {
		t.Fatalf("failed to seed project: %v", err)
	}

	svc := NewTaskService(database)
	tasks, err := svc.GetAllTasks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least 1 crawl task.
	crawlCount := 0
	for _, task := range tasks {
		if task.Type == TaskTypeCrawl {
			crawlCount++
			if task.Name != "Grafana" {
				t.Fatalf("expected crawl task name 'Grafana', got %q", task.Name)
			}
			if task.Status != "scheduled" {
				t.Fatalf("expected crawl task status 'scheduled', got %q", task.Status)
			}
		}
	}
	if crawlCount != 1 {
		t.Fatalf("expected 1 crawl task, got %d", crawlCount)
	}
}

func TestTaskService_GetAllTasks_WithDownloads(t *testing.T) {
	database := setupTaskTestDB(t)
	defer database.Close()

	// Seed a project + a pending file.
	_, err := database.Exec(`INSERT INTO projects (name, display_name, source_type, source_url, enabled)
		VALUES ('testproj', 'TestProj', 'github', 'https://example.com/test', 1)`)
	if err != nil {
		t.Fatalf("failed to seed project: %v", err)
	}
	_, err = database.Exec(`INSERT INTO files (project_id, version, filename, os, arch, ext, size_bytes, download_url, local_path, checksum, status)
		VALUES (1, '1.0.0', 'test-1.0.0.tar.gz', 'linux', 'arm64', '.tar.gz', 2048, 'https://example.com/test.tar.gz', '/tmp/test.tar.gz', 'abc', 'pending')`)
	if err != nil {
		t.Fatalf("failed to seed file: %v", err)
	}

	svc := NewTaskService(database)
	tasks, err := svc.GetAllTasks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	downloadCount := 0
	for _, task := range tasks {
		if task.Type == TaskTypeDownload {
			downloadCount++
			if task.Name != "test-1.0.0.tar.gz" {
				t.Fatalf("expected download task name 'test-1.0.0.tar.gz', got %q", task.Name)
			}
			if task.Status != "pending" {
				t.Fatalf("expected download task status 'pending', got %q", task.Status)
			}
		}
	}
	if downloadCount != 1 {
		t.Fatalf("expected 1 download task, got %d", downloadCount)
	}
}

func TestTaskService_GetAllTasks_WithISO(t *testing.T) {
	database := setupTaskTestDB(t)
	defer database.Close()

	// Seed an ISO catalog entry with auto_update enabled.
	_, err := database.Exec(`INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
		VALUES ('Debian 12', 'debian', 'netinst', 'amd64', 'https://example.com/debian/', 'debian-.*-amd64-netinst.iso', 1, 24, 'available')`)
	if err != nil {
		t.Fatalf("failed to seed ISO catalog: %v", err)
	}

	svc := NewTaskService(database)
	tasks, err := svc.GetAllTasks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	isoCount := 0
	for _, task := range tasks {
		if task.Type == TaskTypeISOCheck {
			isoCount++
			if task.Name != "Debian 12" {
				t.Fatalf("expected ISO task name 'Debian 12', got %q", task.Name)
			}
			if task.Schedule != "every 24h" {
				t.Fatalf("expected ISO task schedule 'every 24h', got %q", task.Schedule)
			}
		}
	}
	if isoCount != 1 {
		t.Fatalf("expected 1 ISO task, got %d", isoCount)
	}
}

func TestTaskService_GetAllTasks_DisabledProjectExcluded(t *testing.T) {
	database := setupTaskTestDB(t)
	defer database.Close()

	// Seed a disabled project.
	_, err := database.Exec(`INSERT INTO projects (name, display_name, source_type, source_url, enabled)
		VALUES ('disabled-proj', 'Disabled', 'github', 'https://example.com/disabled', 0)`)
	if err != nil {
		t.Fatalf("failed to seed project: %v", err)
	}

	svc := NewTaskService(database)
	tasks, err := svc.GetAllTasks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, task := range tasks {
		if task.Type == TaskTypeCrawl && task.Name == "Disabled" {
			t.Fatal("disabled project should not appear as crawl task")
		}
	}
}

func TestTaskService_GetAllTasks_CompletedFileExcluded(t *testing.T) {
	database := setupTaskTestDB(t)
	defer database.Close()

	// Seed a project + a completed file (should NOT appear as download task).
	_, err := database.Exec(`INSERT INTO projects (name, display_name, source_type, source_url, enabled)
		VALUES ('testproj', 'TestProj', 'github', 'https://example.com/test', 1)`)
	if err != nil {
		t.Fatalf("failed to seed project: %v", err)
	}
	_, err = database.Exec(`INSERT INTO files (project_id, version, filename, os, arch, ext, size_bytes, download_url, local_path, checksum, status)
		VALUES (1, '1.0.0', 'done.tar.gz', 'linux', 'arm64', '.tar.gz', 1024, 'https://example.com/done.tar.gz', '/tmp/done.tar.gz', 'abc', 'complete')`)
	if err != nil {
		t.Fatalf("failed to seed file: %v", err)
	}

	svc := NewTaskService(database)
	tasks, err := svc.GetAllTasks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, task := range tasks {
		if task.Type == TaskTypeDownload {
			t.Fatal("completed file should not appear as download task")
		}
	}
}
