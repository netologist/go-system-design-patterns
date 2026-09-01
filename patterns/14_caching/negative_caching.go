package caching

import (
	"context"
	"errors"
	"time"
)

var ErrNotFoundCached = errors.New("resource not found (cached)")

type CacheEntry[T any] struct {
	Value      T
	IsNotFound bool
}

// NegativeCache caches both positive hits (with normal TTL) and negative misses (with short TTL).
type NegativeCache[T any] struct {
	cache       CacheClient[CacheEntry[T]]
	positiveTTL time.Duration
	negativeTTL time.Duration
}

func NewNegativeCache[T any](cache CacheClient[CacheEntry[T]], positiveTTL, negativeTTL time.Duration) *NegativeCache[T] {
	if positiveTTL <= 0 {
		positiveTTL = 10 * time.Minute
	}
	if negativeTTL <= 0 {
		negativeTTL = 30 * time.Second
	}

	return &NegativeCache[T]{
		cache:       cache,
		positiveTTL: positiveTTL,
		negativeTTL: negativeTTL,
	}
}

// GetOrFetch fetches from cache, or on miss calls fetchFn. If fetchFn returns not found, caches a negative entry.
func (c *NegativeCache[T]) GetOrFetch(
	ctx context.Context,
	key string,
	fetchFn func(ctx context.Context) (val T, notFound bool, err error),
) (T, error) {
	entry, err := c.cache.Get(ctx, key)
	if err == nil {
		if entry.IsNotFound {
			var zero T
			return zero, ErrNotFoundCached
		}
		return entry.Value, nil
	}

	// Fetch from source
	val, notFound, err := fetchFn(ctx)
	if err != nil {
		var zero T
		return zero, err
	}

	if notFound {
		// Store negative entry with short TTL
		_ = c.cache.Set(ctx, key, CacheEntry[T]{IsNotFound: true}, c.negativeTTL)
		var zero T
		return zero, ErrNotFoundCached
	}

	// Store positive entry with standard TTL
	_ = c.cache.Set(ctx, key, CacheEntry[T]{Value: val, IsNotFound: false}, c.positiveTTL)
	return val, nil
}
