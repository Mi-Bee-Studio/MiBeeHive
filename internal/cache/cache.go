// Package cache provides the L1 in-memory cache layer for the supply layer.
//
// It wraps hashicorp/golang-lru/v2 (a thread-safe, fixed-size LRU) with
// hit/miss/eviction counters and exposes five bounded cache instances plus an
// event-driven invalidator that keeps them coherent with the underlying store.
//
// Memory budget: all caches are bounded and together stay well under 64MB.
//   - TokenCache:    4096 entries (string key + int64 value)
//   - PathCache:     8192 entries (string key + int64 value)
//   - ChildrenCache: 2048 entries (string key + []int64 value)
//   - RuleCache:      256 entries (int64 key + []string value)
//   - NegativeCache: 4096 entries (string key + TTLItem value, 5s TTL)
package cache

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/eventbus"
	"github.com/hashicorp/golang-lru/v2"
)

// Cache wraps a fixed-size LRU cache with hit/miss/eviction counters.
type Cache[K comparable, V any] struct {
	cache     *lru.Cache[K, V]
	hits      atomic.Int64
	misses    atomic.Int64
	evictions atomic.Int64
}

// NewCache creates a sized LRU cache. Panics if size <= 0.
func NewCache[K comparable, V any](size int) *Cache[K, V] {
	c, err := lru.New[K, V](size)
	if err != nil {
		panic(err)
	}
	return &Cache[K, V]{cache: c}
}

// Get retrieves value. Returns (value, ok). Updates hit/miss counters.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	v, ok := c.cache.Get(key)
	if ok {
		c.hits.Add(1)
	} else {
		c.misses.Add(1)
	}
	return v, ok
}

// Put stores value. Increments the eviction counter if an entry was evicted.
func (c *Cache[K, V]) Put(key K, value V) {
	if evicted := c.cache.Add(key, value); evicted {
		c.evictions.Add(1)
	}
}

// Delete removes a key (event invalidation — never in-place update).
func (c *Cache[K, V]) Delete(key K) {
	c.cache.Remove(key)
}

// Purge clears all entries.
func (c *Cache[K, V]) Purge() {
	c.cache.Purge()
}

// Len returns current item count.
func (c *Cache[K, V]) Len() int {
	return c.cache.Len()
}

// Stats returns (hits, misses, evictions).
func (c *Cache[K, V]) Stats() (hits, misses, evictions int64) {
	return c.hits.Load(), c.misses.Load(), c.evictions.Load()
}

// Global cache instances. Sizes chosen to keep total memory well under 64MB.
var (
	// TokenCache maps public_token → file_id.
	TokenCache = NewCache[string, int64](4096)
	// PathCache maps full_path → node_id.
	PathCache = NewCache[string, int64](8192)
	// ChildrenCache maps parent_id → child node IDs.
	ChildrenCache = NewCache[string, []int64](2048)
	// RuleCache maps rule_node_id → file IDs (first page).
	RuleCache = NewCache[int64, []string](256)
	// NegativeCache maps token/path → "not found" (5s TTL).
	NegativeCache = NewTTLCache(4096, 5*time.Second)
	// ShareTokenCache maps share_token → file_id.
	ShareTokenCache = NewCache[string, int64](4096)
	)

// TTLItem is the value stored in a TTLCache; it carries the expiry timestamp.
type TTLItem struct {
	expiresAt time.Time
}

// TTLCache is a time-to-live wrapper over Cache. Entries auto-expire on Get.
type TTLCache struct {
	inner *Cache[string, TTLItem]
	ttl   time.Duration
}

// NewTTLCache creates a TTL cache with the given size and TTL.
func NewTTLCache(size int, ttl time.Duration) *TTLCache {
	return &TTLCache{
		inner: NewCache[string, TTLItem](size),
		ttl:   ttl,
	}
}

// Get returns the item if present and not expired; expired entries are deleted.
func (c *TTLCache) Get(key string) (TTLItem, bool) {
	item, ok := c.inner.Get(key)
	if !ok {
		return TTLItem{}, false
	}
	if time.Now().After(item.expiresAt) {
		c.inner.Delete(key)
		return TTLItem{}, false
	}
	return item, true
}

// Put stores a fresh entry with the configured TTL.
func (c *TTLCache) Put(key string) {
	c.inner.Put(key, TTLItem{expiresAt: time.Now().Add(c.ttl)})
}

// Delete removes a key.
func (c *TTLCache) Delete(key string) {
	c.inner.Delete(key)
}

// Purge clears all entries.
func (c *TTLCache) Purge() {
	c.inner.Purge()
}

// EventInvalidator subscribes to the event bus and deletes relevant cache
// entries when files/nodes change. It runs until ctx is cancelled or the bus
// is closed.
func EventInvalidator(ctx context.Context, bus *eventbus.Bus) {
	ch := bus.Subscribe(eventbus.TagFilePublished)
	ch2 := bus.Subscribe(eventbus.TagFileRemoved)
	ch3 := bus.Subscribe(eventbus.TagFileMetadataChanged)
	ch4 := bus.Subscribe(eventbus.TagNodeTreeChanged)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-ch:
				if !ok {
					return
				}
				handleFileEvent(e)
			case e, ok := <-ch2:
				if !ok {
					return
				}
				handleFileEvent(e)
			case e, ok := <-ch3:
				if !ok {
					return
				}
				handleFileEvent(e)
			case _, ok := <-ch4:
				if !ok {
					return
				}
				// NodeTreeChanged → clear path and children caches.
				PathCache.Purge()
				ChildrenCache.Purge()
				slog.Info("cache: path/children caches purged (NodeTreeChanged)")
			}
		}
	}()
}

// handleFileEvent purges the negative cache on any file lifecycle event. The
// token is not derivable from the event alone, so the negative cache is the
// only entry we can safely invalidate here; TokenCache invalidation happens at
// the service layer where the caller knows the token.
func handleFileEvent(e eventbus.Event) {
	switch e.(type) {
	case eventbus.FilePublished, eventbus.FileRemoved, eventbus.FileMetadataChanged:
		NegativeCache.Purge()
		slog.Debug("cache: negative cache purged (file event)")
	}
}