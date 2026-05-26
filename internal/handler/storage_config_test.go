package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// mockMigrationService implements MigrationService for testing.
type mockMigrationService struct {
	tasks    []service.MigrationTaskInfo
	nextID   int64
	enqueued []struct {
		module, oldPath, newPath string
	}
	cancelled []int64
}

func (m *mockMigrationService) Enqueue(module, oldPath, newPath string) (int64, error) {
	m.nextID++
	m.enqueued = append(m.enqueued, struct {
		module, oldPath, newPath string
	}{module, oldPath, newPath})
	return m.nextID, nil
}

func (m *mockMigrationService) List() ([]service.MigrationTaskInfo, error) {
	return m.tasks, nil
}

func (m *mockMigrationService) Get(id int64) (*service.MigrationTaskInfo, error) {
	for _, t := range m.tasks {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, os.ErrNotExist
}

func (m *mockMigrationService) Cancel(id int64) error {
	for _, t := range m.tasks {
		if t.ID == id {
			m.cancelled = append(m.cancelled, id)
			return nil
		}
	}
	return os.ErrNotExist
}

func setupStorageConfigHandler(t *testing.T) (*StorageConfigHandler, *config.Config, *mockMigrationService) {
	t.Helper()
	cfg := &config.Config{
		Storage: config.StorageConfig{
			BasePath: "/data/base",
			Modules:  config.ModulePaths{},
		},
		Auth: config.AuthConfig{
			JWTSecret: testJWTSecret,
		},
	}
	configPath := t.TempDir() + "/config.yaml"
	resolver := service.NewStorageResolver(cfg)
	mock := &mockMigrationService{nextID: 10}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	h := NewStorageConfigHandler(cfg, configPath, resolver, mock, logger)
	return h, cfg, mock
}

func registerStorageConfigRoutes(mux *http.ServeMux, h *StorageConfigHandler) {
	mux.HandleFunc("GET "+model.RouteStorageConfig, h.GetStorageConfig)
	mux.HandleFunc("PUT "+model.RouteStorageConfig, h.UpdateStorageConfig)
	mux.HandleFunc("GET "+model.RouteStorageMigrations, h.ListMigrations)
	mux.HandleFunc("GET "+model.RouteStorageMigrationByID+"{id}", h.GetMigration)
	mux.HandleFunc("POST "+model.RouteStorageMigrationByID+"{id}/cancel", h.CancelMigration)
}

// === GetStorageConfig Tests ===

func TestStorageConfigHandler_GetStorageConfig(t *testing.T) {
	h, _, _ := setupStorageConfigHandler(t)

	mux := http.NewServeMux()
	registerStorageConfigRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteStorageConfig, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.StorageConfigResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	// With no module overrides, OSS should be empty, fallback should be base_path.
	if resp.Data.OSS != "" {
		t.Fatalf("expected empty OSS, got %q", resp.Data.OSS)
	}
	if resp.Data.OSSFallback != "/data/base" {
		t.Fatalf("expected OSSFallback=/data/base, got %q", resp.Data.OSSFallback)
	}
	if resp.Data.OSInstallFallback != "/data/base/os-install" {
		t.Fatalf("expected OSInstallFallback=/data/base/os-install, got %q", resp.Data.OSInstallFallback)
	}
	if resp.Data.ISOFallback != "/data/base/os-install" {
		t.Fatalf("expected ISOFallback=/data/base/os-install, got %q", resp.Data.ISOFallback)
	}
}

// === UpdateStorageConfig Tests ===

func TestStorageConfigHandler_UpdateStorageConfig_NoChange(t *testing.T) {
	h, cfg, _ := setupStorageConfigHandler(t)

	// Set existing module path.
	cfg.Storage.Modules.OSS = "/data/base"

	mux := http.NewServeMux()
	registerStorageConfigRoutes(mux, h)
	handler := wrapWithAuth(mux)

	ossPath := "/data/base"
	body, _ := json.Marshal(model.StorageConfigUpdateRequest{
		OSS: &ossPath,
	})
	req := authedRequest(http.MethodPut, model.RouteStorageConfig, body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.StorageConfigUpdateResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	// Same path should produce no migrations.
	if len(resp.Data.MigrationIDs) != 0 {
		t.Fatalf("expected no migrations, got %d", len(resp.Data.MigrationIDs))
	}
}

func TestStorageConfigHandler_UpdateStorageConfig_WithMigration(t *testing.T) {
	h, _, mock := setupStorageConfigHandler(t)

	mux := http.NewServeMux()
	registerStorageConfigRoutes(mux, h)
	handler := wrapWithAuth(mux)

	ossPath := "/data/new-oss"
	body, _ := json.Marshal(model.StorageConfigUpdateRequest{
		OSS: &ossPath,
	})
	req := authedRequest(http.MethodPut, model.RouteStorageConfig, body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.StorageConfigUpdateResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if len(resp.Data.MigrationIDs) != 1 {
		t.Fatalf("expected 1 migration, got %d", len(resp.Data.MigrationIDs))
	}
	if resp.Data.MigrationIDs[0] != 11 {
		t.Fatalf("expected migration ID 11, got %d", resp.Data.MigrationIDs[0])
	}

	// Verify mock captured the enqueue.
	if len(mock.enqueued) != 1 {
		t.Fatalf("expected 1 enqueue call, got %d", len(mock.enqueued))
	}
	if mock.enqueued[0].module != "oss" {
		t.Fatalf("expected module=oss, got %q", mock.enqueued[0].module)
	}
}

func TestStorageConfigHandler_UpdateStorageConfig_EmptyBody(t *testing.T) {
	h, _, _ := setupStorageConfigHandler(t)

	mux := http.NewServeMux()
	registerStorageConfigRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body, _ := json.Marshal(model.StorageConfigUpdateRequest{})
	req := authedRequest(http.MethodPut, model.RouteStorageConfig, body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestStorageConfigHandler_UpdateStorageConfig_RelativePath(t *testing.T) {
	h, _, _ := setupStorageConfigHandler(t)

	mux := http.NewServeMux()
	registerStorageConfigRoutes(mux, h)
	handler := wrapWithAuth(mux)

	ossPath := "relative/path"
	body, _ := json.Marshal(model.StorageConfigUpdateRequest{
		OSS: &ossPath,
	})
	req := authedRequest(http.MethodPut, model.RouteStorageConfig, body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// === ListMigrations Tests ===

func TestStorageConfigHandler_ListMigrations(t *testing.T) {
	h, _, mock := setupStorageConfigHandler(t)
	mock.tasks = []service.MigrationTaskInfo{
		{
			ID:        1,
			Module:    "oss",
			OldPath:   "/data/base",
			NewPath:   "/data/new-oss",
			Status:    "completed",
			TotalFiles: 10,
			CreatedAt: "2025-01-01T00:00:00Z",
		},
	}

	mux := http.NewServeMux()
	registerStorageConfigRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteStorageMigrations, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.MigrationTaskResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 migration, got %d", len(resp.Data))
	}
	if resp.Data[0].Module != "oss" {
		t.Fatalf("expected module=oss, got %q", resp.Data[0].Module)
	}
}

// === CancelMigration Tests ===

func TestStorageConfigHandler_CancelMigration_NotFound(t *testing.T) {
	h, _, _ := setupStorageConfigHandler(t)

	mux := http.NewServeMux()
	registerStorageConfigRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodPost, "/api/v1/admin/storage/migrations/999/cancel", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestStorageConfigHandler_CancelMigration_Success(t *testing.T) {
	h, _, mock := setupStorageConfigHandler(t)
	mock.tasks = []service.MigrationTaskInfo{
		{ID: 1, Module: "oss", Status: "running"},
	}

	mux := http.NewServeMux()
	registerStorageConfigRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodPost, "/api/v1/admin/storage/migrations/1/cancel", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(mock.cancelled) != 1 || mock.cancelled[0] != 1 {
		t.Fatalf("expected cancel call for ID 1, got %v", mock.cancelled)
	}
}

// === GetMigration Tests ===

func TestStorageConfigHandler_GetMigration_NotFound(t *testing.T) {
	h, _, _ := setupStorageConfigHandler(t)

	mux := http.NewServeMux()
	registerStorageConfigRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/storage/migrations/999", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestStorageConfigHandler_GetMigration_Success(t *testing.T) {
	h, _, mock := setupStorageConfigHandler(t)
	startedAt := "2025-01-01T01:00:00Z"
	mock.tasks = []service.MigrationTaskInfo{
		{
			ID:         1,
			Module:     "oss",
			OldPath:    "/data/base",
			NewPath:    "/data/new-oss",
			Status:     "running",
			Progress:   50,
			TotalFiles: 10,
			StartedAt:  &startedAt,
			CreatedAt:  "2025-01-01T00:00:00Z",
		},
	}

	mux := http.NewServeMux()
	registerStorageConfigRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/storage/migrations/1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.MigrationTaskResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Data.ID != 1 {
		t.Fatalf("expected ID=1, got %d", resp.Data.ID)
	}
	if resp.Data.Progress != 50 {
		t.Fatalf("expected progress=50, got %d", resp.Data.Progress)
	}
	if resp.Data.StartedAt != "2025-01-01T01:00:00Z" {
		t.Fatalf("expected started_at, got %q", resp.Data.StartedAt)
	}
}

// === Nil MigrationService Tests ===

func TestStorageConfigHandler_NilMigration_ListMigrations(t *testing.T) {
	cfg := &config.Config{
		Storage: config.StorageConfig{BasePath: "/data/base"},
		Auth:    config.AuthConfig{JWTSecret: testJWTSecret},
	}
	resolver := service.NewStorageResolver(cfg)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	h := NewStorageConfigHandler(cfg, "", resolver, nil, logger)

	mux := http.NewServeMux()
	registerStorageConfigRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteStorageMigrations, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.MigrationTaskResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected empty list, got %d items", len(resp.Data))
	}
}
