# Channel Ownership, Mutex Discipline, and Atomic Synchronization

## Overview & Definition

Writing concurrent Go programs requires choosing the right synchronization primitive for the job: channels, mutexes, or atomic instructions.

The **Channel Ownership, Mutex Discipline, and Atomic Synchronization** pattern establishes clear engineering rules for concurrent safety:
1. **Channel Ownership:** The goroutine that *initializes* and *writes* to a channel is its sole owner and is exclusively responsible for closing it (`defer close(out)`), returning a receive-only channel (`<-chan T`) to consumers.
2. **Mutex Discipline:** Using `sync.RWMutex` to guard complex shared state while minimizing lock hold times and avoiding function calls or I/O while holding locks.
3. **Lock-Free Atomic Counters:** Using `sync/atomic` (`atomic.Int64`) for simple scalar counters to achieve maximum throughput with zero lock contention.

---

## Problem Statement

Violating channel ownership and synchronization discipline leads to Go's most notorious runtime panics and race conditions:

* **Panic on Sending to or Closing Closed Channels:** If multiple goroutines attempt to close the same channel, or if a writer sends to a channel closed by a consumer, Go immediately raises an unrecoverable runtime panic (`panic: send on closed channel` / `panic: close of closed channel`).
* **Deadlocks & Lock Contention:** Holding a mutex across network calls, file system I/O, or channel operations serializes execution and creates cyclic deadlock risks.
* **Overhead of Mutexes for Simple Counters:** Using a heavy `sync.Mutex` for simple increment operations generates unnecessary thread context switching and cache invalidation overhead compared to CPU-level atomic instructions (`LOCK XADD`).

---

## Architectural Mechanism & Flow

```
+-----------------------------------------------------------------------------------+
| 1. CHANNEL OWNERSHIP PATTERN                                                      |
|                                                                                   |
|  [Producer Goroutine (Owner)]                  [Consumer Goroutine(s)]             |
|   1. Creates: out := make(chan int)             1. Receives: <-chan int           |
|   2. Writes:  out <- val                        2. Drains:   for val := range ch  |
|   3. Closes:  defer close(out)                  3. CANNOT CLOSE / CANNOT WRITE    |
+-----------------------------------------------------------------------------------+

+-----------------------------------------------------------------------------------+
| 2. MUTEX DISCIPLINE PATTERN                                                       |
|                                                                                   |
|  c.mu.Lock()                                                                      |
|  c.value++               <-- Only touch memory; NO I/O, NO RPCs, NO Channel Sends |
|  c.mu.Unlock()                                                                    |
+-----------------------------------------------------------------------------------+

+-----------------------------------------------------------------------------------+
| 3. LOCK-FREE ATOMIC PATTERN                                                       |
|                                                                                   |
|  a.val.Add(1)            <-- Single CPU atomic instruction (LOCK XADD / CAS)      |
|  a.val.Load()            <-- Zero lock overhead, lock-free thread safety          |
+-----------------------------------------------------------------------------------+
```

---

## Production Best Practices & Pitfalls

### Best Practices
* **Expose Only Receive-Only Channels:** Function signatures should return `<-chan T` rather than bidirectional `chan T` to prevent consumers from accidentally writing or closing the channel.
* **Keep Mutex Critical Sections Small:** Never perform HTTP requests, database transactions, or sleep operations inside a `mu.Lock()` block. Acquire the lock, copy/mutate the data, and unlock immediately.
* **Prefer `sync/atomic` for Metrics & Flags:** Use `atomic.Int64`, `atomic.Uint64`, and `atomic.Bool` for rate counters, request metrics, and shutdown flags.

### Common Pitfalls
* **Closing Channels from the Consumer Side:** Channels should never be closed by receivers. If a receiver wants to stop receiving, it should signal cancellation via a `context.Context` or a separate `done` channel.
* **Copying Structs Containing Mutexes:** Mutexes must never be copied by value (`pass by value` or struct assignment) because copying a mutex copies its internal state, breaking synchronization. Pass structs containing mutexes exclusively by pointer (`*SafeCounter`).

---

## Code Walkthrough & Usage

### 1. Implementation (`channel_ownership.go`)

```go
package concurrency

import (
	"sync"
	"sync/atomic"
)

// GenerateSequence demonstrates Channel Ownership: The producer creates, writes, and closes the channel,
// returning a read-only channel to consumers.
func GenerateSequence(count int) <-chan int {
	out := make(chan int, count)
	go func() {
		defer close(out)
		for i := 1; i <= count; i++ {
			out <- i
		}
	}()
	return out
}

// SafeCounter demonstrates Mutex Discipline (keeping lock scope minimal).
type SafeCounter struct {
	mu    sync.RWMutex
	value int64
}

func (c *SafeCounter) Inc() {
	c.mu.Lock()
	c.value++
	c.mu.Unlock()
}

func (c *SafeCounter) Value() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value
}

// AtomicCounter demonstrates lock-free concurrent synchronization.
type AtomicCounter struct {
	val atomic.Int64
}

func (a *AtomicCounter) Inc() {
	a.val.Add(1)
}

func (a *AtomicCounter) Value() int64 {
	return a.val.Load()
}
```

### 2. Usage Comparison

```go
func RunBenchmark() {
    // 1. Channel ownership consumption
    seqCh := GenerateSequence(10)
    for n := range seqCh {
        // Consumer safely reads until producer closes channel
        _ = n
    }

    // 2. High-throughput atomic increment
    var activeRequests AtomicCounter
    activeRequests.Inc()
    defer activeRequests.val.Add(-1)
}
```
