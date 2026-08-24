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
		Server:  config.ServerConfig{Port: 9090, HTTPSPort: 9443},
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
	if resp.Data.HTTPSURL == "" {
		t.Fatal("expected non-empty HTTPS URL")
	}
}

// TestWebDAVStatus_NoHTTPSPort verifies that when no HTTPS port is configured
// the service reports itself as disabled: WebDAV is HTTPS-only, so without
// the HTTPS listener there is no reachable endpoint at all.
func TestWebDAVStatus_NoHTTPSPort(t *testing.T) {
	h, _ := setupWebDAVAdminHandler(t)
	h.config.Server.HTTPSPort = 0

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
	if resp.Data.Enabled {
		t.Fatal("expected enabled=false when HTTPS port is not configured")
	}
	if resp.Data.HTTPSURL != "" {
		t.Fatalf("expected empty HTTPS URL, got %q", resp.Data.HTTPSURL)
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
	wantURL := "https://192.168.63.32:9443/webdav/"
	if resp.Data.HTTPSURL != wantURL {
		t.Fatalf("HTTPSURL = %q, want %q (should use the request host, not localhost)", resp.Data.HTTPSURL, wantURL)
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
