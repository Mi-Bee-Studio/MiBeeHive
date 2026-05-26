package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestISOService(t *testing.T) (*ISOService, string) {
	t.Helper()
	dir := t.TempDir()
	svc := NewISOService(dir, 2, nil)
	return svc, dir
}

// --- calcNeededBytes tests ---

func TestCalcNeededBytes(t *testing.T) {
	const gb2 uint64 = 2 * 1024 * 1024 * 1024

	tests := []struct {
		name         string
		totalBytes   int64
		resumeOffset int64
		want         uint64
	}{
		{
			name:       "unknown total falls back to 2GB",
			totalBytes: -1,
			want:       gb2,
		},
		{
			name:       "zero total falls back to 2GB",
			totalBytes: 0,
			want:       gb2,
		},
		{
			name:         "full download of 4GB file",
			totalBytes:   4 * 1024 * 1024 * 1024,
			resumeOffset: 0,
			want:         4 * 1024 * 1024 * 1024,
		},
		{
			name:         "resume with 1GB already downloaded",
			totalBytes:   4 * 1024 * 1024 * 1024,
			resumeOffset: 1 * 1024 * 1024 * 1024,
			want:         3 * 1024 * 1024 * 1024,
		},
		{
			name:         "negative remaining falls back to 2GB",
			totalBytes:   100,
			resumeOffset: 200,
			want:         gb2,
		},
		{
			name:         "small file",
			totalBytes:   1024,
			resumeOffset: 0,
			want:         1024,
		},
		{
			name:         "small file partial resume",
			totalBytes:   1024,
			resumeOffset: 512,
			want:         512,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcNeededBytes(tt.totalBytes, tt.resumeOffset)
			if got != tt.want {
				t.Errorf("calcNeededBytes(%d, %d) = %d, want %d", tt.totalBytes, tt.resumeOffset, got, tt.want)
			}
		})
	}
}

// --- isCorruptionError tests ---

func TestIsCorruptionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "context canceled is transient",
			err:  context.Canceled,
			want: false,
		},
		{
			name: "context deadline exceeded is transient",
			err:  context.DeadlineExceeded,
			want: false,
		},
		{
			name: "wrapped context canceled is transient",
			err:  fmt.Errorf("wrap: %w", context.Canceled),
			want: false,
		},
		{
			name: "generic error is corruption",
			err:  fmt.Errorf("write error"),
			want: true,
		},
		{
			name: "generic wrapped error is corruption",
			err:  fmt.Errorf("outer: %w", errors.New("inner")),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCorruptionError(tt.err)
			if got != tt.want {
				t.Errorf("isCorruptionError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// --- checkDiskSpace tests ---

func TestISOCheckDiskSpace(t *testing.T) {
	svc, _ := newTestISOService(t)

	// Very small value should always pass (temp dir has free space).
	if err := svc.checkDiskSpace(1); err != nil {
		t.Errorf("checkDiskSpace(1) failed: %v", err)
	}

	// Impossibly large value should fail.
	if err := svc.checkDiskSpace(^uint64(0)); err == nil {
		t.Error("checkDiskSpace(maxuint) should have failed")
	}
}

// --- DownloadISO integration tests with httptest ---

func TestDownloadISO_FullDownload(t *testing.T) {
	content := []byte("ISO_CONTENT_HERE_12345")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.Write(content)
	}))
	defer srv.Close()

	svc, dir := newTestISOService(t)
	err := svc.DownloadISO(context.Background(), "test.iso", srv.URL+"/test.iso", "")
	if err != nil {
		t.Fatalf("DownloadISO failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "os-install", "test.iso"))
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("downloaded content mismatch: got %q, want %q", string(data), string(content))
	}
}

func TestDownloadISO_ResumeSupported(t *testing.T) {
	fullContent := []byte("0123456789ABCDEF") // 16 bytes
	existingPart := []byte("01234567")        // first 8 bytes already downloaded
	remainingPart := fullContent[8:]     // bytes 8-15 from full content

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "bytes=8-" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(remainingPart)))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 8-15/%d", len(fullContent)))
			w.WriteHeader(http.StatusPartialContent)
			w.Write(remainingPart)
			return
		}
		// No Range header — full response.
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fullContent)))
		w.Write(fullContent)
	}))
	defer srv.Close()

	svc, dir := newTestISOService(t)

	// Create existing temp file with partial content.
	isoDir := filepath.Join(dir, "os-install")
	if err := os.MkdirAll(isoDir, 0755); err != nil {
		t.Fatal(err)
	}
	tempPath := filepath.Join(isoDir, "resume.iso.tmp")
	if err := os.WriteFile(tempPath, existingPart, 0644); err != nil {
		t.Fatal(err)
	}

	err := svc.DownloadISO(context.Background(), "resume.iso", srv.URL+"/resume.iso", "")
	if err != nil {
		t.Fatalf("DownloadISO resume failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(isoDir, "resume.iso"))
	if err != nil {
		t.Fatalf("reading resumed file: %v", err)
	}
	if string(data) != string(fullContent) {
		t.Errorf("resumed content mismatch: got %q, want %q", string(data), string(fullContent))
	}
}

func TestDownloadISO_ResumeNotSupported(t *testing.T) {
	fullContent := []byte("FULL_CONTENT_DATA")
	existingPart := []byte("PARTIAL")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Server ignores Range header, returns full content with 200.
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fullContent)))
		w.Write(fullContent)
	}))
	defer srv.Close()

	svc, dir := newTestISOService(t)

	// Create existing temp file.
	isoDir := filepath.Join(dir, "os-install")
	if err := os.MkdirAll(isoDir, 0755); err != nil {
		t.Fatal(err)
	}
	tempPath := filepath.Join(isoDir, "fresh.iso.tmp")
	if err := os.WriteFile(tempPath, existingPart, 0644); err != nil {
		t.Fatal(err)
	}

	err := svc.DownloadISO(context.Background(), "fresh.iso", srv.URL+"/fresh.iso", "")
	if err != nil {
		t.Fatalf("DownloadISO fallback failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(isoDir, "fresh.iso"))
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(data) != string(fullContent) {
		t.Errorf("content mismatch: got %q, want %q", string(data), string(fullContent))
	}
}

func TestDownloadISO_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow response — simulate large file.
		w.Header().Set("Content-Length", "10000000")
		w.Write([]byte("start"))
		// Block until client disconnects.
		select {}
	}))
	defer srv.Close()

	svc, _ := newTestISOService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	err := svc.DownloadISO(ctx, "cancel.iso", srv.URL+"/cancel.iso", "")
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestDownloadISO_ProgressTrackingWithResume(t *testing.T) {
	fullContent := []byte("ABCDEFGHIJKLMNOP") // 16 bytes
	existingPart := []byte("ABCD")             // first 4 bytes

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "bytes=4-" {
			w.Header().Set("Content-Length", "12")
			w.Header().Set("Content-Range", "bytes 4-15/16")
			w.WriteHeader(http.StatusPartialContent)
			w.Write(fullContent[4:])
			return
		}
		w.Header().Set("Content-Length", "16")
		w.Write(fullContent)
	}))
	defer srv.Close()

	svc, dir := newTestISOService(t)

	// Pre-create temp file.
	isoDir := filepath.Join(dir, "os-install")
	if err := os.MkdirAll(isoDir, 0755); err != nil {
		t.Fatal(err)
	}
	tempPath := filepath.Join(isoDir, "progress.iso.tmp")
	if err := os.WriteFile(tempPath, existingPart, 0644); err != nil {
		t.Fatal(err)
	}

	err := svc.DownloadISO(context.Background(), "progress.iso", srv.URL+"/progress.iso", "")
	if err != nil {
		t.Fatalf("DownloadISO failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(isoDir, "progress.iso"))
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(data) != string(fullContent) {
		t.Errorf("content mismatch: got %q, want %q", string(data), string(fullContent))
	}
}

func TestDownloadISO_InvalidFilename(t *testing.T) {
	svc, _ := newTestISOService(t)
	err := svc.DownloadISO(context.Background(), "../etc/passwd", "http://example.com/passwd", "")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestDownloadISO_InvalidURL(t *testing.T) {
	svc, _ := newTestISOService(t)
	err := svc.DownloadISO(context.Background(), "test.iso", "ftp://example.com/test.iso", "")
	if err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
}

func TestDownloadISO_DiskSpaceCheck_UsesContentLength(t *testing.T) {
	// Server returns Content-Length for a file.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1024")
		w.Write(make([]byte, 1024))
	}))
	defer srv.Close()

	svc, _ := newTestISOService(t)
	// Should succeed — 1024 bytes is tiny compared to available temp dir space.
	err := svc.DownloadISO(context.Background(), "small.iso", srv.URL+"/small.iso", "")
	if err != nil {
		t.Fatalf("DownloadISO with small file failed: %v", err)
	}
}

func TestDownloadISO_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svc, _ := newTestISOService(t)
	err := svc.DownloadISO(context.Background(), "missing.iso", srv.URL+"/missing.iso", "")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestDownloadISO_EmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "0")
		w.Write([]byte{})
	}))
	defer srv.Close()

	svc, _ := newTestISOService(t)
	err := svc.DownloadISO(context.Background(), "empty.iso", srv.URL+"/empty.iso", "")
	if err == nil {
		t.Fatal("expected error for empty download")
	}
}

func TestDownloadISO_DiskInsufficient(t *testing.T) {
	// Server that advertises a huge Content-Length.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000000000")
		w.Write([]byte("small"))
	}))
	defer srv.Close()

	svc, _ := newTestISOService(t)
	err := svc.DownloadISO(context.Background(), "huge.iso", srv.URL+"/huge.iso", "")
	if err == nil {
		t.Fatal("expected error for insufficient disk space")
	}

	var diskErr *InsufficientStorageError
	if !errors.As(err, &diskErr) {
		t.Fatalf("expected InsufficientStorageError, got %T: %v", err, err)
	}
	if diskErr.Available >= diskErr.Needed {
		t.Fatalf("available (%d) should be less than needed (%d)", diskErr.Available, diskErr.Needed)
	}
}

func TestDownloadISO_Timeout(t *testing.T) {
	// Server that sends a header then blocks, simulating a stalled download.
	blockCh := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10000")
		w.Write([]byte("start"))
		<-blockCh // Block until test signals or closes.
	}))
	defer func() {
		close(blockCh) // Unblock server goroutine on cleanup.
		srv.Close()
	}()

	svc, _ := newTestISOService(t)
	// Use a very short timeout to make the test fast.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := svc.DownloadISO(ctx, "timeout.iso", srv.URL+"/timeout.iso", "")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	// The timeout should propagate as context deadline exceeded.
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Logf("error (acceptable): %v", err)
	}
}
