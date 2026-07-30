package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

func setupWebDAVAdminHandler(t *testing.T) (*WebDAVAdminHandler, *config.Config) {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Auth: config.AuthConfig{
			PasswordHash: "$2a$10$abcdefghijklmnopqrstuvwxABCDEFGH",
			JWTSecret:    "test-jwt-secret-key-12345",
		},
		Storage: config.StorageConfig{BasePath: tmpDir},
		Server:  config.ServerConfig{Port: 9090},
	}
	resolver := service.NewStorageResolver(cfg)
	return NewWebDAVAdminHandler(cfg, resolver), cfg
}

func registerWebDAVAdminRoutes(mux *http.ServeMux, h *WebDAVAdminHandler) {
	mux.HandleFunc("GET "+model.RouteAdminWebDAVStatus, h.WebDAVStatus)
	mux.HandleFunc("GET "+model.RouteAdminWebDAVList, h.WebDAVFileList)
}

func TestWebDAVStatus(t *testing.T) {
	h, _ := setupWebDAVAdminHandler(t)

	mux := http.NewServeMux()
	registerWebDAVAdminRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/webdav/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.WebDAVStatusResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if !resp.Data.Enabled {
		t.Fatal("expected enabled=true")
	}
	if resp.Data.HTTPURL == "" {
		t.Fatal("expected non-empty HTTP URL")
	}
}

// TestWebDAVStatus_UsesRequestHost verifies the generated URLs reflect the
// hostname the admin is browsing from, not the server's bind address. The
// connection guide is pasted into OTHER machines, so localhost (the default
// when bound to 0.0.0.0) is useless — r.Host is reachable externally by
// definition.
func TestWebDAVStatus_UsesRequestHost(t *testing.T) {
	h, _ := setupWebDAVAdminHandler(t)

	mux := http.NewServeMux()
	registerWebDAVAdminRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/webdav/status", nil)
	req.Host = "192.168.63.32:9090" // simulate browsing from the device's LAN IP
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.WebDAVStatusResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	wantURL := "http://192.168.63.32:9090/webdav/"
	if resp.Data.HTTPURL != wantURL {
		t.Fatalf("HTTPURL = %q, want %q (should use the request host, not localhost)", resp.Data.HTTPURL, wantURL)
	}
}

func TestWebDAVFileList_Empty(t *testing.T) {
	h, _ := setupWebDAVAdminHandler(t)

	mux := http.NewServeMux()
	registerWebDAVAdminRoutes(mux, h)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/admin/webdav/files", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[webdavListingResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	// Empty directory should return empty array, not nil
	if resp.Data.Files == nil {
		t.Fatal("expected non-nil files array")
	}
}
