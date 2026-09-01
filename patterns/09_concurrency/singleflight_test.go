package concurrency_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	concurrency "system-design-patterns/patterns/09_concurrency"
)

func TestSingleflight_DeduplicatesConcurrentCalls(t *testing.T) {
	group := concurrency.NewSingleflightGroup[string]()

	var executionCount atomic.Int32

	expensiveDBQuery := func() (string, error) {
		executionCount.Add(1)
		time.Sleep(50 * time.Millisecond) // Simulate slow DB
		return "user_data_record", nil
	}

	var wg sync.WaitGroup
	totalCallers := 10
	results := make([]string, totalCallers)

	for i := range totalCallers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			val, err, _ := group.Do("user:123", expensiveDBQuery)
			if err != nil {
				t.Errorf("call %d failed: %v", idx, err)
			}
			results[idx] = val
		}(i)
	}

	wg.Wait()

	if executionCount.Load() != 1 {
		t.Fatalf("expected exactly 1 DB execution, got: %d", executionCount.Load())
	}

	for i, res := range results {
		if res != "user_data_record" {
			t.Errorf("at index %d: expected 'user_data_record', got: %s", i, res)
		}
	}
}
