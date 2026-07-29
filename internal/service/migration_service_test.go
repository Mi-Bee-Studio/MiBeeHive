package service

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"

	_ "modernc.org/sqlite"
)

func setupMigrationTest(t *testing.T) (*MigrationService, *db.MigrationTaskRepo, *db.FileRepo, *sql.DB, string, string) {
	t.Helper()
	testDB, err := sql.Open("sqlite", ":memory:?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { testDB.Close() })

	testDB.SetMaxOpenConns(1)
	testDB.Exec("PRAGMA journal_mode=WAL")
	testDB.Exec("PRAGMA foreign_keys=ON")

	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			source_type TEXT NOT NULL,
			source_url TEXT NOT NULL,
			config JSON NOT NULL DEFAULT '{}',
			latest_version TEXT DEFAULT '',
			last_crawled_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL REFERENCES projects(id),
			version TEXT NOT NULL,
			filename TEXT NOT NULL,
			os TEXT DEFAULT '',
			arch TEXT DEFAULT '',
			ext TEXT DEFAULT '',
			size_bytes INTEGER DEFAULT 0,
			download_url TEXT NOT NULL,
			local_path TEXT NOT NULL,
			checksum TEXT DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','downloading','complete','error','imported','failed_permanent')),
			error_message TEXT DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			retry_count INTEGER DEFAULT 0,
			last_attempt_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS storage_migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			module TEXT NOT NULL,
			old_path TEXT NOT NULL,
			new_path TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','running','completed','failed','cancelled')),
			progress INTEGER DEFAULT 0,
			total_files INTEGER DEFAULT 0,
			migrated_files INTEGER DEFAULT 0,
			total_bytes INTEGER DEFAULT 0,
			migrated_bytes INTEGER DEFAULT 0,
			started_at DATETIME,
			completed_at DATETIME,
			error_message TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	} {
		if _, err := testDB.Exec(ddl); err != nil {
			t.Fatalf("creating table: %v", err)
		}
	}

	migRepo := db.NewMigrationTaskRepo(testDB)
	fileRepo := db.NewFileRepo(testDB)
	svc := NewMigrationService(migRepo, fileRepo, slog.Default())

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	return svc, migRepo, fileRepo, testDB, srcDir, dstDir
}

func insertTestProject(t *testing.T, testDB *sql.DB) int64 {
	t.Helper()
	res, err := testDB.ExecContext(context.Background(),
		`INSERT INTO projects (name, display_name, source_type, source_url) VALUES (?, ?, ?, ?)`,
		"test-project", "Test Project", "github", "https://github.com/test/project",
	)
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func insertMigrationTestFile(t *testing.T, fileRepo *db.FileRepo, projectID int64, localPath string, sizeBytes int64) int64 {
	t.Helper()
	f := &db.File{
		ProjectID:    projectID,
		Version:      "v1.0.0",
		Filename:     filepath.Base(localPath),
		OS:           "linux",
		Arch:         "arm64",
		Ext:          ".tar.gz",
		SizeBytes:    sizeBytes,
		DownloadURL:  "https://example.com/test.tar.gz",
		LocalPath:    localPath,
		Status:       "complete",
		ErrorMessage: "",
	}
	id, err := fileRepo.Create(context.Background(), f)
	if err != nil {
		t.Fatalf("creating file record: %v", err)
	}
	return id
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("creating dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
}

func TestMigrationService_EnqueueMigration(t *testing.T) {
	svc, migRepo, _, _, srcDir, dstDir := setupMigrationTest(t)
	writeTestFile(t, filepath.Join(srcDir, "test.tar.gz"), "hello world")

	ctx := context.Background()
	taskID, err := svc.EnqueueMigration(ctx, "oss", srcDir, dstDir)
	if err != nil {
		t.Fatalf("EnqueueMigration: %v", err)
	}
	if taskID == 0 {
		t.Fatal("expected non-zero task ID")
	}

	task, err := migRepo.GetByID(ctx, taskID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if task.Status != "pending" {
		t.Errorf("expected status=pending, got %q", task.Status)
	}
	if task.Module != "oss" {
		t.Errorf("expected module=oss, got %q", task.Module)
	}
	if task.OldPath != srcDir {
		t.Errorf("expected old_path=%s, got %q", srcDir, task.OldPath)
	}
	if task.NewPath != dstDir {
		t.Errorf("expected new_path=%s, got %q", dstDir, task.NewPath)
	}
}

func TestMigrationService_EnqueueMigration_SamePath(t *testing.T) {
	svc, _, _, _, srcDir, _ := setupMigrationTest(t)
	_, err := svc.EnqueueMigration(context.Background(), "oss", srcDir, srcDir)
	if err == nil {
		t.Fatal("expected error for same old_path and new_path")
	}
}

func TestMigrationService_EnqueueMigration_EmptyPath(t *testing.T) {
	svc, _, _, _, _, _ := setupMigrationTest(t)
	_, err := svc.EnqueueMigration(context.Background(), "oss", "", "/some/path")
	if err == nil {
		t.Fatal("expected error for empty old_path")
	}
}

func TestMigrationService_StartMigration_CopiesFiles(t *testing.T) {
	svc, migRepo, _, _, srcDir, dstDir := setupMigrationTest(t)

	writeTestFile(t, filepath.Join(srcDir, "a.tar.gz"), "content-a")
	writeTestFile(t, filepath.Join(srcDir, "sub", "b.tar.gz"), "content-b")
	writeTestFile(t, filepath.Join(srcDir, "sub", "deep", "c.tar.gz"), "content-c")

	ctx := context.Background()
	taskID, err := svc.EnqueueMigration(ctx, "oss", srcDir, dstDir)
	if err != nil {
		t.Fatalf("EnqueueMigration: %v", err)
	}

	if err := svc.StartMigration(ctx, taskID); err != nil {
		t.Fatalf("StartMigration: %v", err)
	}

	assertMigrationStatus(t, migRepo, taskID, "completed", 10*time.Second)

	assertFileContent(t, filepath.Join(dstDir, "a.tar.gz"), "content-a")
	assertFileContent(t, filepath.Join(dstDir, "sub", "b.tar.gz"), "content-b")
	assertFileContent(t, filepath.Join(dstDir, "sub", "deep", "c.tar.gz"), "content-c")

	assertFileNotExists(t, filepath.Join(srcDir, "a.tar.gz"))
	assertFileNotExists(t, filepath.Join(srcDir, "sub", "b.tar.gz"))
}

func TestMigrationService_StartMigration_UpdatesDBPaths(t *testing.T) {
	svc, migRepo, fileRepo, testDB, srcDir, dstDir := setupMigrationTest(t)

	srcFile := filepath.Join(srcDir, "app-1.0.0.tar.gz")
	writeTestFile(t, srcFile, "app-content")

	projectID := insertTestProject(t, testDB)
	fileID := insertMigrationTestFile(t, fileRepo, projectID, srcFile, 12)

	ctx := context.Background()
	taskID, err := svc.EnqueueMigration(ctx, "oss", srcDir, dstDir)
	if err != nil {
		t.Fatalf("EnqueueMigration: %v", err)
	}

	if err := svc.StartMigration(ctx, taskID); err != nil {
		t.Fatalf("StartMigration: %v", err)
	}

	assertMigrationStatus(t, migRepo, taskID, "completed", 10*time.Second)

	updated, err := fileRepo.GetByID(ctx, fileID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	expectedPath := filepath.Join(dstDir, "app-1.0.0.tar.gz")
	if updated.LocalPath != expectedPath {
		t.Errorf("expected local_path=%q, got %q", expectedPath, updated.LocalPath)
	}
}

func TestMigrationService_StartMigration_NotFound(t *testing.T) {
	svc, _, _, _, _, _ := setupMigrationTest(t)
	err := svc.StartMigration(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestMigrationService_StartMigration_AlreadyRunning(t *testing.T) {
	svc, _, _, _, srcDir, dstDir := setupMigrationTest(t)
	writeTestFile(t, filepath.Join(srcDir, "test.tar.gz"), "data")

	ctx := context.Background()
	taskID, _ := svc.EnqueueMigration(ctx, "oss", srcDir, dstDir)
	svc.StartMigration(ctx, taskID)

	// Try to start again — should fail because status is now 'running'.
	err := svc.StartMigration(ctx, taskID)
	if err == nil {
		t.Fatal("expected error for already-running task")
	}
}

func TestMigrationService_FailedCopy_NoDBUpdate(t *testing.T) {
	svc, migRepo, fileRepo, testDB, srcDir, dstDir := setupMigrationTest(t)

	srcFile := filepath.Join(srcDir, "app-1.0.0.tar.gz")
	writeTestFile(t, srcFile, "app-content")

	projectID := insertTestProject(t, testDB)
	fileID := insertMigrationTestFile(t, fileRepo, projectID, srcFile, 12)

	// Make destination unwritable.
	if err := os.Chmod(dstDir, 0555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(dstDir, 0755)

	ctx := context.Background()
	taskID, err := svc.EnqueueMigration(ctx, "oss", srcDir, dstDir)
	if err != nil {
		t.Skipf("skipping: EnqueueMigration failed: %v", err)
	}

	if err := svc.StartMigration(ctx, taskID); err != nil {
		t.Fatalf("StartMigration: %v", err)
	}

	assertMigrationStatus(t, migRepo, taskID, "failed", 10*time.Second)

	updated, err := fileRepo.GetByID(ctx, fileID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.LocalPath != srcFile {
		t.Errorf("expected local_path unchanged at %q, got %q", srcFile, updated.LocalPath)
	}

	if _, err := os.Stat(srcFile); err != nil {
		t.Errorf("source file should still exist: %v", err)
	}
}

func TestMigrationService_CancelMigration(t *testing.T) {
	svc, migRepo, _, _, _, _ := setupMigrationTest(t)
	ctx := context.Background()

	taskID, err := migRepo.Create(ctx, &db.MigrationTask{
		Module:  "oss",
		OldPath: "/old",
		NewPath: "/new",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := migRepo.SetStarted(ctx, taskID); err != nil {
		t.Fatalf("SetStarted: %v", err)
	}

	// Manually register a cancel func in the active map to simulate running migration.
	ctxCancel, cancel := context.WithCancel(context.Background())
	_ = ctxCancel
	defer cancel()
	svc.active.Store(taskID, context.CancelFunc(func() { cancel() }))

	if err := svc.CancelMigration(taskID); err != nil {
		t.Fatalf("CancelMigration: %v", err)
	}

	task, err := migRepo.GetByID(ctx, taskID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if task.Status != "cancelled" {
		t.Errorf("expected status=cancelled, got %q", task.Status)
	}

	// Verify cancel func was removed from active map.
	if _, ok := svc.active.Load(taskID); ok {
		t.Error("expected task to be removed from active map")
	}
}

func TestMigrationService_CancelMigration_NotActive(t *testing.T) {
	svc, _, _, _, _, _ := setupMigrationTest(t)
	err := svc.CancelMigration(999)
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestMigrationService_ResetStaleMigrations(t *testing.T) {
	svc, migRepo, _, _, _, _ := setupMigrationTest(t)
	ctx := context.Background()

	taskID, err := migRepo.Create(ctx, &db.MigrationTask{
		Module:  "oss",
		OldPath: "/old",
		NewPath: "/new",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := migRepo.SetStarted(ctx, taskID); err != nil {
		t.Fatalf("SetStarted: %v", err)
	}

	task, _ := migRepo.GetByID(ctx, taskID)
	if task.Status != "running" {
		t.Fatalf("expected running, got %q", task.Status)
	}

	if err := svc.ResetStaleMigrations(ctx); err != nil {
		t.Fatalf("ResetStaleMigrations: %v", err)
	}

	task, err = migRepo.GetByID(ctx, taskID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if task.Status != "pending" {
		t.Errorf("expected status=pending, got %q", task.Status)
	}
}

func TestMigrationService_GetProgress(t *testing.T) {
	svc, _, _, _, srcDir, dstDir := setupMigrationTest(t)
	writeTestFile(t, filepath.Join(srcDir, "test.tar.gz"), "hello")

	ctx := context.Background()
	taskID, _ := svc.EnqueueMigration(ctx, "oss", srcDir, dstDir)

	task, err := svc.GetProgress(ctx, taskID)
	if err != nil {
		t.Fatalf("GetProgress: %v", err)
	}
	if task.ID != taskID {
		t.Errorf("expected task ID %d, got %d", taskID, task.ID)
	}
	if task.Status != "pending" {
		t.Errorf("expected status=pending, got %q", task.Status)
	}
}

func TestMigrationService_ListActive(t *testing.T) {
	svc, _, _, _, srcDir, dstDir := setupMigrationTest(t)
	writeTestFile(t, filepath.Join(srcDir, "test.tar.gz"), "hello")

	ctx := context.Background()
	svc.EnqueueMigration(ctx, "oss", srcDir, dstDir)

	active, err := svc.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active task, got %d", len(active))
	}
	if active[0].Status != "pending" {
		t.Errorf("expected status=pending, got %q", active[0].Status)
	}
}

func assertMigrationStatus(t *testing.T, repo *db.MigrationTaskRepo, taskID int64, wantStatus string, timeout time.Duration) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		task, err := repo.GetByID(ctx, taskID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if task.Status == wantStatus {
			return
		}
		if task.Status == "failed" && wantStatus != "failed" {
			t.Fatalf("migration failed unexpectedly: %s", task.ErrorMessage)
		}
		time.Sleep(50 * time.Millisecond)
	}
	task, _ := repo.GetByID(ctx, taskID)
	t.Fatalf("timeout waiting for status %q, got %q", wantStatus, task.Status)
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("file %s: expected %q, got %q", path, want, string(got))
	}
}

func assertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("file %s should not exist", path)
	}
}
