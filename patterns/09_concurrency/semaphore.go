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

// TryAcquire attempts to acquire n units without blocking. Returns true if acquired.
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

// Release releases n units of weight back to the semaphore and unblocks waiting goroutines.
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
