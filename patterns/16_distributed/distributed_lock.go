package distributed

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrLockHeldByOther = errors.New("distributed lock is already acquired by another instance")
	ErrLockLost        = errors.New("distributed lock lost or lease expired")
)

// LockLease holds lock token and monotonically increasing fencing token.
type LockLease struct {
	Resource     string
	Owner        string
	FencingToken int64
	ExpiresAt    time.Time
}

// DistributedLockManager coordinates exclusive distributed locks with TTL leases and fencing tokens.
type DistributedLockManager struct {
	mu           sync.Mutex
	locks        map[string]*LockLease
	fencingToken atomic.Int64
}

func NewDistributedLockManager() *DistributedLockManager {
	return &DistributedLockManager{
		locks: make(map[string]*LockLease),
	}
}

// Acquire attempts to acquire a lock for a given resource with a lease TTL.
func (m *DistributedLockManager) Acquire(ctx context.Context, resource, owner string, ttl time.Duration) (*LockLease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	existing, ok := m.locks[resource]
	if ok && now.Before(existing.ExpiresAt) && existing.Owner != owner {
		return nil, ErrLockHeldByOther
	}

	fencing := m.fencingToken.Add(1)
	lease := &LockLease{
		Resource:     resource,
		Owner:        owner,
		FencingToken: fencing,
		ExpiresAt:    now.Add(ttl),
	}
	m.locks[resource] = lease

	return lease, nil
}

// Release releases the lock if held by the given owner.
func (m *DistributedLockManager) Release(ctx context.Context, resource, owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.locks[resource]
	if !ok || existing.Owner != owner {
		return nil // Already released or owned by another
	}

	delete(m.locks, resource)
	return nil
}
