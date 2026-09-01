package caching

import (
	"context"
	"time"

	concurrency "system-design-patterns/patterns/09_concurrency"
)

// StampedeProtectedCache combines cache and Singleflight to prevent thundering herd on cache misses.
type StampedeProtectedCache[V any] struct {
	cache CacheClient[V]
	group *concurrency.SingleflightGroup[V]
}

func NewStampedeProtectedCache[V any](cache CacheClient[V]) *StampedeProtectedCache[V] {
	return &StampedeProtectedCache[V]{
		cache: cache,
		group: concurrency.NewSingleflightGroup[V](),
	}
}

// GetOrCompute checks the cache, and on miss, coordinates concurrent requests to execute fetchFn exactly once.
func (s *StampedeProtectedCache[V]) GetOrCompute(
	ctx context.Context,
	key string,
	ttl time.Duration,
	fetchFn func(ctx context.Context) (V, error),
) (V, error) {
	// 1. Fast path: cache hit
	if val, err := s.cache.Get(ctx, key); err == nil {
		return val, nil
	}

	// 2. Cache miss -> Singleflight collapsed execution
	val, err, _ := s.group.Do(key, func() (V, error) {
		// Double check cache inside singleflight lock
		if v, err := s.cache.Get(ctx, key); err == nil {
			return v, nil
		}

		res, err := fetchFn(ctx)
		if err != nil {
			var zero V
			return zero, err
		}

		_ = s.cache.Set(ctx, key, res, ttl)
		return res, nil
	})

	return val, err
}
