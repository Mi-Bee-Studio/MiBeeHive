package webdav

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"golang.org/x/crypto/bcrypt"
)

// setupWebDAV creates a fully-configured WebDAV HTTP handler with BasicAuth middleware
// and http.StripPrefix("/webdav", ...), matching production wiring.
// Returns the handler and the underlying storage path.
func setupWebDAV(t *testing.T, passwordHash string) (http.Handler, string) {
	t.Helper()
	storagePath := t.TempDir()
	h := NewHandler(storagePath, "/webdav")
	var handler http.Handler = h
	if passwordHash != "" {
		handler = middleware.BasicAuthMiddleware(passwordHash)(handler)
	}
	return http.StripPrefix("/webdav", handler), storagePath
}

func TestNewHandler(t *testing.T) {
	storagePath := t.TempDir()
	h := NewHandler(storagePath, "/webdav")
	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
}

func TestWebDAVHandlerImplementsHTTP(t *testing.T) {
	storagePath := t.TempDir()
	h := NewHandler(storagePath, "/webdav")
	var _ http.Handler = h
}

// --- Anonymous Read Methods ---

func TestWebDAVAnonymousReadMethods(t *testing.T) {
	password := "testpass123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	handler, _ := setupWebDAV(t, string(hash))

	tests := []struct {
		name   string
		method string
	}{
		{"GET", http.MethodGet},
		{"HEAD", http.MethodHead},
		{"OPTIONS", http.MethodOptions},
		{"PROPFIND", "PROPFIND"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/webdav/", nil)
			if tt.method == "PROPFIND" {
				req.Header.Set("Depth", "0")
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code == http.StatusUnauthorized {
				t.Errorf("%s returned 401, want non-401", tt.method)
			}
		})
	}
}

// --- Anonymous Write Blocked ---

func TestWebDAVAnonymousWriteBlocked(t *testing.T) {
	password := "testpass123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	handler, _ := setupWebDAV(t, string(hash))

	tests := []struct {
		name   string
		method string
	}{
		{"PUT", http.MethodPut},
		{"DELETE", http.MethodDelete},
		{"MKCOL", "MKCOL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/webdav/test.txt", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s returned %d, want 401", tt.method, rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got == "" {
				t.Errorf("%s: expected WWW-Authenticate header", tt.method)
			}
		})
	}
}

// --- PUT/GET Round-Trip ---

func TestWebDAVPutGetRoundtrip(t *testing.T) {
	password := "testpass123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	handler, _ := setupWebDAV(t, string(hash))

	content := []byte("Hello, WebDAV!")
	req := httptest.NewRequest(http.MethodPut, "/webdav/roundtrip.txt", bytes.NewReader(content))
	req.SetBasicAuth("admin", password)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Fatal("authenticated PUT rejected with 401")
	}

	// Anonymous GET should retrieve the content
	req = httptest.NewRequest(http.MethodGet, "/webdav/roundtrip.txt", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Fatal("anonymous GET rejected with 401")
	}

	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("failed to read GET body: %v", err)
	}
	if !bytes.Equal(body, content) {
		t.Errorf("GET body = %q, want %q", string(body), string(content))
	}
}

// --- Authenticated Write Methods ---

func TestWebDAVAuthenticatedWriteMethods(t *testing.T) {
	password := "testpass123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	handler, _ := setupWebDAV(t, string(hash))

	// PUT with correct credentials — should succeed
	req := httptest.NewRequest(http.MethodPut, "/webdav/auth_write.txt", bytes.NewReader([]byte("authed content")))
	req.SetBasicAuth("admin", password)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("authenticated PUT incorrectly rejected with 401")
	}

	// DELETE with correct credentials — should succeed
	req = httptest.NewRequest(http.MethodDelete, "/webdav/auth_write.txt", nil)
	req.SetBasicAuth("admin", password)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("authenticated DELETE incorrectly rejected with 401")
	}
}

// --- Wrong Credentials ---

func TestWebDAVWriteWithWrongCredentials(t *testing.T) {
	password := "testpass123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	handler, _ := setupWebDAV(t, string(hash))

	tests := []struct {
		name string
		user string
		pass string
	}{
		{"Wrong password", "admin", "wrongpass"},
		{"Wrong username", "hacker", password},
		{"Empty credentials", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/webdav/test.txt", nil)
			if tt.user != "" || tt.pass != "" {
				req.SetBasicAuth(tt.user, tt.pass)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s returned %d, want 401", tt.name, rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got == "" {
				t.Errorf("%s: expected WWW-Authenticate header", tt.name)
			}
		})
	}
}

// --- MKCOL (Create Directory) ---

func TestWebDAVMKCOL(t *testing.T) {
	password := "testpass123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	handler, _ := setupWebDAV(t, string(hash))

	// MKCOL with correct credentials
	req := httptest.NewRequest("MKCOL", "/webdav/collection_dir", nil)
	req.SetBasicAuth("admin", password)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("authenticated MKCOL incorrectly rejected with 401")
	}

	// PUT a file into the newly created collection
	req = httptest.NewRequest(http.MethodPut, "/webdav/collection_dir/nested.txt", bytes.NewReader([]byte("nested")))
	req.SetBasicAuth("admin", password)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("authenticated PUT into collection rejected with 401")
	}

	// GET the nested file back (anonymous read)
	req = httptest.NewRequest(http.MethodGet, "/webdav/collection_dir/nested.txt", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("anonymous GET of nested file rejected with 401")
	}

	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("failed to read GET body: %v", err)
	}
	if string(body) != "nested" {
		t.Errorf("GET body = %q, want %q", string(body), "nested")
	}

	// DELETE the collection (remove all items first)
	req = httptest.NewRequest(http.MethodDelete, "/webdav/collection_dir/nested.txt", nil)
	req.SetBasicAuth("admin", password)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("authenticated DELETE of nested file rejected with 401")
	}
}

// --- Special Characters in Filenames ---

func TestWebDAVSpecialCharFilenames(t *testing.T) {
	password := "testpass123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	handler, _ := setupWebDAV(t, string(hash))

	tests := []struct {
		name    string
		urlPath string
	}{
		{"Spaces", "/webdav/my%20file.txt"},
		{"Unicode (Chinese)", "/webdav/文件.txt"},
		{"Unicode (Cyrillic)", "/webdav/файл.txt"},
		{"Plus sign", "/webdav/file+name.txt"},
		{"Multiple extensions", "/webdav/archive.tar.gz"},
		{"Dots and dashes", "/webdav/my-file.v1.0.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := []byte("content for " + tt.name)

			// PUT with auth
			req := httptest.NewRequest(http.MethodPut, tt.urlPath, bytes.NewReader(content))
			req.SetBasicAuth("admin", password)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code == http.StatusUnauthorized {
				t.Fatalf("authenticated PUT returned 401 for path %s", tt.urlPath)
			}

			// GET back (anonymous read)
			req = httptest.NewRequest(http.MethodGet, tt.urlPath, nil)
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code == http.StatusUnauthorized {
				t.Fatalf("anonymous GET returned 401 for path %s", tt.urlPath)
			}

			body, err := io.ReadAll(rec.Body)
			if err != nil {
				t.Fatalf("failed to read GET body for %s: %v", tt.name, err)
			}
			if !bytes.Equal(body, content) {
				t.Errorf("%s: GET body = %q, want %q", tt.name, string(body), string(content))
			}
		})
	}
}

// --- StripPrefix Verification ---

func TestWebDAVStripPrefix(t *testing.T) {
	password := "testpass123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	handler, storagePath := setupWebDAV(t, string(hash))

	content := []byte("strip prefix verification")

	// PUT via /webdav/strip_test.txt (handler sees only /strip_test.txt after StripPrefix)
	req := httptest.NewRequest(http.MethodPut, "/webdav/strip_test.txt", bytes.NewReader(content))
	req.SetBasicAuth("admin", password)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("authenticated PUT returned 401")
	}

	// Verify the file is stored at storagePath/strip_test.txt (NOT storagePath/webdav/strip_test.txt)
	expectedPath := filepath.Join(storagePath, "strip_test.txt")
	diskContent, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("file not found at %s — StripPrefix likely not configured correctly: %v", expectedPath, err)
	}
	if !bytes.Equal(diskContent, content) {
		t.Errorf("disk content = %q, want %q", string(diskContent), string(content))
	}

	// Verify the file does NOT exist under webdav subdirectory
	wrongPath := filepath.Join(storagePath, "webdav", "strip_test.txt")
	if _, err := os.Stat(wrongPath); err == nil {
		t.Errorf("file erroneously found at %s — StripPrefix is not stripping the prefix", wrongPath)
	}
}

// --- GET on Non-Existent File ---

func TestWebDAVGetNonExistent(t *testing.T) {
	password := "testpass123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	handler, _ := setupWebDAV(t, string(hash))

	req := httptest.NewRequest(http.MethodGet, "/webdav/nonexistent_file_xyz.txt", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized {
		t.Fatal("anonymous GET of non-existent file rejected with 401")
	}
	// Should get 404 or similar, but must not be 401 (read should pass through auth)
}

// --- Multiple Files and List ---

func TestWebDAVMultipleFiles(t *testing.T) {
	password := "testpass123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	handler, storagePath := setupWebDAV(t, string(hash))

	files := map[string][]byte{
		"file1.txt": []byte("first file"),
		"file2.txt": []byte("second file"),
		"file3.txt": []byte("third file"),
	}

	for name, content := range files {
		req := httptest.NewRequest(http.MethodPut, "/webdav/"+name, bytes.NewReader(content))
		req.SetBasicAuth("admin", password)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized {
			t.Fatalf("PUT %s returned 401", name)
		}
	}

	// Verify all files exist on disk at the correct paths
	for name, content := range files {
		path := filepath.Join(storagePath, name)
		diskContent, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("file %s not found at %s: %v", name, path, err)
		}
		if !bytes.Equal(diskContent, content) {
			t.Errorf("file %s content = %q, want %q", name, string(diskContent), string(content))
		}
	}
}

// --- Size Limit (413) ---

func TestWebDAV_SizeLimit(t *testing.T) {
	password := "testpass123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	handler, _ := setupWebDAV(t, string(hash))

	// PUT with Content-Length exceeding 2GB → 413
	req := httptest.NewRequest(http.MethodPut, "/webdav/bigfile.bin", bytes.NewReader([]byte("x")))
	req.ContentLength = MaxUploadSize + 1 // just over the limit
	req.SetBasicAuth("admin", password)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("PUT with oversized Content-Length returned %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

// --- Concurrency Limit (429) ---

func TestWebDAV_ConcurrencyLimit(t *testing.T) {
	password := "testpass123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	// Create handler directly (not via setupWebDAV) to access its semaphore
	storagePath := t.TempDir()
	h := NewHandler(storagePath, "/webdav")
	var handler http.Handler = h
	handler = middleware.BasicAuthMiddleware(string(hash))(handler)
	handler = http.StripPrefix("/webdav", handler)

	// Fill all 3 semaphore slots
	for i := 0; i < MaxConcurrentUploads; i++ {
		h.putSem <- struct{}{}
	}
	defer func() {
		for i := 0; i < MaxConcurrentUploads; i++ {
			<-h.putSem
		}
	}()

	// 4th PUT should get 429
	req := httptest.NewRequest(http.MethodPut, "/webdav/overflow.txt", bytes.NewReader([]byte("data")))
	req.SetBasicAuth("admin", password)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("4th concurrent PUT returned %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

// --- Destination Header Stripping (ISSUE-002) ---

func TestStripPrefixFromDestination(t *testing.T) {
	tests := []struct {
		name     string
		dest     string
		prefix   string
		want     string
		wantOk   bool
	}{
		{"strip /webdav prefix", "http://example.com/webdav/file.txt", "/webdav", "http://example.com/file.txt", true},
		{"strip /webdav prefix with path only", "/webdav/subdir/file.txt", "/webdav", "/subdir/file.txt", true},
		{"no prefix to strip", "http://example.com/file.txt", "/webdav", "http://example.com/file.txt", false},
		{"empty prefix", "http://example.com/file.txt", "", "http://example.com/file.txt", false},
		{"prefix at end", "http://example.com/webdav", "/webdav", "http://example.com/", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := stripPrefixFromDestination(tt.dest, tt.prefix)
			if ok != tt.wantOk {
				t.Errorf("stripPrefixFromDestination(%q, %q) ok = %v, want %v", tt.dest, tt.prefix, ok, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("stripPrefixFromDestination(%q, %q) = %q, want %q", tt.dest, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestWebDAVCOPYMoveStripsDestination(t *testing.T) {
	password := "testpass123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	handler, _ := setupWebDAV(t, string(hash))

	// Create source file via PUT
	srcContent := []byte("copy source content")
	req := httptest.NewRequest(http.MethodPut, "/webdav/copy_src.txt", bytes.NewReader(srcContent))
	req.SetBasicAuth("admin", password)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatal("PUT for copy source returned 401")
	}

	// COPY with Destination header containing /webdav prefix
	req = httptest.NewRequest("COPY", "/webdav/copy_src.txt", nil)
	req.SetBasicAuth("admin", password)
	req.Header.Set("Destination", "http://example.com/webdav/copy_dst.txt")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should not be 404 or 502 (which indicate unstripped Destination path)
	if rec.Code == http.StatusBadGateway {
		t.Errorf("COPY returned 502 Bad Gateway — Destination header /webdav prefix likely not stripped")
	}
}
