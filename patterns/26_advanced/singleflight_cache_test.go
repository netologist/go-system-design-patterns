package advanced_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	advanced "system-design-patterns/patterns/26_advanced"
)

func TestSingleflightWithStaleCache_DeduplicationAndStaleFallback(t *testing.T) {
	cache := advanced.NewSingleflightWithStaleCache[string]()
	ctx := context.Background()

	// 1. Initial warm-up
	val, isStale, err := cache.Fetch(ctx, "k1", 10*time.Millisecond, func(c context.Context) (string, error) {
		return "initial_data", nil
	})
	if err != nil || isStale || val != "initial_data" {
		t.Fatalf("warmup failed: val=%s isStale=%v err=%v", val, isStale, err)
	}

	// Wait for 10ms cache TTL to expire
	time.Sleep(20 * time.Millisecond)

	// 2. DB is down -> Should return stale value!
	val, isStale, err = cache.Fetch(ctx, "k1", 10*time.Millisecond, func(c context.Context) (string, error) {
		return "", errors.New("database connection refused")
	})
	if err != nil {
		t.Fatalf("expected fallback to stale value, got error: %v", err)
	}
	if !isStale || val != "initial_data" {
		t.Errorf("expected stale value 'initial_data', got val=%s (isStale=%v)", val, isStale)
	}

	// 3. Concurrent requests during DB refresh
	var dbExecutions atomic.Int32
	var wg sync.WaitGroup

	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = cache.Fetch(ctx, "k2", 1*time.Hour, func(c context.Context) (string, error) {
				dbExecutions.Add(1)
				time.Sleep(10 * time.Millisecond)
				return "k2_val", nil
			})
		}()
	}

	wg.Wait()

	if dbExecutions.Load() != 1 {
		t.Errorf("expected exactly 1 DB execution for 10 concurrent requests, got: %d", dbExecutions.Load())
	}
}
