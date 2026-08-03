package handler

import (
	"net/http"

	"github.com/Mi-Bee-Studio/mibeehive/internal/cache"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// Snapshotter is implemented by both cache.Cache and cache.TTLCache. It exposes
// a point-in-time view of capacity and hit/miss/eviction counters without
// leaking any cache keys or values.
type Snapshotter interface {
	Snapshot() cache.CacheStats
}

// CacheMetricsHandler exposes point-in-time statistics for the L1 cache layer.
// It only reports aggregate counters — never tokens, paths, or file IDs.
type CacheMetricsHandler struct {
	instances map[string]Snapshotter
}

// NewCacheMetricsHandler creates a handler wired to the six global cache
// instances. The map keys are stable, non-sensitive identifiers.
func NewCacheMetricsHandler() *CacheMetricsHandler {
	return &CacheMetricsHandler{
		instances: map[string]Snapshotter{
			"token_cache":       cache.TokenCache,
			"path_cache":        cache.PathCache,
			"children_cache":    cache.ChildrenCache,
			"rule_cache":        cache.RuleCache,
			"negative_cache":    cache.NegativeCache,
			"share_token_cache": cache.ShareTokenCache,
		},
	}
}

// CacheMetrics handles GET /api/v1/admin/metrics/cache.
func (h *CacheMetricsHandler) CacheMetrics(w http.ResponseWriter, r *http.Request) {
	out := make(map[string]cache.CacheStats, len(h.instances))
	for name, inst := range h.instances {
		out[name] = inst.Snapshot()
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[map[string]cache.CacheStats]{
		Success: true,
		Data:    out,
	})
}