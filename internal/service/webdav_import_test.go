package service

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/eventbus"

	_ "modernc.org/sqlite"
)

// setupWebdavImportTest creates an in-memory files table (without the projects
// FK so manual uploads can use project_id=0) plus a fresh temp webdav dir.
func setupWebdavImportTest(t *testing.T) (*sql.DB, *eventbus.Bus, string) {
	t.Helper()
	testDB, err := sql.Open("sqlite", ":memory:?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { testDB.Close() })
	testDB.SetMaxOpenConns(1)

	if _, err := testDB.Exec(`CREATE TABLE files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id INTEGER DEFAULT 0,
		version TEXT DEFAULT '',
		filename TEXT NOT NULL,
		os TEXT DEFAULT '',
		arch TEXT DEFAULT '',
		ext TEXT DEFAULT '',
		size_bytes INTEGER DEFAULT 0,
		download_url TEXT DEFAULT '',
		local_path TEXT NOT NULL,
		checksum TEXT DEFAULT '',
		status TEXT DEFAULT 'pending',
		error_message TEXT DEFAULT '',
		retry_count INTEGER DEFAULT 0,
		last_attempt_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		source_type TEXT DEFAULT NULL,
		category TEXT DEFAULT NULL,
		storage_subdir TEXT DEFAULT NULL,
		public_token TEXT DEFAULT NULL
	)`); err != nil {
		t.Fatalf("creating files table: %v", err)
	}

	bus := eventbus.NewBus(10)
	t.Cleanup(bus.Close)

	return testDB, bus, t.TempDir()
}

func writeWebdavTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
}

func countFilesRows(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM files").Scan(&n); err != nil {
		t.Fatalf("counting files: %v", err)
	}
	return n
}

// drainPublishedEvents drains all currently-buffered FilePublished events.
func drainPublishedEvents(ch <-chan eventbus.Event) int {
	n := 0
	for {
		select {
		case <-ch:
			n++
		default:
			return n
		}
	}
}

func TestImportEmptyDir(t *testing.T) {
	db, bus, dir := setupWebdavImportTest(t)
	n, err := ImportWebDAVFiles(context.Background(), db, bus, dir)
	if err != nil {
		t.Fatalf("ImportWebDAVFiles: %v", err)
	}
	if n != 0 {
		t.Errorf("imported = %d, want 0", n)
	}
	if countWebdavRows := countFilesRows(t, db); countWebdavRows != 0 {
		t.Errorf("files count = %d, want 0", countWebdavRows)
	}
}

func TestImportNewFiles(t *testing.T) {
	db, bus, dir := setupWebdavImportTest(t)
	writeWebdavTestFile(t, dir, "a.txt", "hello")
	writeWebdavTestFile(t, dir, "b.bin", "world")

	sub := bus.Subscribe(eventbus.TagFilePublished)
	n, err := ImportWebDAVFiles(context.Background(), db, bus, dir)
	if err != nil {
		t.Fatalf("ImportWebDAVFiles: %v", err)
	}
	if n != 2 {
		t.Errorf("imported = %d, want 2", n)
	}
	if count := countFilesRows(t, db); count != 2 {
		t.Errorf("files count = %d, want 2", count)
	}
	if got := drainPublishedEvents(sub); got != 2 {
		t.Errorf("published events = %d, want 2", got)
	}
}

func TestImportIdempotent(t *testing.T) {
	db, bus, dir := setupWebdavImportTest(t)
	writeWebdavTestFile(t, dir, "a.txt", "hello")

	n1, err := ImportWebDAVFiles(context.Background(), db, bus, dir)
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	if n1 != 1 {
		t.Errorf("first import = %d, want 1", n1)
	}

	n2, err := ImportWebDAVFiles(context.Background(), db, bus, dir)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if n2 != 0 {
		t.Errorf("second import = %d, want 0", n2)
	}
	if got := countFilesRows(t, db); got != 1 {
		t.Errorf("files count = %d, want 1", got)
	}
}

func TestImportSkipsDirs(t *testing.T) {
	db, bus, dir := setupWebdavImportTest(t)
	writeWebdavTestFile(t, dir, "root.txt", "root")

	subdir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeWebdavTestFile(t, subdir, "nested.txt", "nested")

	n, err := ImportWebDAVFiles(context.Background(), db, bus, dir)
	if err != nil {
		t.Fatalf("ImportWebDAVFiles: %v", err)
	}
	if n != 2 {
		t.Errorf("imported = %d, want 2", n)
	}
	if got := countFilesRows(t, db); got != 2 {
		t.Errorf("files count = %d, want 2", got)
	}
}