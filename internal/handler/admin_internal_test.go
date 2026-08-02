package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

func TestAdminInternalReturnsPath(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	h := NewAdminInternalHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteFileInternal, h.GetFileInternal)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/files/1/internal", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify the X-Internal marker header is set.
	if rec.Header().Get("X-Internal") != "true" {
		t.Fatalf("expected X-Internal: true header, got %q", rec.Header().Get("X-Internal"))
	}

	var resp model.ApiResponse[FileInternalResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Data.ID != 1 {
		t.Fatalf("expected file id 1, got %d", resp.Data.ID)
	}
	if resp.Data.Filename != "prometheus-2.50.0.linux-arm64.tar.gz" {
		t.Fatalf("unexpected filename: %q", resp.Data.Filename)
	}
	// The physical path must be exposed on this endpoint.
	if resp.Data.LocalPath != "/tmp/p1.tar.gz" {
		t.Fatalf("expected local_path /tmp/p1.tar.gz, got %q", resp.Data.LocalPath)
	}
}

func TestAdminInternalFileNotFound(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	h := NewAdminInternalHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteFileInternal, h.GetFileInternal)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/files/999/internal", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminInternalInvalidID(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	h := NewAdminInternalHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteFileInternal, h.GetFileInternal)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/files/abc/internal", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminInternalNoJWT(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	h := NewAdminInternalHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteFileInternal, h.GetFileInternal)
	handler := wrapWithAuth(mux)

	// No Authorization header → 401.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/files/1/internal", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without JWT, got %d: %s", rec.Code, rec.Body.String())
	}
}