package rulesrc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestDefaultGetter_RateLimitTagged verifies the default HTTP getter tags
// quota responses (429, exhausted X-RateLimit-Remaining, or a body mentioning
// "rate limit") so the crawler's retry policy maps them to the rate_limited
// crawl status (issue #60).
func TestDefaultGetter_RateLimitTagged(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		remain  string
		wantTag bool
	}{
		{"429", http.StatusTooManyRequests, `{"message":"too many requests"}`, "", true},
		{"403 quota body", http.StatusForbidden, `{"message":"API rate limit exceeded for 1.2.3.4."}`, "", true},
		{"403 remaining zero", http.StatusForbidden, `{"message":"nope"}`, "0", true},
		{"404 not rate limit", http.StatusNotFound, `{"message":"not found"}`, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if c.remain != "" {
					w.Header().Set("X-RateLimit-Remaining", c.remain)
				}
				w.WriteHeader(c.status)
				w.Write([]byte(c.body))
			}))
			defer srv.Close()

			g := defaultGetter{hc: srv.Client()}
			_, _, err := g.Get(context.Background(), Request{URL: srv.URL})
			if err == nil {
				t.Fatal("expected error for >=400 response")
			}
			if strings.Contains(err.Error(), "rate limit") != c.wantTag {
				t.Errorf("error = %q, rate-limit tag = %v, want %v", err, !c.wantTag, c.wantTag)
			}
			// The body excerpt must be included for diagnosability.
			if !strings.Contains(err.Error(), c.body) {
				t.Errorf("error should include body excerpt %q, got: %v", c.body, err)
			}
		})
	}
}

// TestNewFetcher_TimeoutBounded verifies the fetcher's HTTP client has a
// finite timeout (it previously used http.DefaultClient, which has none).
func TestNewFetcher_TimeoutBounded(t *testing.T) {
	f := NewFetcher()
	g, ok := f.client.(defaultGetter)
	if !ok {
		t.Fatalf("expected defaultGetter, got %T", f.client)
	}
	if g.hc.Timeout <= 0 {
		t.Errorf("client timeout = %v, want > 0", g.hc.Timeout)
	}
	if g.hc.Timeout > 60*time.Second {
		t.Errorf("client timeout = %v, want a bounded value", g.hc.Timeout)
	}
}
