package concurrency_test

import (
	"context"
	"testing"
	"time"

	concurrency "system-design-patterns/patterns/09_concurrency"
)

func TestSemaphore_TryAcquireAndRelease(t *testing.T) {
	sem := concurrency.NewSemaphore(2)

	if !sem.TryAcquire(1) {
		t.Fatal("expected 1st acquire to succeed")
	}
	if !sem.TryAcquire(1) {
		t.Fatal("expected 2nd acquire to succeed")
	}
	if sem.TryAcquire(1) {
		t.Fatal("expected 3rd acquire to fail on saturated semaphore")
	}

	sem.Release(1)

	if !sem.TryAcquire(1) {
		t.Fatal("expected acquire to succeed after release")
	}
}

func TestSemaphore_ContextTimeout(t *testing.T) {
	sem := concurrency.NewSemaphore(1)
	_ = sem.Acquire(context.Background(), 1)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	err := sem.Acquire(ctx, 1)
	if err == nil {
		t.Fatal("expected context cancellation error on blocked acquire, got nil")
	}
}
