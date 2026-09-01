package database_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	database "system-design-patterns/patterns/13_database"
)

func TestBatchLoader_ConsolidatesNQueriesIntoOne(t *testing.T) {
	var batchExecutions atomic.Int32

	batchFetchUsers := func(ctx context.Context, keys []string) (map[string]string, error) {
		batchExecutions.Add(1)
		res := make(map[string]string)
		for _, k := range keys {
			res[k] = fmt.Sprintf("User:%s", k)
		}
		return res, nil
	}

	loader := database.NewBatchLoader[string, string](10*time.Millisecond, 50, batchFetchUsers)
	ctx := context.Background()

	var wg sync.WaitGroup
	totalCalls := 15
	results := make([]string, totalCalls)

	for i := range totalCalls {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			val, err := loader.Load(ctx, fmt.Sprintf("usr-%d", idx))
			if err != nil {
				t.Errorf("call %d failed: %v", idx, err)
			}
			results[idx] = val
		}(i)
	}

	wg.Wait()

	if batchExecutions.Load() != 1 {
		t.Fatalf("expected exactly 1 batched DB query instead of %d queries, got: %d",
			totalCalls, batchExecutions.Load())
	}

	for i, res := range results {
		expected := fmt.Sprintf("User:usr-%d", i)
		if res != expected {
			t.Errorf("at index %d: expected %s, got: %s", i, expected, res)
		}
	}
}
