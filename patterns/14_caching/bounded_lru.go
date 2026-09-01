package caching

import (
	"container/list"
	"sync"
)

type lruEntry[K comparable, V any] struct {
	key   K
	value V
}

// LRUCache implements a thread-safe, bounded Least-Recently-Used cache.
type LRUCache[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	items    map[K]*list.Element
	evictList *list.List
}

func NewLRUCache[K comparable, V any](capacity int) *LRUCache[K, V] {
	if capacity <= 0 {
		capacity = 100
	}

	return &LRUCache[K, V]{
		capacity:  capacity,
		items:     make(map[K]*list.Element),
		evictList: list.New(),
	}
}

// Get retrieves an item, updating its position to most recently used.
func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.evictList.MoveToFront(elem)
		return elem.Value.(*lruEntry[K, V]).value, true
	}

	var zero V
	return zero, false
}

// Put inserts or updates an item, evicting the least recently used item if at capacity.
func (c *LRUCache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.evictList.MoveToFront(elem)
		elem.Value.(*lruEntry[K, V]).value = value
		return
	}

	// Evict oldest if full
	if c.evictList.Len() >= c.capacity {
		oldest := c.evictList.Back()
		if oldest != nil {
			c.evictList.Remove(oldest)
			kv := oldest.Value.(*lruEntry[K, V])
			delete(c.items, kv.key)
		}
	}

	// Insert new element at front
	entry := &lruEntry[K, V]{key: key, value: value}
	elem := c.evictList.PushFront(entry)
	c.items[key] = elem
}

// Len returns current item count.
func (c *LRUCache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}
