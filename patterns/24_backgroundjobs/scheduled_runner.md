# Scheduled Task Runner Pattern (Periodic Background Ticker Loop)

## Overview & Definition
The **Scheduled Task Runner Pattern** manages long-running, recurring background jobs (such as metric aggregation, cache invalidation, database cleanup, subscription billing, and health checks) in Go backend applications.

It coordinates time-based execution intervals using `time.NewTicker`, enforces structured lifecycle management through `context.Context` cancellation, and prevents orphaned or abruptly terminated goroutines during server shutdown using `sync.WaitGroup`.

---

## Problem Statement
Running periodic background operations using naive `time.Sleep` loops without context propagation or synchronization creates resource leaks and data corruption during deployment lifecycles.

### Failure Scenarios Without This Pattern
- **Orphaned / Leaked Goroutines:** Using `time.Tick` (which can never be stopped or garbage collected) or infinite `for { time.Sleep(...) }` loops leaks goroutines when tasks are stopped or reloaded.
- **Corrupted Background Transactions on Shutdown:** When a service receives `SIGTERM`, uncoordinated background tasks running database migrations or billing batches are killed mid-execution.
- **Overlapping Executions on Slow Tasks:** If a task scheduled for every 10 seconds takes 15 seconds to execute, a naive timer spawner will spawn overlapping concurrent instances, leading to database lock contention and duplicate processing.
- **Panic Propagation:** An unrecovered panic in an unmanaged background goroutine terminates the entire Go process.

---

## Architectural Mechanism & Flow
The `ScheduledTaskRunner` encapsulates the ticker loop within a managed goroutine lifecycle, listening for context cancellations and waiting for in-flight tasks to complete during `Stop()`:

```
[ Application Startup: runner.Start() ]
                  │
                  ▼
   ┌───────────────────────────────┐
   │ wg.Add(1)                     │
   │ go runner.loop()              │
   └──────────────┬────────────────┘
                  │
                  ▼
   ┌───────────────────────────────┐
   │ ticker := time.NewTicker(int) │
   └──────────────┬────────────────┘
                  │
         ┌────────┴────────┐
         │ select { ... }  │
         └────────┬────────┘
                  │
     ┌────────────┴────────────┐
     ▼                         ▼
[ case <-ticker.C ]    [ case <-ctx.Done() ]
     │                         │
     ▼                         ▼
[ Execute task(ctx) ]   [ Exit Loop ]
     │                         │
     └─────────┬───────────────┘
               │
               ▼
[ Application Shutdown: runner.Stop() ]
               │
               ▼
   ┌───────────────────────────────┐
   │ cancel() -> ctx.Done() closes │
   │ wg.Wait() (Blocks for finish) │
   └───────────────────────────────┘
```

---

## Production Best Practices & Pitfalls

### Best Practices
- **Always Stop Tickers:** Use `ticker := time.NewTicker(...)` and ensure `defer ticker.Stop()` is called to release the runtime timer. Never use `time.Tick()` in long-lived or dynamic services.
- **Honor Context Cancellation Inside the Task:** Pass `ctx` directly into `ScheduledTask(ctx)`. Long-running tasks should inspect `ctx.Done()` between batch chunks to abort early during server shutdowns.
- **Wait for Completion in Shutdown Handlers:** Use `sync.WaitGroup.Wait()` inside `Stop()` to ensure that in-flight database writes complete before the process exits.
- **Recover Panics Inside the Task Loop:** Wrap task execution in `defer func() { if r := recover(); r != nil { ... } }()` so an unexpected panic in a background task does not crash the HTTP API server.

### Pitfalls to Avoid
- **Spawning Unbounded Goroutines on Every Tick:** Spawning a new goroutine inside `case <-ticker.C:` without concurrency bounds causes unbounded concurrency if tasks run slower than the interval. Execute sequentially inside the select loop or use a worker pool.
- **Ignoring Drift on Fixed Intervals:** Understand that `time.Ticker` adjusts for minor execution time, but if task execution exceeds the interval, the ticker drops missed ticks rather than queueing them up.

---

## Code Walkthrough & Usage

### Core Implementation
The pattern implementation in `scheduled_runner.go` coordinates task scheduling and clean shutdowns:

```go
package backgroundjobs

import (
	"context"
	"sync"
	"time"
)

// ScheduledTask is a periodic background function.
type ScheduledTask func(ctx context.Context) error

// ScheduledTaskRunner executes a task periodically and supports clean cancellation.
type ScheduledTaskRunner struct {
	interval time.Duration
	task     ScheduledTask
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewScheduledTaskRunner(parentCtx context.Context, interval time.Duration, task ScheduledTask) *ScheduledTaskRunner {
	if interval <= 0 {
		interval = 1 * time.Minute
	}

	ctx, cancel := context.WithCancel(parentCtx)
	return &ScheduledTaskRunner{
		interval: interval,
		task:     task,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start launches the periodic ticker loop.
func (r *ScheduledTaskRunner) Start() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()

		for {
			select {
			case <-r.ctx.Done():
				return
			case <-ticker.C:
				_ = r.task(r.ctx)
			}
		}
	}()
}

// Stop cleanly cancels the scheduler context and waits for the active execution to complete.
func (r *ScheduledTaskRunner) Stop() {
	r.cancel()
	r.wg.Wait()
}
```

### Production Service Integration Example

```go
func main() {
    ctx := context.Background()

    // Define periodic cleanup task
    cleanupTask := func(taskCtx context.Context) error {
        log.Println("[CRON] Starting database session cleanup...")
        return db.DeleteExpiredSessions(taskCtx)
    }

    runner := backgroundjobs.NewScheduledTaskRunner(ctx, 15*time.Minute, cleanupTask)
    runner.Start()

    // Wait for termination signal...
    waitForSIGTERM()

    log.Println("Shutting down background runner...")
    runner.Stop()
    log.Println("Background runner stopped cleanly.")
}
```
