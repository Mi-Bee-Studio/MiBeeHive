package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/crawler"
	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// mockCrawler is a simple mock for testing crawl handler.
type mockCrawler struct {
	name       string
	sourceType model.SourceType
	err        error
}

func (m *mockCrawler) Name() string                 { return m.name }
func (m *mockCrawler) SourceType() model.SourceType { return m.sourceType }
func (m *mockCrawler) FetchReleases(ctx context.Context, owner, repo string) ([]model.ReleaseAsset, error) {
	return nil, m.err
}

func TestCrawlTrigger_RateLimited(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	tmpDir := t.TempDir()
	fileService := service.NewFileService(database, tmpDir, 2, nil)

	cfg := &config.Config{
		Server:  config.ServerConfig{Port: 9090, BindAddr: "0.0.0.0"},
		Storage: config.StorageConfig{BasePath: tmpDir},
		Crawler: config.CrawlerConfig{MaxConcurrent: 2, DefaultInterval: "1h"},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	mc := &mockCrawler{
		name:       "github",
		sourceType: model.SourceTypeGitHub,
		err:        crawler.ErrRateLimited,
	}

	mgr := crawler.NewCrawlManager(database, fileService, cfg, logger, nil)
	mgr.Scheduler().Register(mc)

	crawlLogRepo := dbrepo.NewCrawlLogRepo(database)
	projectRepo := dbrepo.NewProjectRepo(database)

	h := NewCrawlHandler(mgr, crawlLogRepo, projectRepo)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/crawl/trigger", h.Trigger)
	handler := wrapWithAuth(mux)

	body := `{"project_name":"prometheus"}`
	req := authedRequest(http.MethodPost, "/api/v1/crawl/trigger", []byte(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 TooManyRequests, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Fatal("expected success=false for rate limited response")
	}
	if resp.ErrorCode == "" {
		t.Fatal("expected non-empty error_code")
	}
	if resp.Message == "" {
		t.Fatal("expected non-empty message")
	}
}
