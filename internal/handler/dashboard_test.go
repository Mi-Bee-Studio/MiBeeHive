package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

func TestDashboardSummary_Success(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	tmpDir := t.TempDir()
	cfg := &config.Config{
		Storage: config.StorageConfig{BasePath: tmpDir},
	}

	projectRepo := dbrepo.NewProjectRepo(database)
	fileRepo := dbrepo.NewFileRepo(database)
	crawlLogRepo := dbrepo.NewCrawlLogRepo(database)
	osConfigRepo := dbrepo.NewOsInstallConfigRepo(database)
	isoCatalogRepo := dbrepo.NewISOCatalogRepo(database)
	fileService := service.NewFileService(database, tmpDir, 2, nil)

	dashSvc := service.NewDashboardService(fileService, projectRepo, fileRepo, crawlLogRepo, osConfigRepo, isoCatalogRepo, cfg, "test-version")
	h := NewDashboardHandler(dashSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminDashboardSummary, h.Summary)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminDashboardSummary, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.DashboardSummaryResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}

	// System field assertions
	if resp.Data.System.Version != "test-version" {
		t.Fatalf("expected version 'test-version', got %q", resp.Data.System.Version)
	}
	if resp.Data.System.Uptime == "" {
		t.Fatal("expected non-empty uptime")
	}

	// Files field assertions (setupTestDB seeds 1 project, 3 files)
	if resp.Data.Files.ProjectCount != 1 {
		t.Fatalf("expected project_count=1, got %d", resp.Data.Files.ProjectCount)
	}
	if resp.Data.Files.TotalFiles != 3 {
		t.Fatalf("expected total_files=3, got %d", resp.Data.Files.TotalFiles)
	}
	// Queue fields should exist (may be 0)
	_ = resp.Data.Files.QueuePending
	_ = resp.Data.Files.QueueDownloading
	_ = resp.Data.Files.QueueComplete
	_ = resp.Data.Files.QueueError

	// Deploy field should exist
	_ = resp.Data.Deploy.ConfigCount
	_ = resp.Data.Deploy.IsoCount

	// Share field should exist
	_ = resp.Data.Share.FileCount
	_ = resp.Data.Share.TotalSize

	// Activity should be a non-nil slice (may be empty)
	if resp.Data.Activity == nil {
		t.Fatal("expected activity to be non-nil slice")
	}
}

func TestDashboardSummary_EmptyData(t *testing.T) {
	database := setupEmptyTestDB(t)
	defer database.Close()

	tmpDir := t.TempDir()
	cfg := &config.Config{
		Storage: config.StorageConfig{BasePath: tmpDir},
	}

	projectRepo := dbrepo.NewProjectRepo(database)
	fileRepo := dbrepo.NewFileRepo(database)
	crawlLogRepo := dbrepo.NewCrawlLogRepo(database)
	osConfigRepo := dbrepo.NewOsInstallConfigRepo(database)
	isoCatalogRepo := dbrepo.NewISOCatalogRepo(database)
	fileService := service.NewFileService(database, tmpDir, 2, nil)

	dashSvc := service.NewDashboardService(fileService, projectRepo, fileRepo, crawlLogRepo, osConfigRepo, isoCatalogRepo, cfg, "test-version")
	h := NewDashboardHandler(dashSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminDashboardSummary, h.Summary)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminDashboardSummary, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.DashboardSummaryResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}

	// All counts should be 0 for empty DB
	if resp.Data.Files.ProjectCount != 0 {
		t.Fatalf("expected project_count=0, got %d", resp.Data.Files.ProjectCount)
	}
	if resp.Data.Files.TotalFiles != 0 {
		t.Fatalf("expected total_files=0, got %d", resp.Data.Files.TotalFiles)
	}
	if resp.Data.Files.QueuePending != 0 {
		t.Fatalf("expected queue_pending=0, got %d", resp.Data.Files.QueuePending)
	}
	if resp.Data.Files.QueueDownloading != 0 {
		t.Fatalf("expected queue_downloading=0, got %d", resp.Data.Files.QueueDownloading)
	}
	if resp.Data.Files.QueueComplete != 0 {
		t.Fatalf("expected queue_complete=0, got %d", resp.Data.Files.QueueComplete)
	}
	if resp.Data.Files.QueueError != 0 {
		t.Fatalf("expected queue_error=0, got %d", resp.Data.Files.QueueError)
	}
	// ConfigCount may be > 0 due to seed data in migrations
	if resp.Data.Deploy.ConfigCount < 0 {
		t.Fatalf("expected config_count >= 0, got %d", resp.Data.Deploy.ConfigCount)
	}
	// IsoCount may be > 0 due to seed data in migrations (007_iso_catalog_seed.sql)
	if resp.Data.Deploy.IsoCount < 0 {
		t.Fatalf("expected iso_count >= 0, got %d", resp.Data.Deploy.IsoCount)
	}
	if resp.Data.Share.FileCount != 0 {
		t.Fatalf("expected share file_count=0, got %d", resp.Data.Share.FileCount)
	}

	// Activity should be empty array
	if len(resp.Data.Activity) != 0 {
		t.Fatalf("expected empty activity, got %d items", len(resp.Data.Activity))
	}
}

func TestDashboardSummary_AuthRequired(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	tmpDir := t.TempDir()
	cfg := &config.Config{
		Storage: config.StorageConfig{BasePath: tmpDir},
	}

	projectRepo := dbrepo.NewProjectRepo(database)
	fileRepo := dbrepo.NewFileRepo(database)
	crawlLogRepo := dbrepo.NewCrawlLogRepo(database)
	osConfigRepo := dbrepo.NewOsInstallConfigRepo(database)
	isoCatalogRepo := dbrepo.NewISOCatalogRepo(database)
	fileService := service.NewFileService(database, tmpDir, 2, nil)

	dashSvc := service.NewDashboardService(fileService, projectRepo, fileRepo, crawlLogRepo, osConfigRepo, isoCatalogRepo, cfg, "test-version")
	h := NewDashboardHandler(dashSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminDashboardSummary, h.Summary)
	handler := wrapWithAuth(mux)

	// Unauthenticated request
	req := httptest.NewRequest(http.MethodGet, model.RouteAdminDashboardSummary, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
