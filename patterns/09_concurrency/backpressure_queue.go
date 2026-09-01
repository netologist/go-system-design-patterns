package concurrency

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

var ErrQueueFull = errors.New("queue is full: backpressure applied")

// OverflowStrategy defines behavior when pushing to a full queue.
type OverflowStrategy int

const (
	StrategyReject     OverflowStrategy = iota // Return ErrQueueFull immediately
	StrategyDropOldest                         // Drop oldest item to make space
	StrategyBlock                              // Block until space is available or ctx canceled
)

// BoundedQueue manages an in-memory queue with explicit backpressure overflow strategies.
type BoundedQueue[T any] struct {
	mu           sync.Mutex
	items        []T
	capacity     int
	strategy     OverflowStrategy
	notEmpty     sync.Cond
	notFull      sync.Cond
	droppedCount atomic.Int64
	closed       bool
}

// NewBoundedQueue creates a bounded queue with chosen overflow strategy.
func NewBoundedQueue[T any](capacity int, strategy OverflowStrategy) *BoundedQueue[T] {
	if capacity <= 0 {
		capacity = 100
	}

	q := &BoundedQueue[T]{
		items:    make([]T, 0, capacity),
		capacity: capacity,
		strategy: strategy,
	}
	q.notEmpty.L = &q.mu
	q.notFull.L = &q.mu
	return q
}

// Push adds an item to the queue adhering to the overflow strategy.
func (q *BoundedQueue[T]) Push(ctx context.Context, item T) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return errors.New("queue is closed")
	}

	for len(q.items) >= q.capacity {
		switch q.strategy {
		case StrategyReject:
			return ErrQueueFull
		case StrategyDropOldest:
			// Discard oldest element
			q.items = q.items[1:]
			q.droppedCount.Add(1)
		case StrategyBlock:
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				q.notFull.Wait()
			}
		}
	}

	q.items = append(q.items, item)
	q.notEmpty.Signal()
	return nil
}

// Pop removes and returns the next item from the queue, blocking until available or closed.
func (q *BoundedQueue[T]) Pop(ctx context.Context) (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.items) == 0 {
		if q.closed {
			var zero T
			return zero, false
		}
		q.notEmpty.Wait()
	}

	item := q.items[0]
	q.items = q.items[1:]
	q.notFull.Signal()
	return item, true
}

// DroppedCount returns total dropped elements under backpressure.
func (q *BoundedQueue[T]) DroppedCount() int64 {
	return q.droppedCount.Load()
}

// Len returns current queue depth.
func (q *BoundedQueue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
