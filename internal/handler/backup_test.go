package handler

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupBackupHandler creates a BackupHandler with temp directories for testing.
func setupBackupHandler(t *testing.T) (*BackupHandler, string, string) {
	t.Helper()
	backupDir := t.TempDir()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "test.db")
	// Create the db file so restore can copy it.
	if err := os.WriteFile(dbPath, []byte("fake-db-content"), 0644); err != nil {
		t.Fatalf("failed to create fake db: %v", err)
	}
	h := NewBackupHandler(backupDir, dbPath, nil)
	return h, backupDir, dbPath
}

func TestListBackups_Success(t *testing.T) {
	h, backupDir, _ := setupBackupHandler(t)

	// Create some backup files.
	os.WriteFile(filepath.Join(backupDir, "backup-2025-01-01.db"), []byte("db-content"), 0644)
	os.WriteFile(filepath.Join(backupDir, "backup-2025-01-02.tar.gz"), []byte("tar-content"), 0644)
	// Non-backup file should be ignored.
	os.WriteFile(filepath.Join(backupDir, "notes.txt"), []byte("not a backup"), 0644)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminBackupList, h.ListBackups)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminBackupList, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]backupEntry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 backups, got %d", len(resp.Data))
	}
}

func TestListBackups_Empty(t *testing.T) {
	h, backupDir, _ := setupBackupHandler(t)

	// Remove the backup dir to test non-existent path handling.
	os.RemoveAll(backupDir)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminBackupList, h.ListBackups)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminBackupList, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]backupEntry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected 0 backups for empty dir, got %d", len(resp.Data))
	}
}

func TestRestoreBackup_Success(t *testing.T) {
	backupDir := t.TempDir()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "test.db")
	if err := os.WriteFile(dbPath, []byte("fake-db-content"), 0644); err != nil {
		t.Fatalf("failed to create fake db: %v", err)
	}

	// Track if shutdown was requested.
	shutdownCalled := false
	shutdownCh := make(chan struct{}, 1)
	h := NewBackupHandler(backupDir, dbPath, func() {
		shutdownCalled = true
		shutdownCh <- struct{}{}
	})

	// Create a .db backup file.
	backupFile := filepath.Join(backupDir, "restore-test.db")
	os.WriteFile(backupFile, []byte("restored-db-content"), 0644)

	body, _ := json.Marshal(restoreRequest{Filename: "restore-test.db"})
	req := authedRequest(http.MethodPost, model.RouteAdminBackupRestore, body)
	rec := httptest.NewRecorder()
	h.RestoreBackup(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Message != "restore completed, server restarting" {
		t.Fatalf("unexpected message: %s", resp.Message)
	}

	// Verify shutdown function was called.
	select {
	case <-shutdownCh:
		// Expected — shutdown was triggered.
	case <-time.After(time.Second):
		t.Fatal("expected shutdown function to be called")
	}
	if !shutdownCalled {
		t.Fatal("expected shutdownCalled to be true")
	}

	// Verify DB file was actually restored.
	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("failed to read restored db: %v", err)
	}
	if string(data) != "restored-db-content" {
		t.Fatalf("expected 'restored-db-content', got %q", string(data))
	}
}

func TestRestoreBackup_InvalidFilename(t *testing.T) {
	h, _, _ := setupBackupHandler(t)

	tests := []struct {
		name     string
		filename string
		wantCode int
	}{
		{"empty filename", "", http.StatusBadRequest},
		{"path traversal", "../etc/passwd", http.StatusBadRequest},
		{"nested path", "sub/backup.db", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(restoreRequest{Filename: tt.filename})
			req := authedRequest(http.MethodPost, model.RouteAdminBackupRestore, body)

			rec := httptest.NewRecorder()
			h.RestoreBackup(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("expected %d, got %d: %s", tt.wantCode, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRestoreBackup_FileNotFound(t *testing.T) {
	h, _, _ := setupBackupHandler(t)

	body, _ := json.Marshal(restoreRequest{Filename: "nonexistent.db"})
	req := authedRequest(http.MethodPost, model.RouteAdminBackupRestore, body)

	rec := httptest.NewRecorder()
	h.RestoreBackup(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRestoreBackup_InvalidBody(t *testing.T) {
	h, _, _ := setupBackupHandler(t)

	req := authedRequest(http.MethodPost, model.RouteAdminBackupRestore, []byte("not json"))

	rec := httptest.NewRecorder()
	h.RestoreBackup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRestoreFromTarGz(t *testing.T) {
	h, backupDir, dbPath := setupBackupHandler(t)

	// Create a .tar.gz archive containing a .db file.
	archivePath := filepath.Join(backupDir, "test-archive.tar.gz")
	if err := createTarGzWithDB(t, archivePath, "test.db", []byte("tar-db-content")); err != nil {
		t.Fatalf("failed to create test archive: %v", err)
	}

	if err := h.restoreFromTarGz(archivePath); err != nil {
		t.Fatalf("restoreFromTarGz failed: %v", err)
	}

	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("failed to read restored db: %v", err)
	}
	if string(data) != "tar-db-content" {
		t.Fatalf("expected 'tar-db-content', got %q", string(data))
	}
}

func TestRestoreFromTarGz_NoDBInArchive(t *testing.T) {
	h, backupDir, _ := setupBackupHandler(t)

	// Create a .tar.gz without a .db file.
	archivePath := filepath.Join(backupDir, "empty-archive.tar.gz")
	if err := createTarGzWithDB(t, archivePath, "readme.txt", []byte("not a db")); err != nil {
		t.Fatalf("failed to create test archive: %v", err)
	}

	err := h.restoreFromTarGz(archivePath)
	if err == nil {
		t.Fatal("expected error when no .db file in archive")
	}
}

// createTarGzWithDB creates a .tar.gz archive containing a single file.
func createTarGzWithDB(t *testing.T, archivePath, entryName string, content []byte) error {
	t.Helper()
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	hdr := &tar.Header{
		Name: entryName,
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(content); err != nil {
		return err
	}
	return nil
}
