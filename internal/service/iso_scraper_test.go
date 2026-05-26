package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestScrapeLatestISO_ValidHTML(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body>
<a href="ubuntu-22.04.3-live-server-arm64.iso">ubuntu-22.04.3-live-server-arm64.iso</a>
<a href="ubuntu-22.04.2-live-server-arm64.iso">ubuntu-22.04.2-live-server-arm64.iso</a>
<a href="ubuntu-22.04.1-live-server-arm64.iso">ubuntu-22.04.1-live-server-arm64.iso</a>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
	}))
	defer server.Close()

	url, err := ScrapeLatestISO(context.Background(), server.URL, `ubuntu-[\d.]+-live-server-arm64\.iso`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == "" {
		t.Fatal("expected non-empty URL")
	}
	// Sorted alphabetically, last should be 22.04.3.
	if !containsSubstring(url, "22.04.3") {
		t.Errorf("expected latest version URL, got %q", url)
	}
}

func TestScrapeLatestISO_NoMatches(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body>
<a href="some-other-file.txt">some-other-file.txt</a>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer server.Close()

	url, err := ScrapeLatestISO(context.Background(), server.URL, `ubuntu-[\d.]+-live-server-arm64\.iso`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "" {
		t.Errorf("expected empty URL for no matches, got %q", url)
	}
}

func TestScrapeLatestISO_EmptyPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "")
	}))
	defer server.Close()

	url, err := ScrapeLatestISO(context.Background(), server.URL, `.*\.iso`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "" {
		t.Errorf("expected empty URL for empty page, got %q", url)
	}
}

func TestScrapeLatestISO_InvalidPattern(t *testing.T) {
	_, err := ScrapeLatestISO(context.Background(), "http://example.com", `[invalid(`)
	if err == nil {
		t.Fatal("expected error for invalid regex pattern")
	}
}

func TestScrapeLatestISO_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := ScrapeLatestISO(context.Background(), server.URL, `.*\.iso`)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestScrapeLatestISO_URLConstruction(t *testing.T) {
	html := `<html><body>
<a href="debian-12.4.0-arm64-netinst.iso">debian-12.4.0-arm64-netinst.iso</a>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer server.Close()

	url, err := ScrapeLatestISO(context.Background(), server.URL+"/isos/", `debian-[\d.]+-arm64-netinst\.iso`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == "" {
		t.Fatal("expected non-empty URL")
	}
	// Should be a fully resolved URL containing the filename.
	if !containsSubstring(url, "debian-12.4.0-arm64-netinst.iso") {
		t.Errorf("expected URL to contain filename, got %q", url)
	}
}

func TestScrapeLatestISO_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ScrapeLatestISO(ctx, "http://example.com", `.*\.iso`)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestScrapeLatestISO_UserAgentHeader(t *testing.T) {
	var userAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header.Get("User-Agent")
		fmt.Fprint(w, `<html><body><a href="test.iso">test</a></body></html>`)
	}))
	defer server.Close()

	ScrapeLatestISO(context.Background(), server.URL, `test\.iso`)

	if userAgent != "MiBeeHive/1.0" {
		t.Errorf("expected User-Agent 'MiBeeHive/1.0', got %q", userAgent)
	}
}

// containsSubstring is a helper to check if s contains substr.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
