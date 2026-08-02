package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// TestFileCenterFilters verifies that server-side filtering works correctly.
func TestFileCenterFilters(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Add a second project with different files for cross-project testing.
	_, err := database.Exec(`INSERT INTO projects (name, display_name, source_type, source_url, latest_version, last_crawled_at)
		VALUES ('grafana', 'Grafana', 'github', 'https://github.com/grafana/grafana', '10.0.0', '2025-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("failed to seed second project: %v", err)
	}

	// Add files for grafana project with different OS.
	_, err = database.Exec(`INSERT INTO files (project_id, version, filename, os, arch, ext, size_bytes, download_url, local_path, checksum, status, source_type, category, storage_subdir, public_token)
		VALUES (2, '10.0.0', 'grafana-10.0.0.windows-amd64.zip', 'windows', 'amd64', '.zip', 2048, 'https://example.com/g1.zip', '/tmp/g1.zip', 'windowshash', 'complete', 'github', 'ops', 'oss', 'grafana-token-123')`)
	if err != nil {
		t.Fatalf("failed to seed grafana file: %v", err)
	}

	h := NewFileCenterHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteFiles, h.ServeFileCenter)
	handler := wrapWithAuth(mux)

	// Test OS filter: only linux files.
	req := authedRequest(http.MethodGet, "/api/v1/admin/files?os=linux", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]FileCenterFileResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}

	// Should return only linux files (2 from prometheus).
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 linux files, got %d", len(resp.Data))
	}
	for _, f := range resp.Data {
		if f.OS != "linux" {
			t.Fatalf("expected all files to have os=linux, got %q for file %d", f.OS, f.ID)
		}
	}
}

// TestFileCenterSearch verifies keyword search matches filename or version.
func TestFileCenterSearch(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	h := NewFileCenterHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteFiles, h.ServeFileCenter)
	handler := wrapWithAuth(mux)

	// Search by version "2.50.0".
	req := authedRequest(http.MethodGet, "/api/v1/admin/files?q=2.50.0", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]FileCenterFileResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}

	// Should return 2 files with version 2.50.0.
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 files matching version 2.50.0, got %d", len(resp.Data))
	}
	for _, f := range resp.Data {
		if f.Version != "2.50.0" {
			t.Fatalf("expected all files to have version=2.50.0, got %q for file %d", f.Version, f.ID)
		}
	}

	// Search by filename "prometheus".
	req = authedRequest(http.MethodGet, "/api/v1/admin/files?q=prometheus", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}

	// Should return all 3 files with "prometheus" in filename.
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 files matching 'prometheus', got %d", len(resp.Data))
	}
}

// TestFileCenterPagination verifies limit and offset work correctly.
func TestFileCenterPagination(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	h := NewFileCenterHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteFiles, h.ServeFileCenter)
	handler := wrapWithAuth(mux)

	// First page with limit=2.
	req := authedRequest(http.MethodGet, "/api/v1/admin/files?limit=2&offset=0", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]FileCenterFileResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}

	// Check pagination headers.
	totalCount := rec.Header().Get("X-Total-Count")
	if totalCount == "" {
		t.Fatal("expected X-Total-Count header")
	}
	total, _ := strconv.Atoi(totalCount)
	if total != 3 {
		t.Fatalf("expected total=3, got %d", total)
	}

	limitHeader := rec.Header().Get("X-Limit")
	if limitHeader != "2" {
		t.Fatalf("expected X-Limit=2, got %s", limitHeader)
	}


	// Should return 2 files (limited).
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 files with limit=2, got %d", len(resp.Data))
	}

	// Second page with offset=2.
	req = authedRequest(http.MethodGet, "/api/v1/admin/files?limit=2&offset=2", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}

	// Should return 1 file (offset=2 skips first 2, only 1 left).
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 file with offset=2, got %d", len(resp.Data))
	}
}

// TestFileCenterNoPathLeak verifies response doesn't include local_path or storage_subdir.
func TestFileCenterNoPathLeak(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	h := NewFileCenterHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteFiles, h.ServeFileCenter)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/files", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Decode response into a map to check for unwanted fields.
	var rawResp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&rawResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := rawResp["data"].([]any)
	if !ok {
		t.Fatal("expected data to be an array")
	}

	for i, fileData := range data {
		fileMap, ok := fileData.(map[string]any)
		if !ok {
			t.Fatalf("expected file %d to be an object", i)
		}

		// Check that local_path is NOT present.
		if _, exists := fileMap["local_path"]; exists {
			t.Fatalf("file %d should not contain local_path field, got: %v", i, fileMap)
		}

		// Check that storage_subdir is NOT present.
		if _, exists := fileMap["storage_subdir"]; exists {
			t.Fatalf("file %d should not contain storage_subdir field, got: %v", i, fileMap)
		}

		// Verify public_token IS present (should be included).
		if _, exists := fileMap["public_token"]; !exists {
			t.Fatalf("file %d should contain public_token field, got: %v", i, fileMap)
		}
	}
}

// TestFileCenterProjectFilterNumeric verifies project filter with numeric ID.
func TestFileCenterProjectFilterNumeric(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Add second project.
	_, err := database.Exec(`INSERT INTO projects (name, display_name, source_type, source_url, latest_version, last_crawled_at)
		VALUES ('grafana', 'Grafana', 'github', 'https://github.com/grafana/grafana', '10.0.0', '2025-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("failed to seed second project: %v", err)
	}

	_, err = database.Exec(`INSERT INTO files (project_id, version, filename, os, arch, ext, size_bytes, download_url, local_path, checksum, status, source_type, category, storage_subdir, public_token)
		VALUES (2, '10.0.0', 'grafana-10.0.0.linux-amd64.tar.gz', 'linux', 'amd64', '.tar.gz', 2048, 'https://example.com/g1.tar.gz', '/tmp/g1.tar.gz', 'grafanahash', 'complete', 'github', 'ops', 'oss', 'grafana-token-123')`)
	if err != nil {
		t.Fatalf("failed to seed grafana file: %v", err)
	}

	h := NewFileCenterHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteFiles, h.ServeFileCenter)
	handler := wrapWithAuth(mux)

	// Filter by project_id=1 (prometheus).
	req := authedRequest(http.MethodGet, "/api/v1/admin/files?project=1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]FileCenterFileResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}

	// Should return only 3 prometheus files.
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 files for project_id=1, got %d", len(resp.Data))
	}
	for _, f := range resp.Data {
		if f.ProjectID != 1 {
			t.Fatalf("expected all files to have project_id=1, got %d for file %d", f.ProjectID, f.ID)
		}
	}
}

// TestFileCenterProjectFilterName verifies project filter with project name.
func TestFileCenterProjectFilterName(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Add second project.
	_, err := database.Exec(`INSERT INTO projects (name, display_name, source_type, source_url, latest_version, last_crawled_at)
		VALUES ('grafana', 'Grafana', 'github', 'https://github.com/grafana/grafana', '10.0.0', '2025-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("failed to seed second project: %v", err)
	}

	_, err = database.Exec(`INSERT INTO files (project_id, version, filename, os, arch, ext, size_bytes, download_url, local_path, checksum, status, source_type, category, storage_subdir, public_token)
		VALUES (2, '10.0.0', 'grafana-10.0.0.linux-amd64.tar.gz', 'linux', 'amd64', '.tar.gz', 2048, 'https://example.com/g1.tar.gz', '/tmp/g1.tar.gz', 'grafanahash', 'complete', 'github', 'ops', 'oss', 'grafana-token-123')`)
	if err != nil {
		t.Fatalf("failed to seed grafana file: %v", err)
	}

	h := NewFileCenterHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteFiles, h.ServeFileCenter)
	handler := wrapWithAuth(mux)

	// Filter by project name "grafana".
	req := authedRequest(http.MethodGet, "/api/v1/admin/files?project=grafana", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]FileCenterFileResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}

	// Should return only 1 grafana file.
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 file for project 'grafana', got %d", len(resp.Data))
	}
	if resp.Data[0].ProjectID != 2 {
		t.Fatalf("expected project_id=2 for grafana file, got %d", resp.Data[0].ProjectID)
	}
}

// TestFileCenterSorting verifies sorting works correctly.
func TestFileCenterSorting(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	h := NewFileCenterHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteFiles, h.ServeFileCenter)
	handler := wrapWithAuth(mux)

	// Sort by filename asc.
	req := authedRequest(http.MethodGet, "/api/v1/admin/files?sort=filename&order=asc", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]FileCenterFileResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}

	if len(resp.Data) < 2 {
		t.Fatalf("expected at least 2 files for sorting test, got %d", len(resp.Data))
	}

	// Verify ascending order.
	for i := 1; i < len(resp.Data); i++ {
		if resp.Data[i-1].Filename > resp.Data[i].Filename {
			t.Fatalf("expected filenames in ascending order, got %q before %q", resp.Data[i-1].Filename, resp.Data[i].Filename)
		}
	}

	// Sort by filename desc.
	req = authedRequest(http.MethodGet, "/api/v1/admin/files?sort=filename&order=desc", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify descending order.
	for i := 1; i < len(resp.Data); i++ {
		if resp.Data[i-1].Filename < resp.Data[i].Filename {
			t.Fatalf("expected filenames in descending order, got %q before %q", resp.Data[i-1].Filename, resp.Data[i].Filename)
		}
	}
}
