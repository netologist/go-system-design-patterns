# Bounded Queue with Backpressure Overflow Strategies

## Overview & Definition

When producers generate data faster than consumers can process it, queues buffer the difference. If a queue is unbounded, it will eventually consume all available heap memory and crash the application.

The **Bounded Queue with Backpressure Overflow Strategies** pattern provides a generic in-memory queue (`BoundedQueue[T]`) with a fixed capacity and explicit overflow policies:
* **`StrategyReject`:** Rejects new items immediately with `ErrQueueFull` when capacity is reached (fail-fast backpressure).
* **`StrategyDropOldest`:** Drops the oldest unconsumed item from the head of the queue to make room for the fresh item (ideal for real-time telemetry, IoT feeds, and live metrics).
* **`StrategyBlock`:** Blocks the producer goroutine using `sync.Cond` until consumers drain space or the context is canceled.
* Atomic counters track `DroppedCount()` for operational observability.

---

## Problem Statement

Unbounded or poorly managed queueing causes severe production degradation:

* **Out-Of-Memory (OOM) via Memory Queuing:** An unbounded channel (`make(chan Item)`) or infinite slice subjected to a slow consumer accumulates millions of records, triggering fatal OOM termination.
* **Stale Data Processing:** In real-time tracking, processing 10-minute-old sensor pings while dropping fresh telemetry degrades system accuracy.
* **Producer Blockages without Context Awareness:** A simple blocking channel write (`ch <- item`) cannot observe `context.Context` cancellation without an extra `select` branch, causing producer leaks when HTTP requests abort.

---

## Architectural Mechanism & Flow

```
                                Push(ctx, item)
                                       |
                                       v
                              [Acquire Mutex Lock]
                                       |
                                       v
                           [len(items) >= capacity?]
                                  /         \
                              [Yes]         [No]
                               /              \
         +--------------------+                v
         | Evaluate Strategy                   |
         v                                     |
    +--------------------------------------+   |
    | StrategyReject:                      |   |
    |   Return ErrQueueFull                |   |
    |                                      |   |
    | StrategyDropOldest:                  |   |
    |   items = items[1:]                  |   |
    |   droppedCount.Add(1)                |   |
    |                                      |   |
    | StrategyBlock:                       |   |
    |   notFull.Wait() until space drained |   |
    +--------------------------------------+   |
                        |                      |
                        +-----------+----------+
                                    |
                                    v
                         [items = append(items, item)]
                         [notEmpty.Signal()]
                         [Release Mutex Lock]
                                    |
                                    v
                               [Return nil]
```

---

## Production Best Practices & Pitfalls

### Best Practices
* **Choose Strategy Based on Domain Guarantees:**
  * **Transactional / Financial Data:** Use `StrategyReject` (fail fast to caller) or `StrategyBlock` (with strict timeouts). Never drop financial records silently.
  * **Observability / Telemetry / Metrics:** Use `StrategyDropOldest` so current real-time state is preserved during surges.
* **Alert on Dropped Counters:** Continuously export `q.DroppedCount()` to Prometheus/Datadog to detect consumer bottlenecks before system saturation.
* **Pair with `sync.Cond` for Thread Coordination:** Using `sync.Cond` (`notEmpty` and `notFull`) eliminates busy-waiting and CPU spin-locks.

### Common Pitfalls
* **Forgetting to Signal `notEmpty` or `notFull`:** Omitting `notEmpty.Signal()` on Push or `notFull.Signal()` on Pop will leave waiting goroutines permanently deadlocked.
* **Blocking Indefinitely on Deadlock:** When using `StrategyBlock`, always enforce a timeout or deadline on the producer's `context.Context`.

---

## Code Walkthrough & Usage

### 1. Implementation (`backpressure_queue.go`)

```go
package concurrency

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

var ErrQueueFull = errors.New("queue is full: backpressure applied")

type OverflowStrategy int

const (
	StrategyReject     OverflowStrategy = iota // Return ErrQueueFull immediately
	StrategyDropOldest                         // Drop oldest item to make space
	StrategyBlock                              // Block until space is available or ctx canceled
)

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

func (q *BoundedQueue[T]) DroppedCount() int64 {
	return q.droppedCount.Load()
}

func (q *BoundedQueue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
```

### 2. Telemetry Ingestion Example

```go
func StartTelemetryCollector(ctx context.Context) {
    // Drop oldest metrics if processing falls behind 1,000 items
    queue := NewBoundedQueue[MetricPoint](1000, StrategyDropOldest)

    // Ingestion goroutine
    go func() {
        for metric := range incomingMetricsStream {
            _ = queue.Push(ctx, metric)
        }
    }()

    // Consumer goroutine
    go func() {
        for {
            metric, ok := queue.Pop(ctx)
            if !ok {
                return
            }
            writeToPrometheus(metric)
        }
    }()
}
```
