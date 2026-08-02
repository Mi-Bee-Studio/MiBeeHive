package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/cache"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

func TestCacheMetrics_Success(t *testing.T) {
	h := NewCacheMetricsHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminCacheMetrics, h.CacheMetrics)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminCacheMetrics, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[map[string]cache.CacheStats]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}

	// All six cache instances must be present with the expected capacities.
	expected := map[string]int{
		"token_cache":       4096,
		"path_cache":        8192,
		"children_cache":    2048,
		"rule_cache":        256,
		"negative_cache":    4096,
		"share_token_cache": 4096,
	}
	if len(resp.Data) != len(expected) {
		t.Fatalf("expected %d cache entries, got %d", len(expected), len(resp.Data))
	}
	for name, wantSize := range expected {
		stats, ok := resp.Data[name]
		if !ok {
			t.Fatalf("missing cache entry %q", name)
		}
		if stats.Size != wantSize {
			t.Fatalf("cache %q: expected size=%d, got %d", name, wantSize, stats.Size)
		}
		if stats.Hits < 0 || stats.Misses < 0 || stats.Evictions < 0 {
			t.Fatalf("cache %q: counters must be non-negative, got %+v", name, stats)
		}
	}
}

func TestCacheMetrics_AuthRequired(t *testing.T) {
	h := NewCacheMetricsHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminCacheMetrics, h.CacheMetrics)
	handler := wrapWithAuth(mux)

	// Unauthenticated request.
	req := httptest.NewRequest(http.MethodGet, model.RouteAdminCacheMetrics, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}