package lifecycle_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	lifecycle "system-design-patterns/patterns/01_lifecycle"
)

func TestGracefulShutdown_PriorityOrder(t *testing.T) {
	mgr := lifecycle.NewGracefulShutdownManager(2 * time.Second)
	var executionOrder []string
	var mu sync.Mutex

	record := func(name string) lifecycle.CleanupFunc {
		return func(ctx context.Context) error {
			mu.Lock()
			executionOrder = append(executionOrder, name)
			mu.Unlock()
			return nil
		}
	}

	// Register hooks with different priorities
	mgr.Register("HTTP Server Stop", 100, record("HTTP Server Stop"))
	mgr.Register("Worker Queue Drain", 80, record("Worker Queue Drain"))
	mgr.Register("Database Pool Close", 20, record("Database Pool Close"))
	mgr.Register("Logging Flush", 10, record("Logging Flush"))

	err := mgr.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("unexpected error during shutdown: %v", err)
	}

	expected := []string{"HTTP Server Stop", "Worker Queue Drain", "Database Pool Close", "Logging Flush"}
	if len(executionOrder) != len(expected) {
		t.Fatalf("expected %d hooks executed, got %d", len(expected), len(executionOrder))
	}
	for i, name := range expected {
		if executionOrder[i] != name {
			t.Errorf("at index %d: expected %s, got %s", i, name, executionOrder[i])
		}
	}
}

func TestGracefulShutdown_Timeout(t *testing.T) {
	mgr := lifecycle.NewGracefulShutdownManager(50 * time.Millisecond)

	mgr.Register("Hanging Task", 100, func(ctx context.Context) error {
		select {
		case <-time.After(200 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	err := mgr.Shutdown(context.Background())
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestGracefulShutdown_ErrorAggregation(t *testing.T) {
	mgr := lifecycle.NewGracefulShutdownManager(1 * time.Second)

	errDB := errors.New("db close error")
	errRedis := errors.New("redis disconnect error")

	mgr.Register("DB", 50, func(ctx context.Context) error { return errDB })
	mgr.Register("Redis", 40, func(ctx context.Context) error { return errRedis })

	err := mgr.Shutdown(context.Background())
	if err == nil {
		t.Fatal("expected aggregated error, got nil")
	}

	if !errors.Is(err, errDB) || !errors.Is(err, errRedis) {
		t.Errorf("expected combined error containing both errors, got: %v", err)
	}
}
