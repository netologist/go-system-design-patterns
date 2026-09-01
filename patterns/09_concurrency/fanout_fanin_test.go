package concurrency_test

import (
	"context"
	"testing"

	concurrency "system-design-patterns/patterns/09_concurrency"
)

func TestFanOutFanIn_Pipeline(t *testing.T) {
	ctx := context.Background()

	// Producer
	inCh := make(chan int, 20)
	for i := 1; i <= 20; i++ {
		inCh <- i
	}
	close(inCh)

	// Fan-Out across 4 workers
	workerFn := func(c context.Context, n int) int {
		return n * 10
	}
	workerChannels := concurrency.FanOut(ctx, inCh, 4, workerFn)

	// Fan-In
	mergedCh := concurrency.FanIn(ctx, workerChannels...)

	var collected []int
	for res := range mergedCh {
		collected = append(collected, res)
	}

	if len(collected) != 20 {
		t.Fatalf("expected 20 items merged, got: %d", len(collected))
	}
}
