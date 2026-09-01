package caching

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrCacheMiss = errors.New("cache miss")

// CacheClient represents generic key-value cache operations.
type CacheClient[V any] interface {
	Get(ctx context.Context, key string) (V, error)
	Set(ctx context.Context, key string, value V, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// MemoryCache implements in-memory thread-safe CacheClient.
type MemoryCache[V any] struct {
	mu    sync.RWMutex
	items map[string]cacheItem[V]
}

type cacheItem[V any] struct {
	val       V
	expiresAt time.Time
}

func NewMemoryCache[V any]() *MemoryCache[V] {
	return &MemoryCache[V]{
		items: make(map[string]cacheItem[V]),
	}
}

func (c *MemoryCache[V]) Get(ctx context.Context, key string) (V, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		var zero V
		return zero, ErrCacheMiss
	}
	return item.val, nil
}

func (c *MemoryCache[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = cacheItem[V]{
		val:       value,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

func (c *MemoryCache[V]) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	return nil
}

// CacheAsideService coordinates reads and writes adhering to the Cache-Aside pattern.
type CacheAsideService[V any] struct {
	cache CacheClient[V]
	dbGet func(ctx context.Context, key string) (V, error)
	dbSet func(ctx context.Context, key string, val V) error
	ttl   time.Duration
}

func NewCacheAsideService[V any](
	cache CacheClient[V],
	ttl time.Duration,
	dbGet func(ctx context.Context, key string) (V, error),
	dbSet func(ctx context.Context, key string, val V) error,
) *CacheAsideService[V] {
	return &CacheAsideService[V]{
		cache: cache,
		ttl:   ttl,
		dbGet: dbGet,
		dbSet: dbSet,
	}
}

// Get implements Cache-Aside:
// 1. Check cache
// 2. On miss, read from DB
// 3. Write back to cache
func (s *CacheAsideService[V]) Get(ctx context.Context, key string) (V, error) {
	if val, err := s.cache.Get(ctx, key); err == nil {
		return val, nil
	}

	// Cache miss -> Fetch from DB
	val, err := s.dbGet(ctx, key)
	if err != nil {
		var zero V
		return zero, fmt.Errorf("db fetch error: %w", err)
	}

	// Populate cache asynchronously or synchronously
	_ = s.cache.Set(ctx, key, val, s.ttl)

	return val, nil
}

// Set updates the DB and invalidates the cache to prevent stale data.
func (s *CacheAsideService[V]) Set(ctx context.Context, key string, val V) error {
	if err := s.dbSet(ctx, key, val); err != nil {
		return fmt.Errorf("db update failed: %w", err)
	}

	// Invalidate cache
	return s.cache.Delete(ctx, key)
}
