package caching_test

import (
	"testing"

	caching "system-design-patterns/patterns/14_caching"
)

func TestLRUCache_EvictionOrder(t *testing.T) {
	cache := caching.NewLRUCache[string, int](2) // Capacity 2

	cache.Put("k1", 1)
	cache.Put("k2", 2)

	// Access k1 -> moves k1 to front, making k2 the oldest
	_, ok := cache.Get("k1")
	if !ok {
		t.Fatal("expected k1 present")
	}

	// Insert k3 -> should evict k2 (least recently used)
	cache.Put("k3", 3)

	if _, ok := cache.Get("k2"); ok {
		t.Errorf("expected k2 to be evicted")
	}

	if val, ok := cache.Get("k1"); !ok || val != 1 {
		t.Errorf("expected k1 to be retained, got val=%d (ok=%v)", val, ok)
	}

	if val, ok := cache.Get("k3"); !ok || val != 3 {
		t.Errorf("expected k3 to be retained, got val=%d (ok=%v)", val, ok)
	}
}
