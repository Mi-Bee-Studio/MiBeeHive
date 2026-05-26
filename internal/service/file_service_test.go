package service

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database with the files table.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	d.SetMaxOpenConns(1) // :memory: DBs don't share across connections

	// Create files table matching the schema.
	_, err = d.Exec(`
		CREATE TABLE files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL,
			version TEXT NOT NULL DEFAULT '',
			filename TEXT NOT NULL,
			os TEXT NOT NULL DEFAULT '',
			arch TEXT NOT NULL DEFAULT '',
			ext TEXT NOT NULL DEFAULT '',
			size_bytes INTEGER NOT NULL DEFAULT 0,
			download_url TEXT NOT NULL DEFAULT '',
			local_path TEXT NOT NULL DEFAULT '',
			checksum TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			error_message TEXT NOT NULL DEFAULT '',
			retry_count INTEGER NOT NULL DEFAULT 0,
			last_attempt_at DATETIME,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	return d
}

// insertTestFile inserts a file record and returns it as a model.File.
func insertTestFile(t *testing.T, d *sql.DB, filename, downloadURL, localPath string) *model.File {
	t.Helper()
	repo := db.NewFileRepo(d)
	f := &db.File{
		ProjectID:   1,
		Filename:    filename,
		DownloadURL: downloadURL,
		LocalPath:   localPath,
		Status:      string(model.FileStatusPending),
	}
	id, err := repo.Create(context.Background(), f)
	if err != nil {
		t.Fatalf("insert file: %v", err)
	}
	return &model.File{
		ID:          id,
		ProjectID:   1,
		Filename:    filename,
		DownloadURL: downloadURL,
		LocalPath:   localPath,
		Status:      model.FileStatusPending,
	}
}

// mustCreateTestTarGz returns a minimal valid tar.gz as bytes.
func mustCreateTestTarGz(t *testing.T) []byte {
	t.Helper()
	var buf []byte
	pr, pw := io.Pipe()
	go func() {
		gw := gzip.NewWriter(pw)
		tw := tar.NewWriter(gw)
		hdr := &tar.Header{Name: "test.txt", Mode: 0600, Size: int64(5)}
		if err := tw.WriteHeader(hdr); err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := tw.Write([]byte("hello")); err != nil {
			pw.CloseWithError(err)
			return
		}
		tw.Close()
		gw.Close()
		pw.Close()
	}()
	buf, err := io.ReadAll(pr)
	if err != nil {
		t.Fatal(err)
	}
	return buf
}

func TestDownloadFile(t *testing.T) {
	content := mustCreateTestTarGz(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	testDB := setupTestDB(t)
	localPath := filepath.Join(tmpDir, "testfile.tar.gz")

	mf := insertTestFile(t, testDB, "testfile.tar.gz", server.URL+"/testfile.tar.gz", localPath)

	svc := NewFileService(testDB, tmpDir, 2, nil)
	err := svc.DownloadFile(context.Background(), mf)
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}

	// Verify file exists on disk.
	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("file not found: %v", err)
	}
	if string(data) != string(content) {
		t.Fatalf("content mismatch: got %q, want %q", string(data), string(content))
	}

	// Verify DB status updated to complete.
	repo := db.NewFileRepo(testDB)
	f, err := repo.GetByID(context.Background(), mf.ID)
	if err != nil {
		t.Fatalf("get file: %v", err)
	}
	if f.Status != string(model.FileStatusComplete) {
		t.Fatalf("status = %q, want %q", f.Status, model.FileStatusComplete)
	}
}

func TestDownloadFile_TempFileCleanupOnSuccess(t *testing.T) {
	content := mustCreateTestTarGz(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	testDB := setupTestDB(t)
	localPath := filepath.Join(tmpDir, "myapp.tar.gz")

	mf := insertTestFile(t, testDB, "myapp.tar.gz", server.URL+"/myapp.tar.gz", localPath)

	svc := NewFileService(testDB, tmpDir, 2, nil)
	err := svc.DownloadFile(context.Background(), mf)
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}

	// Temp file should NOT exist.
	tempPath := filepath.Join(tmpDir, ".download-myapp.tar.gz")
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("temp file still exists: %v", err)
	}

	// Final file should exist.
	if _, err := os.Stat(localPath); err != nil {
		t.Fatalf("final file missing: %v", err)
	}
}

func TestVerifyIntegrity_ValidTarGz(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "valid.tar.gz")

	// Create a valid tar.gz file.
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{Name: "test.txt", Mode: 0600, Size: int64(len("hello"))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	tw.Write([]byte("hello"))
	tw.Close()
	gw.Close()
	f.Close()

	svc := NewFileService(nil, tmpDir, 2, nil)
	if err := svc.VerifyIntegrity(path); err != nil {
		t.Fatalf("VerifyIntegrity valid tar.gz failed: %v", err)
	}
}

func TestVerifyIntegrity_InvalidTarGz(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.tar.gz")

	// Write garbage data with .tar.gz extension.
	if err := os.WriteFile(path, []byte("this is not a tar.gz"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewFileService(nil, tmpDir, 2, nil)
	err := svc.VerifyIntegrity(path)
	if err == nil {
		t.Fatal("VerifyIntegrity should have failed for invalid tar.gz")
	}
}

func TestVerifyIntegrity_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.tar.gz")

	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewFileService(nil, tmpDir, 2, nil)
	err := svc.VerifyIntegrity(path)
	if err == nil {
		t.Fatal("VerifyIntegrity should have failed for empty file")
	}
}

func TestVerifyIntegrity_NonExistent(t *testing.T) {
	svc := NewFileService(nil, t.TempDir(), 2, nil)
	err := svc.VerifyIntegrity("/nonexistent/path/file.tar.gz")
	if err == nil {
		t.Fatal("VerifyIntegrity should have failed for non-existent file")
	}
}

func TestCheckDiskSpace(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewFileService(nil, tmpDir, 2, nil)

	// Requesting 0 bytes should always succeed.
	if err := svc.CheckDiskSpace(0); err != nil {
		t.Fatalf("CheckDiskSpace(0) failed: %v", err)
	}

	// Requesting 1 byte should succeed on any filesystem.
	if err := svc.CheckDiskSpace(1); err != nil {
		t.Fatalf("CheckDiskSpace(1) failed: %v", err)
	}

	// Requesting an absurdly large amount should fail.
	if err := svc.CheckDiskSpace(int64(1) << 62); err == nil {
		t.Fatal("CheckDiskSpace(huge) should have failed")
	}
}

func TestGetDiskUsage(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewFileService(nil, tmpDir, 2, nil)

	total, used, avail, err := svc.GetDiskUsage(tmpDir)
	if err != nil {
		t.Fatalf("GetDiskUsage failed: %v", err)
	}
	if total <= 0 {
		t.Fatalf("total=%d, expected > 0", total)
	}
	if avail <= 0 {
		t.Fatalf("avail=%d, expected > 0", avail)
	}
	if used < 0 {
		t.Fatalf("used=%d, expected >= 0", used)
	}
}

func TestConcurrentLimit(t *testing.T) {
	var running int32
	var maxRunning int32
	var mu sync.Mutex
	var maxSeen int

	// Server that tracks concurrent requests.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&running, 1)

		mu.Lock()
		if int(cur) > maxSeen {
			maxSeen = int(cur)
		}
		mu.Unlock()

		atomic.StoreInt32(&maxRunning, cur)

		// Hold the connection open briefly.
		time.Sleep(200 * time.Millisecond)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))

		atomic.AddInt32(&running, -1)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	testDB := setupTestDB(t)
	svc := NewFileService(testDB, tmpDir, 2, nil) // maxConcurrent = 2

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			localPath := filepath.Join(tmpDir, fmt.Sprintf("file%d.bin", i))
			mf := insertTestFile(t, testDB, fmt.Sprintf("file%d.bin", i),
				server.URL+fmt.Sprintf("/file%d.bin", i), localPath)
			_ = svc.DownloadFile(context.Background(), mf)
		}(i)
	}
	wg.Wait()

	if maxSeen > 2 {
		t.Fatalf("max concurrent downloads = %d, expected <= 2", maxSeen)
	}
}

func TestRetryOnFailure(t *testing.T) {
	var attempts int32
	validContent := mustCreateTestTarGz(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&attempts, 1)
		if cur < 3 {
			// Fail first 2 attempts with 500.
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(validContent)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	testDB := setupTestDB(t)
	localPath := filepath.Join(tmpDir, "retry.tar.gz")

	mf := insertTestFile(t, testDB, "retry.tar.gz", server.URL+"/retry.tar.gz", localPath)

	// Use very short backoffs for testing.
	origBackoffs := retryBackoffs
	retryBackoffs = []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond}
	defer func() { retryBackoffs = origBackoffs }()

	svc := NewFileService(testDB, tmpDir, 2, nil)
	err := svc.DownloadFile(context.Background(), mf)
	if err != nil {
		t.Fatalf("DownloadFile with retries failed: %v", err)
	}

	if atomic.LoadInt32(&attempts) != 3 {
		t.Fatalf("attempts = %d, expected 3", atomic.LoadInt32(&attempts))
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("file not found: %v", err)
	}
	if string(data) != string(validContent) {
		t.Fatalf("content mismatch: got %d bytes, want %d bytes", len(data), len(validContent))
	}

	// Verify status is complete.
	repo := db.NewFileRepo(testDB)
	f, err := repo.GetByID(context.Background(), mf.ID)
	if err != nil {
		t.Fatalf("get file: %v", err)
	}
	if f.Status != string(model.FileStatusComplete) {
		t.Fatalf("status = %q, want %q", f.Status, model.FileStatusComplete)
	}
}

func TestRetryNoRetryOn4xx(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	testDB := setupTestDB(t)
	localPath := filepath.Join(tmpDir, "fourxx.tar.gz")

	mf := insertTestFile(t, testDB, "fourxx.tar.gz", server.URL+"/fourxx.tar.gz", localPath)

	origBackoffs := retryBackoffs
	retryBackoffs = []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond}
	defer func() { retryBackoffs = origBackoffs }()

	svc := NewFileService(testDB, tmpDir, 2, nil)
	err := svc.DownloadFile(context.Background(), mf)
	if err == nil {
		t.Fatal("DownloadFile should have failed on 404")
	}

	// Should only attempt once (no retry on 4xx).
	if atomic.LoadInt32(&attempts) != 1 {
		t.Fatalf("attempts = %d, expected 1 (no retry on 4xx)", atomic.LoadInt32(&attempts))
	}

	// Verify status is error.
	repo := db.NewFileRepo(testDB)
	f, err := repo.GetByID(context.Background(), mf.ID)
	if err != nil {
		t.Fatalf("get file: %v", err)
	}
	if f.Status != string(model.FileStatusError) {
		t.Fatalf("status = %q, want %q", f.Status, model.FileStatusError)
	}
}

func TestStreamFile(t *testing.T) {
	content := []byte("file content for streaming")
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "stream.txt")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	mf := &model.File{
		Filename:  "stream.txt",
		LocalPath: path,
	}

	svc := NewFileService(nil, tmpDir, 2, nil)

	w := httptest.NewRecorder()
	err := svc.StreamFile(w, mf)
	if err != nil {
		t.Fatalf("StreamFile failed: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.Bytes()
	if string(body) != string(content) {
		t.Fatalf("body = %q, want %q", string(body), string(content))
	}

	// Check headers.
	ct := w.Header().Get("Content-Type")
	if ct != "application/octet-stream" {
		t.Fatalf("Content-Type = %q, want %q", ct, "application/octet-stream")
	}
	cl := w.Header().Get("Content-Length")
	if cl != fmt.Sprintf("%d", len(content)) {
		t.Fatalf("Content-Length = %q, want %q", cl, fmt.Sprintf("%d", len(content)))
	}
	cd := w.Header().Get("Content-Disposition")
	expected := `attachment; filename="stream.txt"`
	if cd != expected {
		t.Fatalf("Content-Disposition = %q, want %q", cd, expected)
	}
}

func TestStreamFile_NotFound(t *testing.T) {
	mf := &model.File{
		Filename:  "missing.txt",
		LocalPath: "/nonexistent/missing.txt",
	}

	svc := NewFileService(nil, t.TempDir(), 2, nil)
	w := httptest.NewRecorder()
	err := svc.StreamFile(w, mf)
	if err == nil {
		t.Fatal("StreamFile should fail for missing file")
	}
}

func TestVerifyIntegrity_ValidZip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "valid.zip")

	// Create a valid zip file.
	createTestZip(t, path, "test.txt", []byte("hello zip"))

	svc := NewFileService(nil, tmpDir, 2, nil)
	if err := svc.VerifyIntegrity(path); err != nil {
		t.Fatalf("VerifyIntegrity valid zip failed: %v", err)
	}
}

func TestVerifyIntegrity_InvalidZip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.zip")
	if err := os.WriteFile(path, []byte("not a zip"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewFileService(nil, tmpDir, 2, nil)
	err := svc.VerifyIntegrity(path)
	if err == nil {
		t.Fatal("VerifyIntegrity should fail for invalid zip")
	}
}

func TestVerifyIntegrity_UnknownExtension(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "data.bin")
	if err := os.WriteFile(path, []byte("binary data"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewFileService(nil, tmpDir, 2, nil)
	if err := svc.VerifyIntegrity(path); err != nil {
		t.Fatalf("VerifyIntegrity should pass for unknown extension: %v", err)
	}
}

// createTestZip creates a valid zip file with one entry.
func createTestZip(t *testing.T, path, name string, data []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadFile_SizeMismatch(t *testing.T) {
	// Server sends actual tar.gz content, but file record expects a larger size.
	content := mustCreateTestTarGz(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	testDB := setupTestDB(t)
	localPath := filepath.Join(tmpDir, "mismatch.tar.gz")

	mf := insertTestFile(t, testDB, "mismatch.tar.gz", server.URL+"/mismatch.tar.gz", localPath)
	mf.SizeBytes = int64(len(content)) * 10 // Expected 10x the actual content.

	svc := NewFileService(testDB, tmpDir, 2, nil)
	err := svc.DownloadFile(context.Background(), mf)
	if err == nil {
		t.Fatal("expected size mismatch error")
	}
	if !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("expected size mismatch error, got: %v", err)
	}
}

func TestFileService_Cancel(t *testing.T) {
	// Create a slow server that sends data in small chunks with delays.
	var received int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send headers first.
		w.Header().Set("Content-Length", "1048576") // 1MB
		// Write slowly so we can cancel mid-download.
		for i := 0; i < 1024; i++ {
			if _, err := w.Write(make([]byte, 1024)); err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			atomic.AddInt32(&received, 1)
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	testDB := setupTestDB(t)
	localPath := filepath.Join(tmpDir, "cancel-test.bin")

	mf := insertTestFile(t, testDB, "cancel-test.bin", server.URL+"/cancel-test.bin", localPath)
	mf.SizeBytes = 0 // Unknown size so no size mismatch

	svc := NewFileService(testDB, tmpDir, 2, nil)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- svc.DownloadFile(ctx, mf)
	}()

	// Give the download time to start and write some data.
	time.Sleep(200 * time.Millisecond)

	// Cancel the context.
	cancel()

	// Download should return an error.
	err := <-errCh
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}

	// Temp file should be cleaned up.
	tempPath := filepath.Join(tmpDir, ".download-cancel-test.bin")
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("temp file should be cleaned up after cancellation, got err: %v", err)
	}

	// Final file should NOT exist (download didn't complete).
	if _, err := os.Stat(localPath); !os.IsNotExist(err) {
		t.Fatalf("final file should not exist after cancellation, got err: %v", err)
	}

	// Shutdown should complete immediately (downloads already stopped).
	svc.Shutdown()
}
