package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/crawler"
	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

func setupCrawlControlHandler(t *testing.T, database *sql.DB) (*CrawlControlHandler, *crawler.CrawlManager, *config.Config) {
	t.Helper()
	projectRepo := db.NewProjectRepo(database)
	credRepo := db.NewSourceCredentialRepo(database)
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Auth: config.AuthConfig{
			PasswordHash: "$2a$10$abcdefghijklmnopqrstuvwxABCDEFGH",
			JWTSecret:    "test-jwt-secret-key-12345",
		},
		Storage: config.StorageConfig{BasePath: tmpDir},
		Crawler: config.CrawlerConfig{MaxConcurrent: 2, DefaultInterval: "6h"},
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	fileService := service.NewFileService(database, service.NewStorageResolver(&config.Config{Storage: config.StorageConfig{BasePath: tmpDir}}), 2, nil)
	cm := crawler.NewCrawlManager(database, fileService, cfg, logger, nil)

	return NewCrawlControlHandler(projectRepo, credRepo, cm), cm, cfg
}

func registerCrawlControlRoutes(mux *http.ServeMux, h *CrawlControlHandler) {
	mux.HandleFunc("POST "+model.RouteAdminCrawlTrigger, h.TriggerCrawl)
	mux.HandleFunc("POST "+model.RouteAdminCrawlTriggerAll, h.TriggerAllCrawls)
	mux.HandleFunc("POST "+model.RouteAdminCrawlPause, h.PauseProject)
	mux.HandleFunc("POST "+model.RouteAdminCrawlResume, h.ResumeProject)
	mux.HandleFunc("GET "+model.RouteAdminCrawlStatus, h.GetCrawlStatus)
	mux.HandleFunc("GET "+model.RouteAdminCredentialsList, h.ListCredentials)
	mux.HandleFunc("PUT "+model.RouteAdminCredentialsUpsert, h.UpsertCredential)
}

func TestAdminTriggerCrawl_NotFound(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	h, _, _ := setupCrawlControlHandler(t, database)
	mux := http.NewServeMux()
	registerCrawlControlRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodPost, "/api/v1/admin/crawl/trigger/nonexistent", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminPauseProject(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	h, _, _ := setupCrawlControlHandler(t, database)
	mux := http.NewServeMux()
	registerCrawlControlRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodPost, "/api/v1/admin/crawl/pause/testproj", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminListCredentials(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	credRepo := db.NewSourceCredentialRepo(database)
	_ = credRepo.Upsert(context.Background(), "github", "ghp_testtoken1234567890")

	h, _, _ := setupCrawlControlHandler(t, database)
	mux := http.NewServeMux()
	registerCrawlControlRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/credentials", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.CredentialResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(resp.Data))
	}
	// Token should be masked.
	if resp.Data[0].Token == "ghp_testtoken1234567890" {
		t.Fatal("expected token to be masked")
	}
	// Should end with last 4 chars.
	if len(resp.Data[0].Token) < 4 {
		t.Fatal("masked token too short")
	}
}

func TestAdminUpsertCredential(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	h, _, _ := setupCrawlControlHandler(t, database)
	mux := http.NewServeMux()
	registerCrawlControlRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body, _ := json.Marshal(model.UpsertCredentialRequest{
		SourceType: "github",
		Token:      "ghp_newtoken12345",
	})
	req := authedRequest(http.MethodPut, "/api/v1/admin/credentials", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify it was saved.
	credRepo := db.NewSourceCredentialRepo(database)
	cred, err := credRepo.GetBySourceType(context.Background(), "github")
	if err != nil {
		t.Fatalf("failed to get credential: %v", err)
	}
	if cred == nil || cred.Token != "ghp_newtoken12345" {
		t.Fatal("expected credential to be saved")
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		token string
		want  string
	}{
		{"abcdefgh", "****efgh"},
		{"abc", "****"},
		{"", "****"},
		{"ghp_1234567890abcdefghijklmnop", "**************************mnop"},
	}
	for _, tt := range tests {
		got := maskToken(tt.token)
		if got != tt.want {
			t.Errorf("maskToken(%q) = %q, want %q", tt.token, got, tt.want)
		}
	}
}
