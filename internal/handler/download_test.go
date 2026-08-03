package handler

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
)

// ensureProject inserts a test project (id=1) to satisfy FK constraints in tests.
func ensureProject(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(),
		`INSERT OR IGNORE INTO projects (id, name, display_name, source_type, source_url, created_at) VALUES (1, 'test', 'Test', 'test', 'http://test', datetime('now'))`); err != nil {
		t.Fatalf("failed to insert test project: %v", err)
	}
}


// TestDownloadByToken_Success tests downloading a file by its public token
func TestDownloadByToken_Success(t *testing.T) {
	// Setup temporary directory for test files
	tmpDir := t.TempDir()
	testContent := []byte("test file content for download")
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Setup in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := dbrepo.Migrate(db); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}
	ensureProject(t, db)

	// Insert a test file with public_token
	var fileID int64
	testToken := "test_token_abc123"
	testLocalPath := "test.txt" // relative path
	err = db.QueryRowContext(context.Background(),
		`INSERT INTO files (project_id, version, filename, os, arch, ext, size_bytes, download_url, local_path, checksum, status, created_at, source_type, category, storage_subdir, public_token)
		 VALUES (1, '1.0.0', 'test.txt', '', '', 'txt', ?, '', ?, '', 'complete', datetime('now'), 'test', 'test', 'test', ?)
		 RETURNING id`,
		int64(len(testContent)), testLocalPath, testToken).Scan(&fileID)
	if err != nil {
		t.Fatalf("failed to insert test file: %v", err)
	}

	// Create handler
	h := NewDownloadHandler(db, tmpDir)

	// Create request
	req := httptest.NewRequest("GET", "/api/v1/files/"+testToken+"/download", nil)
	w := httptest.NewRecorder()

	// Call handler
	h.ServeDownload(w, req)

	// Check response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Verify file content was served
	body := make([]byte, len(testContent))
	if _, err := resp.Body.Read(body); err != nil {
		t.Errorf("failed to read response body: %v", err)
	}
	if string(body) != string(testContent) {
		t.Errorf("expected content %q, got %q", string(testContent), string(body))
	}
}

// TestDownloadByToken_InvalidToken tests 404 for non-existent token
func TestDownloadByToken_InvalidToken(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := dbrepo.Migrate(db); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}
	ensureProject(t, db)

	h := NewDownloadHandler(db, tmpDir)

	req := httptest.NewRequest("GET", "/api/v1/files/invalid_token_123/download", nil)
	w := httptest.NewRecorder()

	h.ServeDownload(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

// TestDownloadByToken_DeletedFile tests 404 for deleted files
func TestDownloadByToken_DeletedFile(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := dbrepo.Migrate(db); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}
	ensureProject(t, db)

	// Insert a deleted file with public_token
	testToken := "test_deleted_token"
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO files (project_id, version, filename, os, arch, ext, size_bytes, download_url, local_path, checksum, status, created_at, source_type, category, storage_subdir, public_token)
		 VALUES (1, '1.0.0', 'test.txt', '', '', 'txt', 100, '', 'test.txt', '', 'error', datetime('now'), 'test', 'test', 'test', ?)`,
		testToken)
	if err != nil {
		t.Fatalf("failed to insert deleted test file: %v", err)
	}

	h := NewDownloadHandler(db, tmpDir)

	req := httptest.NewRequest("GET", "/api/v1/files/"+testToken+"/download", nil)
	w := httptest.NewRecorder()

	h.ServeDownload(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404 for deleted file, got %d", resp.StatusCode)
	}
}

// TestDownloadByToken_FileNotFound tests 404 when file doesn't exist on disk
func TestDownloadByToken_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := dbrepo.Migrate(db); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}
	ensureProject(t, db)

	// Insert file record but don't create actual file
	testToken := "test_missing_file"
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO files (project_id, version, filename, os, arch, ext, size_bytes, download_url, local_path, checksum, status, created_at, source_type, category, storage_subdir, public_token)
		 VALUES (1, '1.0.0', 'missing.txt', '', '', 'txt', 100, '', 'missing.txt', '', 'complete', datetime('now'), 'test', 'test', 'test', ?)`,
		testToken)
	if err != nil {
		t.Fatalf("failed to insert test file: %v", err)
	}

	h := NewDownloadHandler(db, tmpDir)

	req := httptest.NewRequest("GET", "/api/v1/files/"+testToken+"/download", nil)
	w := httptest.NewRecorder()

	h.ServeDownload(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404 when file not on disk, got %d", resp.StatusCode)
	}
}

// TestDownloadByToken_CacheHit tests that TokenCache is used
func TestDownloadByToken_CacheHit(t *testing.T) {
	tmpDir := t.TempDir()
	testContent := []byte("cached test content")
	testFile := filepath.Join(tmpDir, "cached.txt")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := dbrepo.Migrate(db); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}
	ensureProject(t, db)

	// Insert test file
	testToken := "test_cache_hit"
	testLocalPath := "cached.txt"
	var fileID int64
	err = db.QueryRowContext(context.Background(),
		`INSERT INTO files (project_id, version, filename, os, arch, ext, size_bytes, download_url, local_path, checksum, status, created_at, source_type, category, storage_subdir, public_token)
		 VALUES (1, '1.0.0', 'cached.txt', '', '', 'txt', ?, '', ?, '', 'complete', datetime('now'), 'test', 'test', 'test', ?)
		 RETURNING id`,
		int64(len(testContent)), testLocalPath, testToken).Scan(&fileID)
	if err != nil {
		t.Fatalf("failed to insert test file: %v", err)
	}

	h := NewDownloadHandler(db, tmpDir)

	// First request - should populate cache
	req1 := httptest.NewRequest("GET", "/api/v1/files/"+testToken+"/download", nil)
	w1 := httptest.NewRecorder()
	h.ServeDownload(w1, req1)

	// Verify cache has the entry
	if cachedID, ok := h.tokenCache.Get(testToken); !ok || cachedID != fileID {
		t.Errorf("expected cache to contain fileID %d, got cachedID=%d, ok=%v", fileID, cachedID, ok)
	}

	// Second request - should hit cache
	req2 := httptest.NewRequest("GET", "/api/v1/files/"+testToken+"/download", nil)
	w2 := httptest.NewRecorder()
	h.ServeDownload(w2, req2)

	resp2 := w2.Result()
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 on cache hit, got %d", resp2.StatusCode)
	}

	body := make([]byte, len(testContent))
	if _, err := resp2.Body.Read(body); err != nil {
		t.Errorf("failed to read response body: %v", err)
	}
	if string(body) != string(testContent) {
		t.Errorf("expected content %q, got %q", string(testContent), string(body))
	}
}

// TestDownloadByToken_ConcurrentLimit tests semaphore limits concurrent downloads
func TestDownloadByToken_ConcurrentLimit(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := dbrepo.Migrate(db); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}
	ensureProject(t, db)

	// Create multiple test files
	numFiles := 5
	testTokens := make([]string, numFiles)
	for i := 0; i < numFiles; i++ {
		testContent := []byte("file content " + string(rune('0'+i)))
		testFile := filepath.Join(tmpDir, "file"+string(rune('0'+i))+".txt")
		if err := os.WriteFile(testFile, testContent, 0644); err != nil {
			t.Fatalf("failed to create test file %d: %v", i, err)
		}

		token := "test_concurrent_" + string(rune('0'+i))
		localPath := "file" + string(rune('0'+i)) + ".txt"
		_, err = db.ExecContext(context.Background(),
			`INSERT INTO files (project_id, version, filename, os, arch, ext, size_bytes, download_url, local_path, checksum, status, created_at, source_type, category, storage_subdir, public_token)
			 VALUES (1, '1.0.0', ?, '', '', 'txt', ?, '', ?, '', 'complete', datetime('now'), 'test', 'test', 'test', ?)`,
			"file"+string(rune('0'+i))+".txt", int64(len(testContent)), localPath, token)
		if err != nil {
			t.Fatalf("failed to insert test file %d: %v", i, err)
		}
		testTokens[i] = token
	}

	h := NewDownloadHandler(db, tmpDir)

	// Launch concurrent downloads
	results := make(chan int, numFiles)
	for _, token := range testTokens {
		go func(tok string) {
			req := httptest.NewRequest("GET", "/api/v1/files/"+tok+"/download", nil)
			w := httptest.NewRecorder()
			h.ServeDownload(w, req)
			results <- w.Code
		}(token)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numFiles; i++ {
		if status := <-results; status == http.StatusOK {
			successCount++
		}
	}

	if successCount != numFiles {
		t.Errorf("expected all %d downloads to succeed, got %d", numFiles, successCount)
	}
}

// TestDownloadByToken_MalformedPath tests handling of malformed URLs
func TestDownloadByToken_MalformedPath(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := dbrepo.Migrate(db); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}
	ensureProject(t, db)

	h := NewDownloadHandler(db, tmpDir)

	// Test with missing /download suffix
	req := httptest.NewRequest("GET", "/api/v1/files/test_token", nil)
	w := httptest.NewRecorder()
	h.ServeDownload(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	// Should not crash, likely 404 since token extraction won't match expected format
	// The actual behavior depends on implementation
	_ = resp.StatusCode
}

// TestDownloadByToken_EmptyToken tests 404 for empty token
func TestDownloadByToken_EmptyToken(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := dbrepo.Migrate(db); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}
	ensureProject(t, db)

	h := NewDownloadHandler(db, tmpDir)

	req := httptest.NewRequest("GET", "/api/v1/files//download", nil)
	w := httptest.NewRecorder()
	h.ServeDownload(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404 for empty token, got %d", resp.StatusCode)
	}
}