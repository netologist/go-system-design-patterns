package caching_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	caching "system-design-patterns/patterns/14_caching"
)

func TestStampedeProtectedCache_CollapsesConcurrentMisses(t *testing.T) {
	rawCache := caching.NewMemoryCache[string]()
	protected := caching.NewStampedeProtectedCache[string](rawCache)
	ctx := context.Background()

	var dbFetches atomic.Int32

	fetchFn := func(c context.Context) (string, error) {
		dbFetches.Add(1)
		time.Sleep(30 * time.Millisecond) // Slow DB query
		return "computed_heavy_data", nil
	}

	var wg sync.WaitGroup
	totalCallers := 20
	results := make([]string, totalCallers)

	for i := range totalCallers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			val, err := protected.GetOrCompute(ctx, "popular_key", 1*time.Hour, fetchFn)
			if err != nil {
				t.Errorf("call %d failed: %v", idx, err)
			}
			results[idx] = val
		}(i)
	}

	wg.Wait()

	if dbFetches.Load() != 1 {
		t.Fatalf("expected exactly 1 DB fetch despite 20 concurrent callers, got: %d", dbFetches.Load())
	}

	for i, res := range results {
		if res != "computed_heavy_data" {
			t.Errorf("at index %d: expected 'computed_heavy_data', got: %s", i, res)
		}
	}
}
