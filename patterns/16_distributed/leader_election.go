package distributed

import (
	"sync"
	"time"
)

// LeaderElector implements lease-based leader election among distributed nodes.
type LeaderElector struct {
	mu           sync.RWMutex
	currentLeader string
	leaseExpiry  time.Time
}

func NewLeaderElector() *LeaderElector {
	return &LeaderElector{}
}

// TryAcquireOrRenew attempts to become the leader or renew the active leadership lease.
func (e *LeaderElector) TryAcquireOrRenew(candidateID string, leaseDuration time.Duration) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()

	// 1. If currently leader, renew lease
	if e.currentLeader == candidateID {
		e.leaseExpiry = now.Add(leaseDuration)
		return true
	}

	// 2. If existing lease has expired or no leader exists, claim leadership
	if e.currentLeader == "" || now.After(e.leaseExpiry) {
		e.currentLeader = candidateID
		e.leaseExpiry = now.Add(leaseDuration)
		return true
	}

	// 3. Active lease held by another candidate
	return false
}

// IsLeader checks if candidate is the currently active leader.
func (e *LeaderElector) IsLeader(candidateID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.currentLeader == candidateID && time.Now().Before(e.leaseExpiry)
}

// Resign voluntary step-down by leader.
func (e *LeaderElector) Resign(candidateID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.currentLeader == candidateID {
		e.currentLeader = ""
		e.leaseExpiry = time.Time{}
	}
}
