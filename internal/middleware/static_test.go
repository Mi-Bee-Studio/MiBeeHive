package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testBody is a reusable response body for tests.
const testBody = "Hello, World!"

// okHandler returns a simple 200 handler that writes testBody.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testBody))
	})
}

func TestCacheAndGzip_CSS_CacheControl(t *testing.T) {
	handler := CacheAndGzip(okHandler())

	req := httptest.NewRequest("GET", "/style.css", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc != "public, max-age=86400" {
		t.Errorf("expected Cache-Control 'public, max-age=86400' for CSS, got %q", cc)
	}
}

func TestCacheAndGzip_JS_CacheControl(t *testing.T) {
	handler := CacheAndGzip(okHandler())

	req := httptest.NewRequest("GET", "/app.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc != "public, max-age=86400" {
		t.Errorf("expected Cache-Control 'public, max-age=86400' for JS, got %q", cc)
	}
}

func TestCacheAndGzip_HTML_NoCache(t *testing.T) {
	handler := CacheAndGzip(okHandler())

	req := httptest.NewRequest("GET", "/index.html", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("expected Cache-Control 'no-cache' for HTML, got %q", cc)
	}
}

func TestCacheAndGzip_RootPath_NoCache(t *testing.T) {
	handler := CacheAndGzip(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("expected Cache-Control 'no-cache' for root path, got %q", cc)
	}
}

func TestCacheAndGzip_GzipCompression(t *testing.T) {
	body := strings.Repeat("x", 1000)
	handler := CacheAndGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))

	req := httptest.NewRequest("GET", "/app.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Error("expected Content-Encoding: gzip")
	}
	if rec.Header().Get("Vary") != "Accept-Encoding" {
		t.Error("expected Vary: Accept-Encoding")
	}

	gr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	decompressed, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("failed to read gzip body: %v", err)
	}
	if string(decompressed) != body {
		t.Errorf("decompressed body mismatch")
	}
}

func TestCacheAndGzip_NoGzipWithoutAcceptEncoding(t *testing.T) {
	handler := CacheAndGzip(okHandler())

	req := httptest.NewRequest("GET", "/app.js", nil)
	// No Accept-Encoding header.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Error("should not gzip when Accept-Encoding is not set")
	}
	if rec.Body.String() != testBody {
		t.Errorf("expected plain body, got %q", rec.Body.String())
	}
}

func TestCacheAndGzip_SVG_GzipCompression(t *testing.T) {
	handler := CacheAndGzip(okHandler())

	req := httptest.NewRequest("GET", "/icon.svg", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Error("expected gzip compression for SVG")
	}
}

func TestCacheAndGzip_NonTextFile_NoGzip(t *testing.T) {
	handler := CacheAndGzip(okHandler())

	req := httptest.NewRequest("GET", "/image.png", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") == "gzip" {
		t.Error("should not gzip non-text files like PNG")
	}
}
