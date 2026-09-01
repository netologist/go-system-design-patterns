# Bounded Worker Pool Pattern

## Overview & Definition

In Go, spawning unbounded goroutines (`go doWork()`) for every incoming task is an anti-pattern under high load. Each goroutine consumes stack memory (starting at ~2KB) and system resources; 100,000 unbounded goroutines can instantly allocate hundreds of megabytes of heap and overwhelm downstream databases, external APIs, and the Go runtime scheduler.

The **Bounded Worker Pool** pattern provides a generic, type-safe pool (`BoundedWorkerPool[T, R]`) that:
1. Spawns a fixed number of long-lived worker goroutines (`workerCount`).
2. Buffers pending work in a bounded channel (`taskQueue`).
3. Streams typed results and errors back via a dedicated output channel (`results`).
4. Implements safe graceful shutdown (`Shutdown()`) that closes the task queue, waits for in-flight tasks to complete via `sync.WaitGroup`, and closes the results channel without deadlocks or goroutine leaks.

---

## Problem Statement

Unbounded concurrency in production systems leads to severe failure scenarios:

* **Out-Of-Memory (OOM) Crashes:** When a burst of 50,000 tasks arrives, spawning 50,000 goroutines causes high GC pressure, CPU thrashing from context switching, and memory exhaustion.
* **Downstream Connection Flooding:** If every task opens a database transaction or outbound HTTP connection, downstream pools are instantly exhausted, causing database lockups and connection timeouts.
* **Ungraceful Shutdowns & Lost Work:** Terminating a service during task execution without tracking worker completion leads to half-written files, corrupted database records, and leaked resources.

---

## Architectural Mechanism & Flow

```
                               Submit(Task[T, R])
                                       |
                                       v
                    +------------------------------------+
                    |       Bounded taskQueue chan       |
                    |           (Capacity: N)            |
                    +------------------------------------+
                                       |
                   +-------------------+-------------------+
                   |                   |                   |
                   v                   v                   v
          +-----------------+ +-----------------+ +-----------------+
          |    Worker 1     | |    Worker 2     | |    Worker N     |
          |  goroutine loop | |  goroutine loop | |  goroutine loop |
          +-----------------+ +-----------------+ +-----------------+
                   |                   |                   |
                   +-------------------+-------------------+
                                       |
                                       v
                    +------------------------------------+
                    |       Bounded results chan         |
                    |         TaskResult[R]              |
                    +------------------------------------+
                                       |
                                       v
                             Consumer Results() loop
```

### Shutdown Lifecycle
1. `pool.Shutdown()` closes `taskQueue`, signaling workers to finish all queued tasks and exit.
2. `p.wg.Wait()` blocks until all worker goroutines complete their loops.
3. `close(p.results)` closes the results stream, signaling to consumers that all output is processed.
4. `p.cancel()` cancels the pool's root context.

---

## Production Best Practices & Pitfalls

### Best Practices
* **Tune Worker Count to Task Type:**
  * **CPU-bound tasks:** Set `workerCount` to `runtime.GOMAXPROCS(0)` or `runtime.NumCPU()`.
  * **I/O-bound tasks:** Set `workerCount` higher (e.g. 20–100) based on downstream database and API connection limits.
* **Propagate Context into Tasks:** Always pass `p.ctx` (or task-specific derived contexts) into `task.Fn` to allow graceful cancellation of individual tasks during shutdown.
* **Drain Results Asynchronously:** Ensure the consumer goroutine reads from `pool.Results()` concurrently with task submission to prevent output channel backpressure from stalling workers.

### Common Pitfalls
* **Closing the Results Channel Early:** Closing `results` before `p.wg.Wait()` finishes will cause panicked writes on a closed channel if any worker attempts to emit a result.
* **Submitting Tasks After Shutdown:** Attempting to write to `taskQueue` after `close(taskQueue)` will trigger a runtime panic.

---

## Code Walkthrough & Usage

### 1. Implementation (`worker_pool.go`)

```go
package concurrency

import (
	"context"
	"sync"
)

// Task represents an executable unit of work.
type Task[T any, R any] struct {
	Input T
	Fn    func(ctx context.Context, input T) (R, error)
}

// TaskResult holds the outcome of a Task.
type TaskResult[R any] struct {
	Output R
	Err    error
}

// BoundedWorkerPool processes tasks using a fixed number of worker goroutines.
type BoundedWorkerPool[T any, R any] struct {
	workerCount int
	taskQueue   chan Task[T, R]
	results     chan TaskResult[R]
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewBoundedWorkerPool initializes the pool and starts workers.
func NewBoundedWorkerPool[T any, R any](parentCtx context.Context, workerCount int, queueCapacity int) *BoundedWorkerPool[T, R] {
	if workerCount <= 0 {
		workerCount = 4
	}
	if queueCapacity <= 0 {
		queueCapacity = 100
	}

	ctx, cancel := context.WithCancel(parentCtx)

	pool := &BoundedWorkerPool[T, R]{
		workerCount: workerCount,
		taskQueue:   make(chan Task[T, R], queueCapacity),
		results:     make(chan TaskResult[R], queueCapacity),
		ctx:         ctx,
		cancel:      cancel,
	}

	pool.start()
	return pool
}

func (p *BoundedWorkerPool[T, R]) start() {
	for range p.workerCount {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for {
				select {
				case <-p.ctx.Done():
					return
				case task, ok := <-p.taskQueue:
					if !ok {
						return
					}
					out, err := task.Fn(p.ctx, task.Input)
					select {
					case <-p.ctx.Done():
						return
					case p.results <- TaskResult[R]{Output: out, Err: err}:
					}
				}
			}
		}()
	}
}

// Submit enqueues a task. Blocks if queue is full or returns false if context canceled.
func (p *BoundedWorkerPool[T, R]) Submit(task Task[T, R]) bool {
	select {
	case <-p.ctx.Done():
		return false
	case p.taskQueue <- task:
		return true
	}
}

// Results returns the read-only output channel.
func (p *BoundedWorkerPool[T, R]) Results() <-chan TaskResult[R] {
	return p.results
}

// Shutdown closes the task queue, waits for workers to finish, and closes results.
func (p *BoundedWorkerPool[T, R]) Shutdown() {
	close(p.taskQueue)
	p.wg.Wait()
	close(p.results)
	p.cancel()
}
```

### 2. Processing Pipeline Example

```go
func ProcessImageBatch(imageURLs []string) ([]ImageMetadata, error) {
    ctx := context.Background()
    pool := NewBoundedWorkerPool[string, ImageMetadata](ctx, 8, 100)

    // Drain results concurrently
    var results []ImageMetadata
    var errs []error
    var drainWg sync.WaitGroup
    drainWg.Add(1)

    go func() {
        defer drainWg.Done()
        for res := range pool.Results() {
            if res.Err != nil {
                errs = append(errs, res.Err)
                continue
            }
            results = append(results, res.Output)
        }
    }()

    // Submit batch
    for _, url := range imageURLs {
        pool.Submit(Task[string, ImageMetadata]{
            Input: url,
            Fn: func(c context.Context, u string) (ImageMetadata, error) {
                return downloadAndResize(c, u)
            },
        })
    }

    pool.Shutdown()
    drainWg.Wait()

    return results, errors.Join(errs...)
}
```
