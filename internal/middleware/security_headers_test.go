package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders_AddsAllExpectedHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mw := SecurityHeaders(handler)
	req := httptest.NewRequest("GET", "/any-path", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	expected := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}
	for key, want := range expected {
		if got := rec.Header().Get(key); got != want {
			t.Errorf("header %q: want %q, got %q", key, want, got)
		}
	}
}

func TestSecurityHeaders_NoHSTS(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := SecurityHeaders(handler)
	req := httptest.NewRequest("GET", "/any-path", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if h := rec.Header().Get("Strict-Transport-Security"); h != "" {
		t.Errorf("HSTS header must NOT be present when self-signed certs are in use, got: %q", h)
	}
}

func TestSecurityHeaders_WithoutExplicitWriteHeader(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	mw := SecurityHeaders(handler)
	req := httptest.NewRequest("GET", "/any-path", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("header should be set even without WriteHeader, got: %q", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("header should be set even without WriteHeader, got: %q", got)
	}
}

func TestSecurityHeaders_DownstreamCannotRemove(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Del("X-Frame-Options")
		w.WriteHeader(http.StatusOK)
	})

	mw := SecurityHeaders(handler)
	req := httptest.NewRequest("GET", "/any-path", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("header should survive downstream Del(), got: %q", got)
	}
}

func TestSecurityHeaders_PassthroughStatusCode(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	mw := SecurityHeaders(handler)
	req := httptest.NewRequest("GET", "/any-path", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status code should pass through, want 404, got %d", rec.Code)
	}
}

func TestSecurityHeaders_HeadersOnMultipleWrites(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("first"))
		w.Write([]byte("second"))
	})

	mw := SecurityHeaders(handler)
	req := httptest.NewRequest("GET", "/any-path", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("headers must be set on first Write, got: %q", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("default status should be 200, got %d", rec.Code)
	}
}
