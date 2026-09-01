# Visibility Job Queue Pattern (Lease-Based Asynchronous Worker Queue)

## Overview & Definition
The **Visibility Job Queue Pattern** provides asynchronous, decoupled task processing using **lease-based visibility timeouts** (analogous to AWS SQS, Google Cloud Tasks, or RabbitMQ dead-letter queues).

When a worker goroutine dequeues a job from the queue, the job is not deleted immediately; instead, its `VisibleAfter` timestamp is set into the future (`time.Now() + visibilityTimeout`), rendering it invisible to other competing worker goroutines. If the worker processes the job successfully, it issues an **Acknowledgment (`Ack`)**, permanently deleting the job. If the worker crashes, panics, or hangs, the visibility timeout expires and the job naturally becomes visible again for another worker to retry.

For long-running tasks, the worker periodically sends a **Heartbeat** to extend the visibility lease before it expires.

---

## Problem Statement
Naive asynchronous job queues that delete tasks upon dequeuing or lack lease-extension mechanisms suffer from lost work and duplicate concurrent executions.

### Failure Scenarios Without This Pattern
- **Lost Jobs on Worker Crashes (At-Most-Once Anti-Pattern):** If a worker pops and deletes a job immediately before starting work, a container restart, OOM kill, or panic permanently destroys the job.
- **Duplicate Concurrent Processing on Long Jobs:** If a task takes 45 seconds but the fixed visibility timeout is 30 seconds without heartbeats, a second worker dequeues the same job while the first worker is still halfway through, causing dual execution.
- **Zombie Job Accumulation:** Unacknowledged failed jobs block FIFO queues if head-of-line blocking is present.
- **Worker Starvation:** Inefficient polling mechanisms consume excessive CPU cycles when queues are empty.

---

## Architectural Mechanism & Flow
The `VisibilityQueue` tracks jobs in a synchronized registry, updating visibility deadlines upon dequeue and heartbeat:

```
[ Producer: Enqueue(ID="job-1", MaxAttempts=3) ]
                     │
                     ▼
       ┌───────────────────────────┐
       │   Queue: VisibleAfter=Now │
       └─────────────┬─────────────┘
                     │
                     ▼
[ Worker 1: Dequeue() ] ──► [ VisibleAfter = Now + 30s ] ──► [ Returns Job to Worker 1 ]
                     │
                     │ (Job is now hidden from Worker 2 & 3)
                     │
         ┌───────────┴───────────┐
         ▼                       ▼
 [ Fast Execution ]      [ Long Task: >30s ]
         │                       │
         ▼                       ▼
  [ Execute Job ]        [ Worker Heartbeat() ]
         │               [ Extends VisibleAfter ]
         ▼                       │
   [ Queue.Ack() ]               ▼
   [ Permanently ]        [ Execute Job ]
   [ Deleted ]                   │
                                 ▼
                          [ Queue.Ack() ]
```

---

## Production Best Practices & Pitfalls

### Best Practices
- **Periodic Heartbeats for Variable Workloads:** Run a background goroutine alongside long-running tasks that calls `q.Heartbeat(jobID, 30*time.Second)` every 10 seconds until task completion.
- **Idempotent Job Consumers:** Because distributed queues provide **At-Least-Once Delivery**, jobs may occasionally be re-delivered if network acknowledgments drop. Ensure task handlers are strictly idempotent.
- **Track Attempt Counts:** Increment `job.Attempts` on each dequeue to enforce dead-letter queue (DLQ) quarantining after exceeding `MaxAttempts`.
- **Thread-Safe Mutex Boundaries:** Guard queue state with `sync.Mutex`, returning copies of job structures to prevent data races between workers and the queue registry.

### Pitfalls to Avoid
- **Unbounded Visibility Timeouts:** Setting a 2-hour visibility timeout means if a worker crashes on second 1, the job will not be retried for 2 hours. Use short timeouts (30s) combined with active heartbeats.
- **Forgetting to Acknowledge (`Ack`):** If a successful task forgets to call `Ack()`, it will be continuously reprocessed indefinitely.
- **Blocking Heartbeats on Business Logic:** Running heartbeats sequentially in the same goroutine as business logic defeats the heartbeat's purpose if the business logic blocks on a database query. Run heartbeats in a dedicated companion goroutine.

---

## Code Walkthrough & Usage

### Core Implementation
The pattern implementation in `job_queue.go` provides lease-based visibility queuing:

```go
package backgroundjobs

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrNoJobAvailable = errors.New("no jobs available in queue")
)

// Job represents a background task in the queue.
type Job struct {
	ID           string
	Payload      string
	VisibleAfter time.Time
	Attempts     int
	MaxAttempts  int
}

// VisibilityQueue manages background jobs with visibility timeout (SQS/RabbitMQ-style).
type VisibilityQueue struct {
	mu                sync.Mutex
	jobs              map[string]*Job
	visibilityTimeout time.Duration
}

func NewVisibilityQueue(visibilityTimeout time.Duration) *VisibilityQueue {
	if visibilityTimeout <= 0 {
		visibilityTimeout = 30 * time.Second
	}
	return &VisibilityQueue{
		jobs:              make(map[string]*Job),
		visibilityTimeout: visibilityTimeout,
	}
}

// Enqueue adds a job to the queue.
func (q *VisibilityQueue) Enqueue(id, payload string, maxAttempts int) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.jobs[id] = &Job{
		ID:           id,
		Payload:      payload,
		VisibleAfter: time.Now(),
		Attempts:     0,
		MaxAttempts:  maxAttempts,
	}
}

// Dequeue fetches the next visible job and hides it for visibilityTimeout duration.
func (q *VisibilityQueue) Dequeue(ctx context.Context) (*Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	for _, j := range q.jobs {
		if now.After(j.VisibleAfter) {
			j.Attempts++
			j.VisibleAfter = now.Add(q.visibilityTimeout)
			return &Job{
				ID:           j.ID,
				Payload:      j.Payload,
				Attempts:     j.Attempts,
				MaxAttempts:  j.MaxAttempts,
				VisibleAfter: j.VisibleAfter,
			}, nil
		}
	}

	return nil, ErrNoJobAvailable
}

// Heartbeat extends the visibility timeout of a currently running job.
func (q *VisibilityQueue) Heartbeat(id string, extension time.Duration) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	j, ok := q.jobs[id]
	if !ok {
		return errors.New("job not found")
	}

	j.VisibleAfter = time.Now().Add(extension)
	return nil
}

// Ack acknowledges completion and permanently removes the job.
func (q *VisibilityQueue) Ack(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.jobs, id)
}
```

### Production Worker Loop Example

```go
func StartWorker(ctx context.Context, queue *backgroundjobs.VisibilityQueue) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
            job, err := queue.Dequeue(ctx)
            if errors.Is(err, backgroundjobs.ErrNoJobAvailable) {
                time.Sleep(100 * time.Millisecond) // Back off when queue is empty
                continue
            }

            // Execute job with background heartbeat
            done := make(chan struct{})
            go func() {
                ticker := time.NewTicker(10 * time.Second)
                defer ticker.Stop()
                for {
                    select {
                    case <-done:
                        return
                    case <-ticker.C:
                        _ = queue.Heartbeat(job.ID, 30*time.Second)
                    }
                }
            }()

            // Run business logic
            processTaskPayload(job.Payload)
            close(done)

            // Acknowledge upon completion
            queue.Ack(job.ID)
        }
    }
}
```
