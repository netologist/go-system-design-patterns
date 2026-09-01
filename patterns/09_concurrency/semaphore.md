# Weighted Counting Semaphore Pattern

## Overview & Definition

While a mutex (`sync.Mutex`) restricts access to a shared resource to a single goroutine at a time, a **Counting Semaphore** permits up to $N$ concurrent units of work. A **Weighted Semaphore** allows operations to acquire varying weights of capacity (e.g. acquiring 1 unit for a lightweight query vs 4 units for an expensive batch job).

The **Weighted Counting Semaphore** pattern provides:
1. `Acquire(ctx, n)`: Blocks until $n$ units of capacity are available or context is canceled.
2. `TryAcquire(n)`: Non-blocking capacity check that returns immediately with a boolean flag.
3. `Release(n)`: Restores $n$ units of capacity and unblocks waiting goroutines via FIFO channel notification.
4. `Current()`: Observability accessor for tracking active capacity utilization.

---

## Problem Statement

Without weighted concurrency controls:

* **Resource Monopolization by Heavy Jobs:** A single batch job requesting a 100MB database export can run concurrently alongside 20 other heavy jobs, overwhelming database buffer pools and memory.
* **Hung Goroutines without Context Cancellation:** If a goroutine is waiting for a concurrency slot on a saturated resource, failing to observe context timeouts (`ctx.Done()`) causes goroutine leaks when callers abort.
* **Lack of Try-Acquire for Fast Rejection:** In low-latency APIs, queuing behind saturated resources increases latency. APIs need the ability to test capacity immediately and return HTTP 429 / 503 if capacity is unavailable.

---

## Architectural Mechanism & Flow

```
                      Acquire(ctx, n)
                             |
                             v
                    [Acquire Lock mu]
                             |
                   +---------+---------+
                   |                   |
                   v                   v
     [Capacity - Current >= n?]   [Capacity Saturated]
                   |                   |
           [Yes: Fast Path]    [Create waiter chan]
                   |           [Append to s.waiters]
            s.current += n     [Release Lock mu]
            [Release Lock mu]          |
                   |                   v
             [Return nil]     +--------------------+
                              |      select {      |
                              | case <-ctx.Done(): |
                              |   Remove waiter    |
                              |   Return ctx.Err() |
                              | case <-waiter:     |
                              |   Return nil       |
                              | }                  |
                              +--------------------+
```

### Release & Unblocking Mechanism
When `Release(n)` is invoked:
1. `s.current -= n` reduces active capacity.
2. The semaphore iterates through `s.waiters`, popping waiters and closing their notification channels (`close(waiter)`) as long as available capacity permits, ensuring orderly FIFO unblocking.

---

## Production Best Practices & Pitfalls

### Best Practices
* **Always Pair Acquire with Defer Release:** Immediately defer `sem.Release(n)` after a successful acquire to prevent capacity leaks during panics or early error returns.
* **Weight by Resource Consumption:** Assign weights proportional to actual cost (e.g., lightweight API calls = 1, complex reporting queries = 5, file exports = 10).
* **Enforce Capacity Constraints:** Reject any acquire where $n > \text{capacity}$ immediately with `ErrInvalidWeight` to avoid indefinite deadlocks.

### Common Pitfalls
* **Releasing More Units than Acquired:** Calling `Release(n)` with a higher value than acquired corrupts the semaphore's internal counters.
* **Releasing After Context Cancellation:** If `Acquire(ctx, n)` fails due to `ctx.Err()`, the semaphore was never acquired. Calling `Release()` in this failure branch will corrupt capacity.

---

## Code Walkthrough & Usage

### 1. Implementation (`semaphore.go`)

```go
package concurrency

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrSemaphoreExhausted = errors.New("semaphore capacity exhausted")
	ErrInvalidWeight      = errors.New("weight must be positive and <= total capacity")
)

// Semaphore provides a weighted counting semaphore with context cancellation support.
type Semaphore struct {
	mu       sync.Mutex
	capacity int64
	current  int64
	waiters  []chan struct{}
}

// NewSemaphore creates a semaphore with given maximum capacity.
func NewSemaphore(capacity int64) *Semaphore {
	if capacity <= 0 {
		capacity = 1
	}
	return &Semaphore{
		capacity: capacity,
	}
}

// Acquire acquires n units of weight, blocking until available or ctx canceled.
func (s *Semaphore) Acquire(ctx context.Context, n int64) error {
	if n <= 0 || n > s.capacity {
		return ErrInvalidWeight
	}

	s.mu.Lock()
	if s.capacity-s.current >= n {
		s.current += n
		s.mu.Unlock()
		return nil
	}

	// Create notification channel for this waiter
	waiter := make(chan struct{})
	s.waiters = append(s.waiters, waiter)
	s.mu.Unlock()

	select {
	case <-ctx.Done():
		s.mu.Lock()
		// Remove waiter on cancellation
		for i, ch := range s.waiters {
			if ch == waiter {
				s.waiters = append(s.waiters[:i], s.waiters[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
		return ctx.Err()
	case <-waiter:
		// Capacity became available
		return nil
	}
}

// TryAcquire attempts to acquire n units without blocking.
func (s *Semaphore) TryAcquire(n int64) bool {
	if n <= 0 || n > s.capacity {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.capacity-s.current >= n {
		s.current += n
		return true
	}
	return false
}

// Release releases n units of weight back to the semaphore.
func (s *Semaphore) Release(n int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.current -= n
	if s.current < 0 {
		s.current = 0
	}

	// Unblock waiters if capacity allows
	for len(s.waiters) > 0 && s.capacity-s.current > 0 {
		waiter := s.waiters[0]
		s.waiters = s.waiters[1:]
		s.current++ // Grant 1 unit to unblocked waiter
		close(waiter)
	}
}

// Current returns currently utilized capacity.
func (s *Semaphore) Current() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}
```

### 2. Database Concurrency Throttling Example

```go
type QueryService struct {
    db  *sql.DB
    sem *Semaphore
}

func NewQueryService(db *sql.DB, maxConcurrentQueries int64) *QueryService {
    return &QueryService{
        db:  db,
        sem: NewSemaphore(maxConcurrentQueries),
    }
}

func (s *QueryService) RunHeavyReport(ctx context.Context) (*ReportData, error) {
    // Acquire 3 units for a heavy report
    if err := s.sem.Acquire(ctx, 3); err != nil {
        return nil, fmt.Errorf("could not acquire report semaphore: %w", err)
    }
    defer s.sem.Release(3)

    return s.executeComplexQuery(ctx)
}
```
