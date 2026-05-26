package backup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

func TestUploadBackup_Success(t *testing.T) {
	var storedBody []byte
	var storedLen int64
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case "MKCOL":
			w.WriteHeader(http.StatusCreated)
		case "PUT":
			storedLen = r.ContentLength
			buf := make([]byte, r.ContentLength)
			n, _ := r.Body.Read(buf)
			storedBody = buf[:n]
			w.WriteHeader(http.StatusCreated)
		case "HEAD":
			w.Header().Set("Content-Length", strconv.Itoa(len(storedBody)))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer ts.Close()

	// Create a temp file to upload.
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "test-backup.db")
	content := []byte("this is a backup file with some data")
	if err := os.WriteFile(localFile, content, 0o644); err != nil {
		t.Fatal(err)
	}

	remoteURL := ts.URL + "/backups/test-backup.db"
	err := UploadBackup(context.Background(), localFile, remoteURL, "user", "pass")
	if err != nil {
		t.Fatalf("UploadBackup: %v", err)
	}

	// Verify stored content matches.
	if string(storedBody) != string(content) {
		t.Errorf("stored body mismatch: got %q, want %q", string(storedBody), string(content))
	}
	if storedLen != int64(len(content)) {
		t.Errorf("content length mismatch: got %d, want %d", storedLen, len(content))
	}
}

func TestUploadBackup_MKCOLFailureNonFatal(t *testing.T) {
	mkcolCalled := false
	putCalled := false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "MKCOL":
			mkcolCalled = true
			// Return 500 — simulates MKCOL failure.
			w.WriteHeader(http.StatusInternalServerError)
		case "PUT":
			putCalled = true
			w.WriteHeader(http.StatusCreated)
		case "HEAD":
			w.Header().Set("Content-Length", strconv.Itoa(5))
		}
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "backup.db")
	if err := os.WriteFile(localFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := UploadBackup(context.Background(), localFile, ts.URL+"/dir/backup.db", "user", "pass")
	if err != nil {
		t.Fatalf("MKCOL failure should not be fatal: %v", err)
	}
	if !mkcolCalled {
		t.Error("MKCOL should have been called")
	}
	if !putCalled {
		t.Error("PUT should have been called despite MKCOL failure")
	}
}

func TestUploadBackup_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal server error"))
		}
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "backup.db")
	if err := os.WriteFile(localFile, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := UploadBackup(context.Background(), localFile, ts.URL+"/backup.db", "user", "pass")
	if err == nil {
		t.Fatal("expected error for server 500 response")
	}
}

func TestUploadBackup_SizeMismatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "MKCOL":
			w.WriteHeader(http.StatusCreated)
		case "PUT":
			w.WriteHeader(http.StatusCreated)
		case "HEAD":
			// Return wrong content length.
			w.Header().Set("Content-Length", "999")
		}
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "backup.db")
	if err := os.WriteFile(localFile, []byte("small"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := UploadBackup(context.Background(), localFile, ts.URL+"/backups/backup.db", "user", "pass")
	if err == nil {
		t.Fatal("expected error for size mismatch")
	}
}

func TestUploadBackup_FileNotFound(t *testing.T) {
	err := UploadBackup(context.Background(), "/nonexistent/file.db", "http://example.com/file.db", "u", "p")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestUploadBackup_StatusNoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PUT":
			w.WriteHeader(http.StatusNoContent)
		case "HEAD":
			w.Header().Set("Content-Length", strconv.Itoa(4))
		}
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "backup.db")
	if err := os.WriteFile(localFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := UploadBackup(context.Background(), localFile, ts.URL+"/backup.db", "user", "pass")
	if err != nil {
		t.Fatalf("StatusNoContent should be accepted: %v", err)
	}
}

func TestParentDirURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://host/path/to/file.db", "https://host/path/to"},
		{"http://example.com/backups/mibeehive-20260101-030000.db", "http://example.com/backups"},
		{"https://host/file.db", ""},
		{"https://host/", ""},
		{"https://host", ""},
		{"/path/to/file", "/path/to"},
		{"file.db", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parentDirURL(tt.input)
			if got != tt.want {
				t.Errorf("parentDirURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestUploadBackup_ContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block forever — context should cancel.
		select {}
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "backup.db")
	if err := os.WriteFile(localFile, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	err := UploadBackup(ctx, localFile, ts.URL+"/backup.db", "user", "pass")
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}
