package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSharedHTTPClientSingleton(t *testing.T) {
	c1 := SharedHTTPClient()
	c2 := SharedHTTPClient()
	if c1 != c2 {
		t.Error("SharedHTTPClient should return the same instance")
	}
}

func TestSharedHTTPClientTransport(t *testing.T) {
	c := SharedHTTPClient()
	transport, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if transport.MaxIdleConns != 10 {
		t.Errorf("MaxIdleConns = %d, want 10", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 5 {
		t.Errorf("MaxIdleConnsPerHost = %d, want 5", transport.MaxIdleConnsPerHost)
	}
}

func TestSharedHTTPClientTimeout(t *testing.T) {
	c := SharedHTTPClient()
	if c.Timeout != 30*1e9 { // 30 seconds in nanoseconds
		t.Errorf("Timeout = %v, want 30s", c.Timeout)
	}
}

func TestLimitedReadAllUnderLimit(t *testing.T) {
	body := strings.NewReader("hello")
	data, err := LimitedReadAll(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want %q", string(data), "hello")
	}
}

func TestLimitedReadAllOverLimit(t *testing.T) {
	// Create a reader that's larger than the limit.
	oversized := io.NopCloser(newOverLimitReader(MaxResponseBodySize + 1))
	_, err := LimitedReadAll(oversized)
	if err == nil {
		t.Fatal("expected error for oversized body")
	}
	if !strings.Contains(err.Error(), "exceeds 100MB limit") {
		t.Errorf("error = %q, want mention of 100MB limit", err.Error())
	}
}

func TestLimitedReadBodyOverLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", MaxResponseBodySize+1024))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	_, err = LimitedReadBody(resp)
	if err == nil {
		t.Fatal("expected error for oversized body via Content-Length")
	}
	if !strings.Contains(err.Error(), "Content-Length") {
		t.Errorf("error = %q, want mention of Content-Length", err.Error())
	}
}

func TestLargeResponseRejected(t *testing.T) {
	// Serve a response whose body exceeds the 100MB limit.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write more than MaxResponseBodySize bytes.
		w.WriteHeader(http.StatusOK)
		chunk := make([]byte, 1024*1024) // 1MB chunks
		for i := int64(0); i <= MaxResponseBodySize/int64(len(chunk)); i++ {
			w.Write(chunk)
		}
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	_, err = LimitedReadBody(resp)
	if err == nil {
		t.Fatal("expected error for response exceeding 100MB limit")
	}
	if !strings.Contains(err.Error(), "exceeds 100MB limit") {
		t.Errorf("error = %q, want mention of 100MB limit", err.Error())
	}
}

func TestLimitedReadBodyUnderLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	data, err := LimitedReadBody(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "ok" {
		t.Errorf("got %q, want %q", string(data), "ok")
	}
}

// overLimitReader is an io.Reader that returns bytes up to a given total,
// used to simulate an oversized response body.
type overLimitReader struct {
	remaining int64
}

func newOverLimitReader(total int64) *overLimitReader {
	return &overLimitReader{remaining: total}
}

func (r *overLimitReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > r.remaining {
		n = r.remaining
	}
	for i := int64(0); i < n; i++ {
		p[i] = 'x'
	}
	r.remaining -= n
	return int(n), nil
}

// Test that crawlers actually use SharedHTTPClient by checking they
// get the singleton instance.
func TestCrawlersUseSharedClient(t *testing.T) {
	shared := SharedHTTPClient()

	cases := []struct {
		name   string
		client *http.Client
	}{
		{"github", NewGitHubCrawler("", nil).httpClient},
		{"golang", NewGoCrawler(nil).httpClient},
		{"hashicorp", NewHashiCorpCrawler("", nil).httpClient},
		{"grafana", NewGrafanaCrawler(nil).httpClient},
		{"npm", NewNPMCrawler("", nil).httpClient},
		{"pypi", NewPyPICrawler("", nil).httpClient},
		{"crates", NewCratesCrawler("", nil).httpClient},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.client != shared {
				t.Errorf("%s crawler: httpClient is not the shared instance", tc.name)
			}
		})
	}
}

// TestCrawlerFetchWithSharedClient verifies a crawler can fetch using the shared client.
func TestCrawlerFetchWithSharedClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewGitHubCrawler("", nil)
	c.baseURL = srv.URL
	// The httpClient should already be the shared one from the constructor.
	ctx := context.Background()
	assets, err := c.FetchReleases(ctx, "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assets) != 0 {
		t.Errorf("expected 0 assets, got %d", len(assets))
	}
}
