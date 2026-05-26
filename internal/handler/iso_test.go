package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

func setupISOHandler(t *testing.T) (*ISOHandler, string) {
	t.Helper()
	tmpDir := t.TempDir()
	isoDir := filepath.Join(tmpDir, "os-install")
	if err := os.MkdirAll(isoDir, 0755); err != nil {
		t.Fatalf("failed to create iso dir: %v", err)
	}
	resolver := service.NewStorageResolver(&config.Config{Storage: config.StorageConfig{BasePath: tmpDir}})
	isoService := service.NewISOService(resolver, 1, nil)
	h := NewISOHandler(isoService, nil, testJWTSecret)
	return h, isoDir
}

func TestTriggerDownload_MissingFields(t *testing.T) {
	h, _ := setupISOHandler(t)

	tests := []struct {
		name     string
		body     isoDownloadRequest
		wantCode int
	}{
		{"empty body", isoDownloadRequest{}, http.StatusBadRequest},
		{"missing url", isoDownloadRequest{Filename: "test.iso"}, http.StatusBadRequest},
		{"missing filename", isoDownloadRequest{URL: "https://example.com/test.iso"}, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := authedRequest(http.MethodPost, model.RouteAdminISODownload, body)
			rec := httptest.NewRecorder()

			mux := http.NewServeMux()
			mux.HandleFunc("POST "+model.RouteAdminISODownload, h.TriggerDownload)
			handler := wrapWithAuth(mux)
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("expected %d, got %d: %s", tt.wantCode, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestTriggerDownload_PathTraversal(t *testing.T) {
	h, _ := setupISOHandler(t)

	tests := []struct {
		name     string
		filename string
		wantCode int
	}{
		{"double dot", "../etc/passwd", http.StatusBadRequest},
		{"forward slash", "sub/test.iso", http.StatusBadRequest},
		{"backslash", `sub\test.iso`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(isoDownloadRequest{
				Filename: tt.filename,
				URL:      "https://example.com/test.iso",
			})
			req := authedRequest(http.MethodPost, model.RouteAdminISODownload, body)
			rec := httptest.NewRecorder()

			h.TriggerDownload(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("expected %d, got %d: %s", tt.wantCode, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestTriggerDownload_InvalidURL(t *testing.T) {
	h, _ := setupISOHandler(t)

	body, _ := json.Marshal(isoDownloadRequest{
		Filename: "test.iso",
		URL:      "ftp://example.com/test.iso",
	})
	req := authedRequest(http.MethodPost, model.RouteAdminISODownload, body)
	rec := httptest.NewRecorder()

	h.TriggerDownload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-http URL, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTriggerDownload_InvalidBody(t *testing.T) {
	h, _ := setupISOHandler(t)

	req := authedRequest(http.MethodPost, model.RouteAdminISODownload, []byte("not json"))
	rec := httptest.NewRecorder()

	h.TriggerDownload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListISOs_Success(t *testing.T) {
	h, isoDir := setupISOHandler(t)

	// Create some ISO files.
	os.WriteFile(filepath.Join(isoDir, "ubuntu-22.04.iso"), []byte("ubuntu-content"), 0644)
	os.WriteFile(filepath.Join(isoDir, "debian-12.iso"), []byte("debian-content"), 0644)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminISOsList, h.ListISOs)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminISOsList, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]service.ISOInfo]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 ISOs, got %d", len(resp.Data))
	}
}

func TestListISOs_Empty(t *testing.T) {
	h, _ := setupISOHandler(t)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminISOsList, h.ListISOs)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminISOsList, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]service.ISOInfo]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected 0 ISOs, got %d", len(resp.Data))
	}
}

func TestDeleteISO_Success(t *testing.T) {
	h, isoDir := setupISOHandler(t)

	// Create an ISO file to delete.
	isoFile := filepath.Join(isoDir, "to-delete.iso")
	os.WriteFile(isoFile, []byte("delete-me"), 0644)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE "+model.RouteAdminISODelete, h.DeleteISO)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodDelete, "/api/v1/admin/os-install/isos/to-delete.iso", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify file is gone.
	if _, err := os.Stat(isoFile); !os.IsNotExist(err) {
		t.Fatal("expected ISO file to be deleted")
	}
}

func TestDeleteISO_NotFound(t *testing.T) {
	h, _ := setupISOHandler(t)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE "+model.RouteAdminISODelete, h.DeleteISO)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodDelete, "/api/v1/admin/os-install/isos/nonexistent.iso", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
