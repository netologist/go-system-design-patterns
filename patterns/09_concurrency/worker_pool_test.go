package concurrency_test

import (
	"context"
	"sync"
	"testing"
	"time"

	concurrency "system-design-patterns/patterns/09_concurrency"
)

func TestBoundedWorkerPool_Processing(t *testing.T) {
	ctx := context.Background()
	pool := concurrency.NewBoundedWorkerPool[int, int](ctx, 3, 50)

	totalTasks := 20
	for i := range totalTasks {
		task := concurrency.Task[int, int]{
			Input: i,
			Fn: func(c context.Context, input int) (int, error) {
				time.Sleep(5 * time.Millisecond)
				return input * 2, nil
			},
		}
		pool.Submit(task)
	}

	var results []int
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for res := range pool.Results() {
			if res.Err != nil {
				t.Errorf("unexpected task error: %v", res.Err)
			}
			results = append(results, res.Output)
		}
	}()

	pool.Shutdown()
	wg.Wait()

	if len(results) != totalTasks {
		t.Fatalf("expected %d results, got: %d", totalTasks, len(results))
	}
}
