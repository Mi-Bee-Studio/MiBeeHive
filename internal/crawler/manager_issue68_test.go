package crawler

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// #68: the WebDAV virtual project (source_type manual_upload) must never be
// scheduled for crawling, and a manual trigger must fail fast with a clear
// error instead of "no fetcher registered" noise in the crawl log.
func TestManualUploadProjectNotCrawled(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	setupTestProject(t, db, "realproject", "github")
	// Seed the virtual project the same way webdav/vfs.go does (enabled=1).
	if _, err := db.Exec(
		`INSERT INTO projects (name, display_name, source_type, source_url, config, enabled)
		 VALUES ('manual_uploads', 'Manual Uploads', 'manual_upload', '', '{}', 1)`); err != nil {
		t.Fatalf("seeding manual_uploads: %v", err)
	}

	tmpDir := t.TempDir()
	cfg := makeTestConfig()
	cfg.Storage.BasePath = tmpDir
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	fileService := service.NewFileService(db, service.NewStorageResolver(&config.Config{Storage: config.StorageConfig{BasePath: tmpDir}}), 2, nil)

	mgr := NewCrawlManager(db, fileService, cfg, logger, nil)
	fetchCalls := 0
	mgr.SetFetchFunc(func(ctx context.Context, name, sourceType string, params map[string]string) ([]model.ReleaseAsset, error) {
		fetchCalls++
		return nil, nil
	})

	// Direct trigger of the virtual project → clear error, no crawl log row.
	_, err := mgr.TriggerCrawl(context.Background(), "manual_uploads")
	if !errors.Is(err, ErrVirtualProject) {
		t.Fatalf("TriggerCrawl(manual_uploads) err = %v, want ErrVirtualProject", err)
	}

	// TriggerAll skips it entirely.
	results := mgr.TriggerAllCrawls(context.Background())
	for _, r := range results {
		if r.ProjectName == "manual_uploads" {
			t.Errorf("TriggerAllCrawls included manual_uploads: %+v", r)
		}
	}

	// Scheduled start also skips it: no entries in the scheduler.
	if err := mgr.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer mgr.Stop()
	if mgr.Scheduler().Running("manual_uploads") {
		t.Error("manual_uploads was scheduled for crawling")
	}
	if !mgr.Scheduler().Running("realproject") {
		t.Error("realproject missing from scheduler")
	}

	var logCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM crawl_logs cl
		JOIN projects p ON p.id = cl.project_id WHERE p.source_type = 'manual_upload'`).Scan(&logCount); err != nil {
		t.Fatalf("counting crawl logs: %v", err)
	}
	if logCount != 0 {
		t.Errorf("manual_uploads produced %d crawl log rows, want 0", logCount)
	}
}
