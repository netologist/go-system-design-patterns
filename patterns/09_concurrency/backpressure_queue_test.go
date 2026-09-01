package concurrency_test

import (
	"context"
	"errors"
	"testing"

	concurrency "system-design-patterns/patterns/09_concurrency"
)

func TestBoundedQueue_StrategyReject(t *testing.T) {
	q := concurrency.NewBoundedQueue[string](2, concurrency.StrategyReject)
	ctx := context.Background()

	_ = q.Push(ctx, "msg-1")
	_ = q.Push(ctx, "msg-2")

	err := q.Push(ctx, "msg-3")
	if !errors.Is(err, concurrency.ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull on saturated queue, got: %v", err)
	}
}

func TestBoundedQueue_StrategyDropOldest(t *testing.T) {
	q := concurrency.NewBoundedQueue[string](2, concurrency.StrategyDropOldest)
	ctx := context.Background()

	_ = q.Push(ctx, "msg-1")
	_ = q.Push(ctx, "msg-2")
	_ = q.Push(ctx, "msg-3") // Drops msg-1

	if q.DroppedCount() != 1 {
		t.Errorf("expected dropped count 1, got: %d", q.DroppedCount())
	}

	item1, _ := q.Pop(ctx)
	if item1 != "msg-2" {
		t.Errorf("expected msg-2 next, got: %s", item1)
	}

	item2, _ := q.Pop(ctx)
	if item2 != "msg-3" {
		t.Errorf("expected msg-3 next, got: %s", item2)
	}
}
