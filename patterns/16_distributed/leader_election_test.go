package distributed_test

import (
	"testing"
	"time"

	distributed "system-design-patterns/patterns/16_distributed"
)

func TestLeaderElector_ElectionAndFailover(t *testing.T) {
	elector := distributed.NewLeaderElector()

	// 1. Node 1 becomes leader
	if !elector.TryAcquireOrRenew("node-1", 50*time.Millisecond) {
		t.Fatal("expected node-1 to be elected leader")
	}

	if !elector.IsLeader("node-1") {
		t.Error("expected node-1 to be recognized as leader")
	}

	// 2. Node 2 fails to usurp while node-1's lease is valid
	if elector.TryAcquireOrRenew("node-2", 50*time.Millisecond) {
		t.Fatal("node-2 should not be able to become leader while node-1 lease is active")
	}

	// 3. Node 1 resigns
	elector.Resign("node-1")

	// 4. Node 2 can now claim leadership
	if !elector.TryAcquireOrRenew("node-2", 50*time.Millisecond) {
		t.Fatal("expected node-2 to become leader after node-1 resigned")
	}

	if !elector.IsLeader("node-2") {
		t.Error("expected node-2 to be recognized as leader")
	}
}
