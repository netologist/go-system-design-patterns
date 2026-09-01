package persistence_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	persistence "system-design-patterns/patterns/04_persistence"
)

func TestPoolLimiter_AcquireAndRelease(t *testing.T) {
	pool := persistence.NewConnectionPoolLimiter(2)
	ctx := context.Background()

	conn1, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire conn1 failed: %v", err)
	}

	conn2, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire conn2 failed: %v", err)
	}

	stats := pool.Stats()
	if stats.ActiveConns != 2 {
		t.Errorf("expected 2 active conns, got: %d", stats.ActiveConns)
	}

	// 3rd acquire with short timeout must fail because pool capacity is 2
	ctxTimeout, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	_, err = pool.Acquire(ctxTimeout)
	if !errors.Is(err, persistence.ErrPoolExhausted) {
		t.Fatalf("expected ErrPoolExhausted on saturated pool, got: %v", err)
	}

	// Release conn1
	conn1.Close()

	// Now 3rd acquire should succeed
	conn3, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("expected acquire to succeed after release, got: %v", err)
	}

	conn2.Close()
	conn3.Close()
}

func TestPoolLimiter_ConcurrentStress(t *testing.T) {
	pool := persistence.NewConnectionPoolLimiter(5)
	ctx := context.Background()

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reqCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			defer cancel()

			conn, err := pool.Acquire(reqCtx)
			if err == nil {
				time.Sleep(10 * time.Millisecond)
				conn.Close()
			}
		}()
	}

	wg.Wait()

	stats := pool.Stats()
	if stats.ActiveConns != 0 {
		t.Errorf("expected 0 active conns after all released, got: %d", stats.ActiveConns)
	}
}
