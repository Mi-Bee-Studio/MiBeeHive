package cache

import (
	"context"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/eventbus"
)

func TestCacheHitMiss(t *testing.T) {
	c := NewCache[string, int](4)
	c.Put("a", 1)

	tests := []struct {
		name string
		key  string
		want int
		ok   bool
	}{
		{"hit", "a", 1, true},
		{"miss", "b", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := c.Get(tt.key)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("Get(%q) = (%v, %v), want (%v, %v)", tt.key, got, ok, tt.want, tt.ok)
			}
		})
	}

	hits, misses, evictions := c.Stats()
	if hits != 1 {
		t.Errorf("hits = %d, want 1", hits)
	}
	if misses != 1 {
		t.Errorf("misses = %d, want 1", misses)
	}
	if evictions != 0 {
		t.Errorf("evictions = %d, want 0", evictions)
	}
}

func TestCacheEviction(t *testing.T) {
	c := NewCache[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3) // evicts "a" (LRU)

	if _, ok := c.Get("a"); ok {
		t.Error("expected 'a' to be evicted")
	}
	if _, ok := c.Get("b"); !ok {
		t.Error("expected 'b' to be present")
	}
	if _, ok := c.Get("c"); !ok {
		t.Error("expected 'c' to be present")
	}
	if c.Len() != 2 {
		t.Errorf("Len = %d, want 2", c.Len())
	}
	_, _, evictions := c.Stats()
	if evictions != 1 {
		t.Errorf("evictions = %d, want 1", evictions)
	}
}

func TestCacheDelete(t *testing.T) {
	c := NewCache[string, int](4)
	c.Put("a", 1)
	c.Delete("a")
	if _, ok := c.Get("a"); ok {
		t.Error("expected 'a' to be deleted")
	}
	if c.Len() != 0 {
		t.Errorf("Len = %d, want 0", c.Len())
	}
}

func TestCachePurge(t *testing.T) {
	c := NewCache[string, int](4)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	c.Purge()
	if c.Len() != 0 {
		t.Errorf("Len = %d, want 0 after purge", c.Len())
	}
	if _, ok := c.Get("a"); ok {
		t.Error("expected 'a' to be gone after purge")
	}
}

func TestCacheNewPanicsOnZero(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for size <= 0")
		}
	}()
	NewCache[string, int](0)
}

func TestNegativeCacheTTL(t *testing.T) {
	c := NewTTLCache(4, 50*time.Millisecond)
	c.Put("token")

	if _, ok := c.Get("token"); !ok {
		t.Error("expected hit within TTL")
	}

	time.Sleep(80 * time.Millisecond)
	if _, ok := c.Get("token"); ok {
		t.Error("expected miss after TTL expiry")
	}
	if c.inner.Len() != 0 {
		t.Errorf("Len = %d, want 0 after expiry", c.inner.Len())
	}
}

func TestNegativeCacheDelete(t *testing.T) {
	c := NewTTLCache(4, time.Minute)
	c.Put("token")
	c.Delete("token")
	if _, ok := c.Get("token"); ok {
		t.Error("expected miss after delete")
	}
}

func TestEventInvalidation(t *testing.T) {
	// Seed the negative cache, then fire each file event and verify it purges.
	events := []eventbus.Event{
		eventbus.FilePublished{FileID: 1},
		eventbus.FileRemoved{FileID: 2},
		eventbus.FileMetadataChanged{FileID: 3},
	}
	for _, e := range events {
		NegativeCache.Put("token")
		handleFileEvent(e)
		if _, ok := NegativeCache.Get("token"); ok {
			t.Errorf("negative cache not purged after %T", e)
		}
	}
}

func TestEventInvalidatorNodeTree(t *testing.T) {
	bus := eventbus.NewBus(4)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	EventInvalidator(ctx, bus)

	PathCache.Put("/a/b", 1)
	ChildrenCache.Put("1", []int64{2, 3})

	bus.Publish(ctx, eventbus.NodeTreeChanged{ViewID: 1})

	// Wait for the invalidator goroutine to process the event.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if PathCache.Len() == 0 && ChildrenCache.Len() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if PathCache.Len() != 0 {
		t.Errorf("PathCache not purged, Len = %d", PathCache.Len())
	}
	if ChildrenCache.Len() != 0 {
		t.Errorf("ChildrenCache not purged, Len = %d", ChildrenCache.Len())
	}
}

func TestEventInvalidatorFileEvent(t *testing.T) {
	bus := eventbus.NewBus(2)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	EventInvalidator(ctx, bus)

	NegativeCache.Put("token")
	bus.Publish(ctx, eventbus.FilePublished{FileID: 1})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := NegativeCache.Get("token"); !ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, ok := NegativeCache.Get("token"); ok {
		t.Error("negative cache not purged after FilePublished")
	}
}