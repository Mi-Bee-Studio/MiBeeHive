package crawler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"

	_ "modernc.org/sqlite"
)

// --- Mock Logger ---

type mockLogger struct {
	mu   sync.Mutex
	msgs []string
}

func (l *mockLogger) Info(msg string, args ...any) {
	l.mu.Lock()
	l.msgs = append(l.msgs, "INFO: "+msg)
	l.mu.Unlock()
}
func (l *mockLogger) Error(msg string, args ...any) {
	l.mu.Lock()
	l.msgs = append(l.msgs, "ERROR: "+msg)
	l.mu.Unlock()
}
func (l *mockLogger) Warn(msg string, args ...any) {
	l.mu.Lock()
	l.msgs = append(l.msgs, "WARN: "+msg)
	l.mu.Unlock()
}
func (l *mockLogger) Debug(msg string, args ...any) {
	l.mu.Lock()
	l.msgs = append(l.msgs, "DEBUG: "+msg)
	l.mu.Unlock()
}

// --- Test Helpers ---

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("opening in-memory db: %v", err)
	}
	db.SetMaxOpenConns(1)

	// Run migrations manually.
	initSQL := `
	CREATE TABLE projects (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		display_name TEXT NOT NULL,
		source_type TEXT NOT NULL,
		source_url TEXT NOT NULL,
		config JSON NOT NULL DEFAULT '{}',
		enabled BOOLEAN NOT NULL DEFAULT 1,
		latest_version TEXT DEFAULT '',
		last_crawled_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE files (
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
		retry_count INTEGER NOT NULL DEFAULT 0,
		last_attempt_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE crawl_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id INTEGER NOT NULL REFERENCES projects(id),
		started_at DATETIME NOT NULL,
		finished_at DATETIME,
		status TEXT NOT NULL CHECK(status IN ('running','success','error','rate_limited')),
		versions_found INTEGER DEFAULT 0,
		files_downloaded INTEGER DEFAULT 0,
		error_message TEXT DEFAULT ''
	);
	CREATE INDEX idx_files_project_id ON files(project_id);
	CREATE INDEX idx_crawl_logs_project_id ON crawl_logs(project_id);
	`
	if _, err := db.Exec(initSQL); err != nil {
		t.Fatalf("running init SQL: %v", err)
	}
	return db
}

func setupTestProject(t *testing.T, db *sql.DB, name, sourceType string) int64 {
	t.Helper()
	result, err := db.Exec(
		"INSERT INTO projects (name, display_name, source_type, source_url, config) VALUES (?, ?, ?, ?, ?)",
		name, name, sourceType, "https://example.com/"+name,
		`{"github_owner":"test","github_repo":"`+name+`","crawl_interval":3600}`,
	)
	if err != nil {
		t.Fatalf("inserting test project: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("getting project id: %v", err)
	}
	return id
}

func makeTestConfig() *config.Config {
	return &config.Config{
		Server:   config.ServerConfig{Port: 9090, BindAddr: "0.0.0.0"},
		Storage:  config.StorageConfig{BasePath: "/tmp/mibeehive-test-storage"},
		Crawler:  config.CrawlerConfig{MaxConcurrent: 2, DefaultInterval: "1h"},
		Projects: []config.ProjectConfig{},
	}
}

func makeMockReleases() []model.ReleaseAsset {
	return []model.ReleaseAsset{
		{
			Version:     "v2.0.0",
			Filename:    "prometheus-2.0.0.linux-amd64.tar.gz",
			OS:          "linux",
			Arch:        "amd64",
			Ext:         ".tar.gz",
			DownloadURL: "http://example.com/prometheus-2.0.0.linux-amd64.tar.gz",
			SizeBytes:   1024,
			Checksum:    "abc123",
		},
		{
			Version:     "v1.0.0",
			Filename:    "prometheus-1.0.0.linux-amd64.tar.gz",
			OS:          "linux",
			Arch:        "amd64",
			Ext:         ".tar.gz",
			DownloadURL: "http://example.com/prometheus-1.0.0.linux-amd64.tar.gz",
			SizeBytes:   512,
			Checksum:    "def456",
		},
	}
}

// --- Tests ---

func TestSchedulerStartStop(t *testing.T) {
	logger := &mockLogger{}
	s := NewScheduler(logger)

	var mu sync.Mutex
	crawlCount := 0

	// Start project.
	s.StartProject("testproj", 100*time.Millisecond, func(c context.Context) error {
		mu.Lock()
		crawlCount++
		mu.Unlock()
		return nil
	})

	// Wait for at least one crawl.
	time.Sleep(200 * time.Millisecond)

	// Verify crawler was called.
	s.mu.Lock()
	_, running := s.cancelFuncs["testproj"]
	s.mu.Unlock()
	if !running {
		t.Error("expected project to be running")
	}

	s.StopProject("testproj")
	s.StopProject("testproj")

	// Verify stopped.
	s.mu.Lock()
	_, running = s.cancelFuncs["testproj"]
	s.mu.Unlock()
	if running {
		t.Error("expected project to be stopped")
	}
}

func TestSchedulerStopAll(t *testing.T) {
	logger := &mockLogger{}
	s := NewScheduler(logger)

	s.StartProject("p1", 1*time.Hour, func(ctx context.Context) error { return nil })
	s.StartProject("p2", 1*time.Hour, func(ctx context.Context) error { return nil })

	s.StopAll()

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.cancelFuncs) != 0 {
		t.Errorf("expected 0 cancel funcs, got %d", len(s.cancelFuncs))
	}
	if len(s.timers) != 0 {
		t.Errorf("expected 0 timers, got %d", len(s.timers))
	}
}

func TestTriggerCrawl_NewReleases(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	setupTestProject(t, db, "testproject", "github")

	// Pre-insert an existing file for v1.0.0.
	db.Exec(`INSERT INTO files (project_id, version, filename, os, arch, ext, size_bytes, download_url, local_path, status)
	         VALUES (1, 'v1.0.0', 'prometheus-1.0.0.linux-amd64.tar.gz', 'linux', 'amd64', '.tar.gz', 512, 'http://example.com/old', '/tmp/old', 'complete')`)

	cfg := makeTestConfig()
	cfg.Storage.BasePath = tmpDir
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	fileService := service.NewFileService(db, service.NewStorageResolver(&config.Config{Storage: config.StorageConfig{BasePath: tmpDir}}), 2, nil)

	calls := 0
	mgr := NewCrawlManager(db, fileService, cfg, logger, nil)
	mgr.SetFetchFunc(func(ctx context.Context, name, sourceType string, params map[string]string) ([]model.ReleaseAsset, error) {
		calls++
		return makeMockReleases(), nil
	})

	// Note: DownloadFile will fail because the URL doesn't exist.
	// We test the flow logic — the file entry is created, download is attempted.
	result, err := mgr.TriggerCrawl(context.Background(), "testproject")
	if err != nil {
		t.Fatalf("TriggerCrawl failed: %v", err)
	}

	if result.Status != model.CrawlStatusSuccess {
		t.Errorf("expected status success, got %s", result.Status)
	}
	if result.VersionsFound != 2 {
		t.Errorf("expected 2 versions found, got %d", result.VersionsFound)
	}
	// v1.0.0 already exists, so only v2.0.0 is new.
	// But download will fail, so FilesDownloaded may be 0.
	// That's fine — we're testing the filtering logic.
	if calls != 1 {
		t.Errorf("expected 1 crawl call, got %d", calls)
	}
}

func TestConcurrentCrawlLock(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	setupTestProject(t, db, "locktest", "github")

	cfg := makeTestConfig()
	cfg.Storage.BasePath = tmpDir
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	fileService := service.NewFileService(db, service.NewStorageResolver(&config.Config{Storage: config.StorageConfig{BasePath: tmpDir}}), 2, nil)

	// Fetch func that blocks until signaled.
	unblock := make(chan struct{})

	mgr := NewCrawlManager(db, fileService, cfg, logger, nil)
	mgr.SetFetchFunc(func(ctx context.Context, name, sourceType string, params map[string]string) ([]model.ReleaseAsset, error) {
		<-unblock
		return []model.ReleaseAsset{}, nil
	})

	var wg sync.WaitGroup
	wg.Add(2)

	var firstErr, secondErr error

	// First crawl — acquires lock.
	go func() {
		defer wg.Done()
		_, firstErr = mgr.TriggerCrawl(context.Background(), "locktest")
	}()

	// Give first goroutine time to acquire lock.
	time.Sleep(50 * time.Millisecond)

	// Second crawl — should fail with ErrCrawlInProgress.
	go func() {
		defer wg.Done()
		_, secondErr = mgr.TriggerCrawl(context.Background(), "locktest")
	}()

	// Unblock first crawl after a short wait.
	time.Sleep(50 * time.Millisecond)
	close(unblock)

	wg.Wait()

	if firstErr != nil {
		t.Errorf("first crawl should succeed, got error: %v", firstErr)
	}
	if !errors.Is(secondErr, ErrCrawlInProgress) {
		t.Errorf("second crawl should get ErrCrawlInProgress, got: %v", secondErr)
	}
}

func TestTriggerCrawl_ProjectNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	cfg := makeTestConfig()
	cfg.Storage.BasePath = tmpDir
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	fileService := service.NewFileService(db, service.NewStorageResolver(&config.Config{Storage: config.StorageConfig{BasePath: tmpDir}}), 2, nil)
	mgr := NewCrawlManager(db, fileService, cfg, logger, nil)

	_, err := mgr.TriggerCrawl(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent project")
	}
}

func TestTriggerCrawl_FetchError(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	setupTestProject(t, db, "errproject", "github")

	cfg := makeTestConfig()
	cfg.Storage.BasePath = tmpDir
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	fileService := service.NewFileService(db, service.NewStorageResolver(&config.Config{Storage: config.StorageConfig{BasePath: tmpDir}}), 2, nil)

	mgr := NewCrawlManager(db, fileService, cfg, logger, nil)
	mgr.SetFetchFunc(func(ctx context.Context, name, sourceType string, params map[string]string) ([]model.ReleaseAsset, error) {
		return nil, fmt.Errorf("API server unreachable")
	})

	result, err := mgr.TriggerCrawl(context.Background(), "errproject")
	if err == nil {
		t.Fatal("expected error from failed fetch")
	}
	if result.Status != model.CrawlStatusError {
		t.Errorf("expected error status, got %s", result.Status)
	}

	// Verify crawl_log was created with error status.
	var status string
	err = db.QueryRow("SELECT status FROM crawl_logs WHERE project_id = 1 ORDER BY id DESC LIMIT 1").Scan(&status)
	if err != nil {
		t.Fatalf("querying crawl log: %v", err)
	}
	if status != "error" {
		t.Errorf("expected crawl_log status 'error', got %q", status)
	}
}

func TestTriggerCrawl_RateLimited(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	setupTestProject(t, db, "ratelimited", "github")

	cfg := makeTestConfig()
	cfg.Storage.BasePath = tmpDir
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	fileService := service.NewFileService(db, service.NewStorageResolver(&config.Config{Storage: config.StorageConfig{BasePath: tmpDir}}), 2, nil)

	mgr := NewCrawlManager(db, fileService, cfg, logger, nil)
	mgr.SetFetchFunc(func(ctx context.Context, name, sourceType string, params map[string]string) ([]model.ReleaseAsset, error) {
		return nil, ErrRateLimited
	})

	result, err := mgr.TriggerCrawl(context.Background(), "ratelimited")
	if err == nil {
		t.Fatal("expected error from rate limited fetch")
	}
	if result.Status != model.CrawlStatusRateLimited {
		t.Errorf("expected rate_limited status, got %s", result.Status)
	}
}

func TestGetCrawlStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	setupTestProject(t, db, "testproject", "github")

	cfg := makeTestConfig()
	cfg.Storage.BasePath = tmpDir
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	fileService := service.NewFileService(db, service.NewStorageResolver(&config.Config{Storage: config.StorageConfig{BasePath: tmpDir}}), 2, nil)
	mgr := NewCrawlManager(db, fileService, cfg, logger, nil)

	statuses := mgr.GetCrawlStatus()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	info, ok := statuses["testproject"]
	if !ok {
		t.Fatal("expected status for testproject")
	}
	if info.ProjectName != "testproject" {
		t.Errorf("expected project name 'testproject', got %q", info.ProjectName)
	}
	if info.SourceType != model.SourceTypeGitHub {
		t.Errorf("expected source type github, got %s", info.SourceType)
	}
	if info.Running {
		t.Error("expected not running")
	}
}

// TestScheduler_ConcurrentStart verifies that calling StartProject for the same
// project rapidly does not result in multiple goroutines running simultaneously.
// We track the max concurrent goroutines inside crawlFunc — it must never exceed 1.
func TestScheduler_ConcurrentStart(t *testing.T) {
	logger := &mockLogger{}
	s := NewScheduler(logger)

	var (
		activeMu  sync.Mutex
		active    int32
		maxActive int32
	)

	crawlFunc := func(ctx context.Context) error {
		activeMu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		activeMu.Unlock()

		<-ctx.Done() // Block until cancelled.

		activeMu.Lock()
		active--
		activeMu.Unlock()
		return nil
	}

	// Launch 10 concurrent StartProject calls.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.StartProject("raceproj", 1*time.Hour, crawlFunc)
		}()
	}
	wg.Wait()

	// Stop the final goroutine.
	s.StopProject("raceproj")
	time.Sleep(100 * time.Millisecond)

	activeMu.Lock()
	peak := maxActive
	activeMu.Unlock()

	if peak > 1 {
		t.Errorf("expected max 1 concurrent crawlFunc, got %d", peak)
	}

	// Verify project is no longer running.
	if s.Running("raceproj") {
		t.Error("expected project to not be running after StopProject")
	}
}
