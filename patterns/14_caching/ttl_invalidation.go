package caching

import (
	"strings"
	"sync"
	"time"
)

type taggedItem struct {
	value     any
	tags      []string
	expiresAt time.Time
}

// InvalidationManager manages cache entries with TTL, prefix-matching, and tag-based invalidation.
type InvalidationManager struct {
	mu    sync.RWMutex
	items map[string]*taggedItem
}

func NewInvalidationManager() *InvalidationManager {
	return &InvalidationManager{
		items: make(map[string]*taggedItem),
	}
}

// Set stores an item with TTL and optional invalidation tags.
func (m *InvalidationManager) Set(key string, val any, ttl time.Duration, tags ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items[key] = &taggedItem{
		value:     val,
		tags:      tags,
		expiresAt: time.Now().Add(ttl),
	}
}

// Get retrieves an item if not expired.
func (m *InvalidationManager) Get(key string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, ok := m.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		return nil, false
	}
	return item.value, true
}

// InvalidatePrefix drops all keys starting with prefix (e.g. "user:123:").
func (m *InvalidationManager) InvalidatePrefix(prefix string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for k := range m.items {
		if strings.HasPrefix(k, prefix) {
			delete(m.items, k)
			count++
		}
	}
	return count
}

// InvalidateTag drops all entries tagged with given tag.
func (m *InvalidationManager) InvalidateTag(tag string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for k, item := range m.items {
		for _, t := range item.tags {
			if t == tag {
				delete(m.items, k)
				count++
				break
			}
		}
	}
	return count
}
