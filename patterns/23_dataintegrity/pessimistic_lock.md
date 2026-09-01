# Pessimistic Locking Pattern (Keyed Mutex Lock)

## Overview & Definition
The **Pessimistic Locking Pattern** (also known as Keyed Mutex Locking or In-Memory `SELECT FOR UPDATE`) enforces exclusive, serialized access to specific domain resources identified by a unique key (such as an Account ID, User ID, or Inventory SKU) while allowing concurrent operations on distinct keys to proceed in parallel.

In high-concurrency applications, locking the entire service or database table with a single global mutex severely degrades throughput. Conversely, omitting concurrency controls allows race conditions (e.g., two concurrent requests deducting money from the same account simultaneously).

The `KeyedMutexLock` dynamically provisions fine-grained mutexes per key, maintains reference counting (`refCount`) so memory is automatically reclaimed when keys are idle, and returns an idiomatic Go cleanup closure (`defer unlock()`).

---

## Problem Statement
High-concurrency updates to shared state result in race conditions, lost updates, or global throughput bottlenecks.

### Failure Scenarios Without This Pattern
- **Lost Update Anomaly:** Request A reads Account balance ($100), Request B reads Account balance ($100). Both deduct $50 and write back $50. The customer spent $100 but only $50 was deducted.
- **Global Lock Contention Bottlenecks:** Using a single global application mutex (`sync.Mutex`) forces operations on User 1 to wait for operations on User 10,000, reducing multi-core CPU scaling to single-threaded performance.
- **Memory Leaks from Unbounded Key Maps:** Naive keyed lock implementations allocate a mutex per key in a map without deleting them, leading to unbounded heap growth over millions of transactions.
- **Deadlocks from Inconsistent Lock Ordering:** Acquiring multiple locks across resources in non-deterministic orders can cause classic deadlock cycles.

---

## Architectural Mechanism & Flow
The `KeyedMutexLock` uses a two-tier locking architecture: a fast global lock protects the reference-counted key registry, while an individual mutex serializes execution for the target key:

```
[ Goroutine 1: Lock("account-42") ]     [ Goroutine 2: Lock("account-99") ]
                 │                                       │
                 ▼                                       ▼
 ┌───────────────────────────────────────────────────────────────┐
 │          Global Registry Mutex (Fast Lookup & Increment)       │
 └───────────────────────────────┬───────────────────────────────┘
                                 │
         ┌───────────────────────┴───────────────────────┐
         ▼                                               ▼
 ┌───────────────────────────────┐               ┌───────────────────────────────┐
 │ Key: "account-42"             │               │ Key: "account-99"             │
 │ - Mutex: Locked (G1 holds)    │               │ - Mutex: Locked (G2 holds)    │
 │ - RefCount: 1                 │               │ - RefCount: 1                 │
 └───────────────┬───────────────┘               └───────────────┬───────────────┘
                 │                                               │
  [ G1 Executes Account 42 ]                      [ G2 Executes Account 99 ]
  (Concurrent with G2!)                           (Concurrent with G1!)
                 │                                               │
                 ▼                                               ▼
         [ defer unlock() ]                              [ defer unlock() ]
                 │                                               │
 ┌───────────────────────────────┐               ┌───────────────────────────────┐
 │ 1. entry.mu.Unlock()          │               │ 1. entry.mu.Unlock()          │
 │ 2. globalMu.Lock()            │               │ 2. globalMu.Lock()            │
 │ 3. refCount-- (if 0 -> delete)│               │ 3. refCount-- (if 0 -> delete)│
 └───────────────────────────────┘               └───────────────────────────────┘
```

---

## Production Best Practices & Pitfalls

### Best Practices
- **Return Idiomatic Unlock Closures:** Return a cleanup closure `unlock := lock.Lock(key); defer unlock()` to guarantee locks are freed even in panic scenarios.
- **Automatic Memory Reclaim via Reference Counting:** Track `refCount` on each lock entry. When `refCount == 0`, remove the key from the map so the garbage collector reclaims the memory.
- **Deterministic Lock Ordering for Multi-Key Operations:** When acquiring multiple keys simultaneously (e.g., transferring funds between Account A and Account B), always sort the keys lexicographically before acquiring locks to prevent deadlocks:
  ```go
  if keyA > keyB { keyA, keyB = keyB, keyA }
  ```
- **Integrate with Database-Level Locking:** For distributed multi-instance services, back this pattern with Redis Redlock or database-level row locks (`SELECT ... FOR UPDATE`).

### Pitfalls to Avoid
- **Holding Global Mutex During Slow Operations:** Holding `globalMu` while executing business logic or network I/O serializes the entire server. `globalMu` must only be held for microsecond map pointer operations.
- **Leaking Locks on Panic:** Forgetting `defer unlock()` causes subsequent operations on that key to hang forever.
- **Unbounded Wait Times:** In production, consider adding context-aware lock acquisition (`TryLock` or channel-based timeouts) so requests fail fast if a key is held too long.

---

## Code Walkthrough & Usage

### Core Implementation
The pattern implementation in `pessimistic_lock.go` manages reference-counted key locks:

```go
package dataintegrity

import (
	"sync"
)

type keyRefMutex struct {
	mu       sync.Mutex
	refCount int
}

// KeyedMutexLock coordinates fine-grained pessimistic locking per resource ID (simulating SELECT FOR UPDATE).
type KeyedMutexLock struct {
	globalMu sync.Mutex
	locks    map[string]*keyRefMutex
}

func NewKeyedMutexLock() *KeyedMutexLock {
	return &KeyedMutexLock{
		locks: make(map[string]*keyRefMutex),
	}
}

// Lock acquires exclusive lock for the specific key.
func (l *KeyedMutexLock) Lock(key string) func() {
	l.globalMu.Lock()
	entry, ok := l.locks[key]
	if !ok {
		entry = &keyRefMutex{}
		l.locks[key] = entry
	}
	entry.refCount++
	l.globalMu.Unlock()

	entry.mu.Lock()

	// Return unlock closure
	return func() {
		entry.mu.Unlock()

		l.globalMu.Lock()
		defer l.globalMu.Unlock()
		entry.refCount--
		if entry.refCount == 0 {
			delete(l.locks, key)
		}
	}
}
```

### Production High-Throughput Wallet Service Example

```go
type WalletService struct {
    lockMgr *dataintegrity.KeyedMutexLock
    db      *sql.DB
}

func (s *WalletService) DeductBalance(accountID string, amount int64) error {
    // Acquire fine-grained lock for this specific account
    unlock := s.lockMgr.Lock(accountID)
    defer unlock()

    // Execute critical section exclusively for this account
    // Other accounts continue executing concurrently
    return s.executeDeduction(accountID, amount)
}
```
