package caching_test

import (
	"context"
	"errors"
	"testing"
	"time"

	caching "system-design-patterns/patterns/14_caching"
)

func TestNegativeCache_CachesMisses(t *testing.T) {
	mem := caching.NewMemoryCache[caching.CacheEntry[string]]()
	negCache := caching.NewNegativeCache[string](mem, 10*time.Minute, 1*time.Minute)
	ctx := context.Background()

	dbLookups := 0
	fetcher := func(c context.Context) (string, bool, error) {
		dbLookups++
		return "", true, nil // Resource does not exist (notFound = true)
	}

	// 1st lookup -> Calls DB, records negative entry
	_, err := negCache.GetOrFetch(ctx, "missing_user_999", fetcher)
	if !errors.Is(err, caching.ErrNotFoundCached) {
		t.Fatalf("expected ErrNotFoundCached, got: %v", err)
	}
	if dbLookups != 1 {
		t.Errorf("expected 1 DB lookup, got: %d", dbLookups)
	}

	// 2nd lookup -> Returns negative entry directly from cache without calling DB!
	_, err = negCache.GetOrFetch(ctx, "missing_user_999", fetcher)
	if !errors.Is(err, caching.ErrNotFoundCached) {
		t.Fatalf("expected ErrNotFoundCached on 2nd fetch, got: %v", err)
	}
	if dbLookups != 1 {
		t.Errorf("expected 0 additional DB lookups on cached negative entry, got: %d", dbLookups)
	}
}
