package distributed_test

import (
	"context"
	"errors"
	"testing"
	"time"

	distributed "system-design-patterns/patterns/16_distributed"
)

func TestDistributedLock_MutualExclusionAndFencing(t *testing.T) {
	mgr := distributed.NewDistributedLockManager()
	ctx := context.Background()

	// Instance A acquires lock
	leaseA, err := mgr.Acquire(ctx, "payment_sync", "instance-A", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("instance A failed to acquire lock: %v", err)
	}

	if leaseA.FencingToken != 1 {
		t.Errorf("expected fencing token 1, got: %d", leaseA.FencingToken)
	}

	// Instance B attempts to acquire while A holds lock -> Rejected
	_, err = mgr.Acquire(ctx, "payment_sync", "instance-B", 100*time.Millisecond)
	if !errors.Is(err, distributed.ErrLockHeldByOther) {
		t.Fatalf("expected ErrLockHeldByOther for instance B, got: %v", err)
	}

	// Instance A releases
	_ = mgr.Release(ctx, "payment_sync", "instance-A")

	// Instance B can now acquire lock -> Receives fencing token 2
	leaseB, err := mgr.Acquire(ctx, "payment_sync", "instance-B", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("instance B failed to acquire after release: %v", err)
	}

	if leaseB.FencingToken != 2 {
		t.Errorf("expected incremented fencing token 2, got: %d", leaseB.FencingToken)
	}
}
