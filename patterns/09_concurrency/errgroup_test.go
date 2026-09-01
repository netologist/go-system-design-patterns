package concurrency_test

import (
	"context"
	"errors"
	"testing"
	"time"

	concurrency "system-design-patterns/patterns/09_concurrency"
)

func TestErrGroup_Success(t *testing.T) {
	g, _ := concurrency.WithContext(context.Background())

	counter := 0
	for range 5 {
		g.Go(func() error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		t.Fatalf("expected all tasks to succeed, got: %v", err)
	}
	_ = counter
}

func TestErrGroup_FirstErrorCancelsSiblings(t *testing.T) {
	g, ctx := concurrency.WithContext(context.Background())

	errTarget := errors.New("database connection failure")

	// Failing task
	g.Go(func() error {
		time.Sleep(10 * time.Millisecond)
		return errTarget
	})

	// Sibling task observing context cancellation
	siblingCanceled := false
	g.Go(func() error {
		select {
		case <-time.After(200 * time.Millisecond):
			return nil
		case <-ctx.Done():
			siblingCanceled = true
			return ctx.Err()
		}
	})

	err := g.Wait()
	if !errors.Is(err, errTarget) {
		t.Errorf("expected first error %v, got: %v", errTarget, err)
	}

	if !siblingCanceled {
		t.Errorf("expected sibling goroutine to be canceled on first error")
	}
}
