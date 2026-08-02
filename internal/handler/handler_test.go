package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/crawler"
	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

const testJWTSecret = "test-jwt-secret-key-12345"

// setupTestDB creates an in-memory SQLite database, runs migrations, and seeds test data.
func setupTestDB(t *testing.T) *sql.DB {
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
	seedTestData(t, database)
	return database
}

func seedTestData(t *testing.T, database *sql.DB) {
	t.Helper()
	// Insert a project.
	_, err := database.Exec(`INSERT INTO projects (name, display_name, source_type, source_url, latest_version, last_crawled_at)
		VALUES ('prometheus', 'Prometheus', 'github', 'https://github.com/prometheus/prometheus', '2.50.0', '2025-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("failed to seed project: %v", err)
	}

	// Insert files for the project.
	for _, f := range []struct {
		version, filename, os, arch, ext, downloadURL, localPath, checksum, status string
	}{
		{"2.50.0", "prometheus-2.50.0.linux-arm64.tar.gz", "linux", "arm64", ".tar.gz", "https://example.com/p1.tar.gz", "/tmp/p1.tar.gz", "abc123", "complete"},
		{"2.50.0", "prometheus-2.50.0.darwin-amd64.tar.gz", "darwin", "amd64", ".tar.gz", "https://example.com/p2.tar.gz", "/tmp/p2.tar.gz", "def456", "complete"},
		{"2.49.0", "prometheus-2.49.0.linux-arm64.tar.gz", "linux", "arm64", ".tar.gz", "https://example.com/p3.tar.gz", "/tmp/p3.tar.gz", "ghi789", "pending"},
	} {
		_, err := database.Exec(`INSERT INTO files (project_id, version, filename, os, arch, ext, size_bytes, download_url, local_path, checksum, status, source_type, category, storage_subdir, public_token)
			VALUES (1, ?, ?, ?, ?, ?, 1024, ?, ?, ?, ?, 'github', 'ops', 'oss', ?)`,
			f.version, f.filename, f.os, f.arch, f.ext, f.downloadURL, f.localPath, f.checksum, f.status, f.filename+"-token")
		if err != nil {
			t.Fatalf("failed to seed file: %v", err)
		}
	}
}
// validTestToken returns a valid JWT token string for testing.
// Reuses generateTestToken from auth_test.go (same package).
func validTestToken() string {
	return generateTestToken(testJWTSecret, time.Now().Add(24*time.Hour))
}

// authedRequest creates an httptest request with a valid JWT token.
func authedRequest(method, url string, body []byte) *http.Request {
	var bodyReader *bytes.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, url, bodyReader)
	req.Header.Set("Authorization", "Bearer "+validTestToken())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// wrapWithAuth wraps a handler with auth middleware for testing.
func wrapWithAuth(h http.Handler) http.Handler {
	return middleware.AuthMiddleware(testJWTSecret)(h)
}

// containsSQLOrDatabase checks if the message contains raw SQL or database details.
func containsSQLOrDatabase(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "sql") || strings.Contains(lower, "database")
}

// === Project Handler Tests ===

func TestListProjects(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	h := NewProjectHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects", h.List)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.ProjectResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 project, got %d", len(resp.Data))
	}
	if resp.Data[0].Name != "prometheus" {
		t.Fatalf("expected project name 'prometheus', got %q", resp.Data[0].Name)
	}
	if resp.Data[0].FileCount != 3 {
		t.Fatalf("expected file_count=3, got %d", resp.Data[0].FileCount)
	}
	if resp.Data[0].LatestVersion != "2.50.0" {
		t.Fatalf("expected latest_version='2.50.0', got %q", resp.Data[0].LatestVersion)
	}
}

func TestGetProjectByID(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	h := NewProjectHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects/{id}", h.GetByID)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/projects/1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.ProjectResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Data.Name != "prometheus" {
		t.Fatalf("expected 'prometheus', got %q", resp.Data.Name)
	}
	if resp.Data.FileCount != 3 {
		t.Fatalf("expected file_count=3, got %d", resp.Data.FileCount)
	}
}

func TestGetProjectByID_NotFound(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	h := NewProjectHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects/{id}", h.GetByID)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/projects/999", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetProjectByID_InvalidID(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	h := NewProjectHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects/{id}", h.GetByID)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/projects/abc", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// === File Handler Tests ===

func TestListFilesByProject(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	h := NewFileHandler(database, nil, testJWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects/{id}/files", h.ListByProject)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/projects/1/files", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.FileResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 files, got %d", len(resp.Data))
	}
}

func TestListFilesByProject_WithFilters(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	h := NewFileHandler(database, nil, testJWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects/{id}/files", h.ListByProject)
	handler := wrapWithAuth(mux)

	// Filter by version.
	req := authedRequest(http.MethodGet, "/api/v1/projects/1/files?version=2.50.0", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp model.ApiResponse[[]model.FileResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 files for version 2.50.0, got %d", len(resp.Data))
	}

	// Filter by OS.
	req = authedRequest(http.MethodGet, "/api/v1/projects/1/files?os=linux", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 files for os=linux, got %d", len(resp.Data))
	}

	// Filter by arch.
	req = authedRequest(http.MethodGet, "/api/v1/projects/1/files?arch=amd64", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 file for arch=amd64, got %d", len(resp.Data))
	}
}

func TestSearchFiles(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	h := NewFileHandler(database, nil, testJWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/files/search", h.Search)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/files/search?q=linux", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.FileResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 files matching 'linux', got %d", len(resp.Data))
	}
}

func TestSearchFiles_MissingQuery(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	h := NewFileHandler(database, nil, testJWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/files/search", h.Search)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/files/search", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDownloadFile_NotFound(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	h := NewFileHandler(database, nil, testJWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/files/{id}/download", h.Download)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/files/999/download", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDownloadFile(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Create a temp file to "download".
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "testfile.tar.gz")
	if err := os.WriteFile(testFile, []byte("test file content here"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Update the local_path in DB to point to our temp file.
	_, err := database.Exec("UPDATE files SET local_path = ? WHERE id = 1", testFile)
	if err != nil {
		t.Fatalf("failed to update file path: %v", err)
	}

	fileService := service.NewFileService(database, service.NewStorageResolver(&config.Config{Storage: config.StorageConfig{BasePath: tmpDir}}), 2, nil)
	h := NewFileHandler(database, fileService, testJWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/files/{id}/download", h.Download)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/files/1/download", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if body != "test file content here" {
		t.Fatalf("unexpected body: %q", body)
	}
	// Check Content-Disposition header.
	cd := rec.Header().Get("Content-Disposition")
	if cd == "" {
		t.Fatal("expected Content-Disposition header")
	}
}

// === System Handler Tests ===

func TestSystemInfo(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	tmpDir := t.TempDir()
	fileService := service.NewFileService(database, service.NewStorageResolver(&config.Config{Storage: config.StorageConfig{BasePath: tmpDir}}), 2, nil)
	h := NewSystemHandler(database, fileService, tmpDir, "test-version", "http://localhost:9100/metrics")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/system/info", h.Info)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/system/info", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.SystemInfoResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Data.FileCount != 3 {
		t.Fatalf("expected file_count=3, got %d", resp.Data.FileCount)
	}
	if resp.Data.ProjectCount != 1 {
		t.Fatalf("expected project_count=1, got %d", resp.Data.ProjectCount)
	}
	if resp.Data.Version == "" {
		t.Fatal("expected non-empty version")
	}
	if resp.Data.DiskTotal <= 0 {
		t.Fatal("expected positive disk_total")
	}
}

// === OS Install Handler Tests ===

func TestOSInstallConfigs_List(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	h := NewOSInstallHandler(db.NewOsInstallConfigRepo(database), service.NewOsTemplateService(), t.TempDir())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/admin/os-install/configs", h.ListConfigs)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/os-install/configs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.OsInstallConfig]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if resp.Data == nil {
		t.Fatal("expected non-nil data")
	}
}

// === Auth Tests ===

func TestUnauthenticatedRequest(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	h := NewProjectHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects", h.List)
	handler := wrapWithAuth(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// === Frontend Serving Tests ===
// Frontend serving is tested via the web package in web_test.go.
// Here we verify the API routes are correctly registered and non-API paths return 404
// (since there's no file server attached in the test mux).

func TestNonAPIPathReturns404(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()
	h := NewProjectHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects", h.List)
	// No catch-all handler — non-API paths should 404.
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/some-random-page", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-API path, got %d", rec.Code)
	}
}

// === Query-Param Token Download Tests ===

func TestFileDownloadQueryToken(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	h := NewFileHandler(database, nil, testJWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/files/{id}/download", h.Download)
	// Plain mux — no auth middleware, handler does its own JWT validation.

	token := validTestToken()

	// Valid token via query param → should NOT be 401 (will be 404 or 500 since no real file).
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/1/download?token="+token, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("expected non-401 with valid query token, got %d: %s", rec.Code, rec.Body.String())
	}

	// Invalid token via query param → should be 401.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/files/1/download?token=invalid-token-here", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with invalid query token, got %d: %s", rec.Code, rec.Body.String())
	}

	// No token at all → should be 401.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/files/1/download", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d: %s", rec.Code, rec.Body.String())
	}

	// Valid token via Authorization header (backward compat) → should NOT be 401.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/files/1/download", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("expected non-401 with valid header token, got %d: %s", rec.Code, rec.Body.String())
	}
}

// === Project Handler Error Sanitization Tests ===

func TestListProjects_SanitizedError(t *testing.T) {
	database := setupTestDB(t)
	// Close the database to simulate a DB error.
	database.Close()
	h := NewProjectHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects", h.List)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Fatal("expected success=false")
	}
	// Verify no raw database error details are exposed.
	if containsSQLOrDatabase(resp.Message) {
		t.Fatalf("response message should not contain SQL details: %s", resp.Message)
	}
}

func TestGetProjectByID_SanitizedError(t *testing.T) {
	database := setupTestDB(t)
	// Close the database to simulate a DB error.
	database.Close()
	h := NewProjectHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects/{id}", h.GetByID)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/projects/1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Fatal("expected success=false")
	}
	// Verify no raw database error details are exposed.
	if containsSQLOrDatabase(resp.Message) {
		t.Fatalf("response message should not contain SQL details: %s", resp.Message)
	}
}

// setupAdminTestDB creates an in-memory SQLite database with migrations and admin test data.
// It is identical to setupEmptyTestDB but is named for clarity in admin handler tests.
func setupAdminTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	database.SetMaxOpenConns(1)
	return database
}

// TestAdminEndpoints_Return401WithoutJWT verifies that admin endpoints require JWT authentication.
func TestAdminEndpoints_Return401WithoutJWT(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

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

	projectRepo := db.NewProjectRepo(database)
	credRepo := db.NewSourceCredentialRepo(database)
	adminH := NewAdminHandler(projectRepo, db.NewFileRepo(database), credRepo, cm, cfg, "", service.NewStorageResolver(cfg))

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminProjectsList, adminH.ListProjects)
	mux.HandleFunc("POST "+model.RouteAdminProjectsCreate, adminH.CreateProject)
	mux.HandleFunc("GET "+model.RouteAdminCredentialsList, adminH.ListCredentials)
	mux.HandleFunc("POST "+model.RouteAdminPasswordChange, adminH.ChangePassword)
	mux.HandleFunc("GET "+model.RouteAdminCrawlStatus, adminH.GetCrawlStatus)
	handler := wrapWithAuth(mux)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/admin/projects"},
		{http.MethodPost, "/api/v1/admin/projects"},
		{http.MethodGet, "/api/v1/admin/credentials"},
		{http.MethodPost, "/api/v1/admin/password"},
		{http.MethodGet, "/api/v1/admin/crawl/status"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 for %s %s, got %d", ep.method, ep.path, rec.Code)
			}
		})
	}
}
