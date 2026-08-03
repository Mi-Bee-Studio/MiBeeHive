package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// TestOldWebdavRedirect verifies the legacy /webdav/ root redirects (301) to
// the new default view /webdav/public/default/.
func TestOldWebdavRedirect(t *testing.T) {
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux := http.NewServeMux()
	mux.Handle("GET /webdav/", WebDAVRedirectHandler(stub))
	mux.Handle("/webdav/", stub)

	req := httptest.NewRequest(http.MethodGet, "/webdav/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/webdav/public/default/" {
		t.Fatalf("expected Location /webdav/public/default/, got %q", loc)
	}
}

// TestOldWebdavRedirect_SubpathPassesThrough verifies that sub-paths under
// /webdav/ are NOT redirected and continue to reach the underlying handler.
func TestOldWebdavRedirect_SubpathPassesThrough(t *testing.T) {
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux := http.NewServeMux()
	mux.Handle("GET /webdav/", WebDAVRedirectHandler(stub))
	mux.Handle("/webdav/", stub)

	req := httptest.NewRequest(http.MethodGet, "/webdav/public/default/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sub-path, got %d", rec.Code)
	}
}

// TestOldJWTDownloadWorks verifies the legacy JWT download endpoint still works
// and now carries the Deprecation header.
func TestOldJWTDownloadWorks(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "testfile.tar.gz")
	if err := os.WriteFile(testFile, []byte("test file content here"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if _, err := database.Exec("UPDATE files SET local_path = ? WHERE id = 1", testFile); err != nil {
		t.Fatalf("failed to update file path: %v", err)
	}

	fileService := service.NewFileService(database, service.NewStorageResolver(&config.Config{Storage: config.StorageConfig{BasePath: tmpDir}}), 2, nil)
	h := NewFileHandler(database, fileService, testJWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteFileDownload, h.Download)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, "/api/v1/files/1/download", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if dep := rec.Header().Get("Deprecation"); dep != "true" {
		t.Fatalf("expected Deprecation header 'true', got %q", dep)
	}
	if warn := rec.Header().Get("Warning"); warn == "" {
		t.Fatal("expected Warning header")
	}
}