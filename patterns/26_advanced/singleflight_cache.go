package advanced

import (
	"context"
	"sync"
	"time"

	concurrency "system-design-patterns/patterns/09_concurrency"
	caching "system-design-patterns/patterns/14_caching"
)

// SingleflightWithStaleCache combines Singleflight deduplication with stale-while-revalidate cache fallback.
type SingleflightWithStaleCache[V any] struct {
	cache *caching.MemoryCache[V]
	group *concurrency.SingleflightGroup[V]
	mu    sync.RWMutex
	stale map[string]V
}

func NewSingleflightWithStaleCache[V any]() *SingleflightWithStaleCache[V] {
	return &SingleflightWithStaleCache[V]{
		cache: caching.NewMemoryCache[V](),
		group: concurrency.NewSingleflightGroup[V](),
		stale: make(map[string]V),
	}
}

// Fetch returns fresh data from cache, deduplicates DB miss fetches, or falls back to stale data if DB fails.
func (s *SingleflightWithStaleCache[V]) Fetch(
	ctx context.Context,
	key string,
	ttl time.Duration,
	dbQuery func(ctx context.Context) (V, error),
) (val V, isStale bool, err error) {
	// 1. Fresh cache hit
	if v, err := s.cache.Get(ctx, key); err == nil {
		return v, false, nil
	}

	// 2. Singleflight DB fetch
	v, err, _ := s.group.Do(key, func() (V, error) {
		res, dbErr := dbQuery(ctx)
		if dbErr != nil {
			var zero V
			return zero, dbErr
		}

		_ = s.cache.Set(ctx, key, res, ttl)
		s.mu.Lock()
		s.stale[key] = res
		s.mu.Unlock()

		return res, nil
	})

	if err == nil {
		return v, false, nil
	}

	// 3. Fallback to stale value on DB failure
	s.mu.RLock()
	staleVal, hasStale := s.stale[key]
	s.mu.RUnlock()

	if hasStale {
		return staleVal, true, nil
	}

	var zero V
	return zero, false, err
}
