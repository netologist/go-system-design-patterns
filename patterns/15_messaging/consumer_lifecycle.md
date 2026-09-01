# Consumer Lifecycle & Graceful Drain Pattern

## Overview & Definition
The **Consumer Lifecycle & Graceful Drain** pattern governs how background message consumer services start up, process concurrent work streams, handle OS termination signals (`SIGTERM`, `SIGINT`), and gracefully shut down without dropping in-flight messages or corrupting downstream datastores.

In containerized production environments (e.g., Kubernetes), deploying a new version of a service or scaling down a consumer deployment sends a `SIGTERM` signal to existing consumer pods. If a consumer process abruptly calls `os.Exit(0)` or kills worker goroutines while they are midway through executing database mutations or committing Kafka offsets, messages are left half-processed, database connections are severed abruptly, and transactions remain uncommitted.

The Graceful Consumer pattern coordinates:
1. Immediate cessation of pulling new messages from the broker.
2. Controlled draining of all in-flight messages already in local memory buffers.
3. Bounded wait deadline (`Shutdown(timeout)`) using `sync.WaitGroup` and context cancellation.

---

## Problem Statement (Failure Scenarios Without This Pattern)

### 1. In-Flight Message Abortion and Zombie Work
A consumer worker is executing a 3-second report generation task. At second 1.5, Kubernetes rolls out a new deployment and sends `SIGTERM`. Without graceful draining:
- The process terminates immediately.
- The external API call completes, but the local database update is never reached.
- When the message is redelivered to a new pod, it re-runs from the beginning, generating duplicate reports or double-billing clients.

### 2. Kafka Offset Rebalance Storms
If multiple consumer pods terminate abruptly without closing their consumer groups and committing in-flight offsets cleanly, Kafka triggers consecutive consumer group rebalances, causing the entire consumer cluster to freeze for 30–60 seconds while partition assignments are recalculated.

---

## Architectural Mechanism & Flow

```mermaid
sequenceDiagram
    autonumber
    actor OS as Kubernetes / OS Signal (SIGTERM)
    participant GC as GracefulConsumer Manager
    participant Queue as Internal Work Channel
    participant Workers as Worker Goroutines Pool (sync.WaitGroup)

    OS->>GC: Shutdown(timeout=10s)
    Note over GC: 1. Set closed=true (Stop accepting new Enqueue)
    Note over GC: 2. close(msgQueue) channel
    
    rect rgb(30, 45, 60)
        Note over Workers,Queue: 3. Drain In-Flight Work
        loop Until Queue Empty
            Workers->>Queue: Pull remaining buffered task
            Workers->>Workers: Complete execution task(ctx)
        end
        Note over Workers: All workers exit; call wg.Done()
    end

    alt Drained before timeout
        Workers-->>GC: wg.Wait() unblocks
        Note over GC: cancel() context cleanly
        GC-->>OS: Process exits with code 0 (Clean Drain)
    else Timeout Exceeded (10s)
        GC->>Workers: cancel() context (Force Abort)
        GC-->>OS: Return shutdown timeout error
    end
```

---

## Production Best Practices & Pitfalls

### Best Practices
- **Coordinate with Kubernetes Termination Grace Period**: Set the Go shutdown timeout (e.g., 25s) slightly below the Kubernetes `terminationGracePeriodSeconds` (e.g., 30s) to guarantee the process finishes its drain before receiving `SIGKILL`.
- **Channel Close for Worker Exit**: In Go, closing the input channel (`close(c.msgQueue)`) is the idiomatic way to signal worker goroutines to drain remaining buffered items and exit naturally once the channel is exhausted (`for task := range c.msgQueue`).
- **Context Cancellation on Timeout**: If the shutdown deadline expires before all tasks finish, cancel the shared context (`cancel()`) to immediately abort blocking I/O calls inside remaining workers.
- **Stop Broker Polling First**: Disconnect from Kafka/RabbitMQ first so no new messages enter the application, then drain the internal worker queues.

### Pitfalls to Avoid
- **Panic on Send to Closed Channel**: Always protect enqueue operations with a mutex and check the `closed` boolean state to avoid panicking on a closed Go channel.
- **Unbuffered Channels Under Burst Load**: Ensure internal task channels have sufficient capacity (`queueBuffer`) to absorb brief processing bursts without blocking the broker fetch loop.

---

## Code Walkthrough & Usage

In `patterns/15_messaging/consumer_lifecycle.go`, `GracefulConsumer` manages a pool of workers with drain mechanics:

```go
type ConsumerTask func(ctx context.Context) error

type GracefulConsumer struct {
    concurrency int
    msgQueue    chan ConsumerTask
    wg          sync.WaitGroup
    ctx         context.Context
    cancel      context.CancelFunc
    closed      bool
    mu          sync.Mutex
}

func NewGracefulConsumer(parentCtx context.Context, concurrency int, queueBuffer int) *GracefulConsumer {
    if concurrency <= 0 {
        concurrency = 4
    }
    if queueBuffer <= 0 {
        queueBuffer = 50
    }

    ctx, cancel := context.WithCancel(parentCtx)
    c := &GracefulConsumer{
        concurrency: concurrency,
        msgQueue:    make(chan ConsumerTask, queueBuffer),
        ctx:         ctx,
        cancel:      cancel,
    }

    c.start()
    return c
}
```

### Worker Loop and Draining
```go
func (c *GracefulConsumer) start() {
    for range c.concurrency {
        c.wg.Add(1)
        go func() {
            defer c.wg.Done()
            // Drains all items in msgQueue until channel is closed
            for task := range c.msgQueue {
                _ = task(c.ctx)
            }
        }()
    }
}
```

### Controlled Bounded Shutdown
```go
func (c *GracefulConsumer) Shutdown(timeout time.Duration) error {
    c.mu.Lock()
    if c.closed {
        c.mu.Unlock()
        return nil
    }
    c.closed = true
    close(c.msgQueue) // Closes channel: workers finish pending items then exit
    c.mu.Unlock()

    done := make(chan struct{})
    go func() {
        c.wg.Wait() // Wait for all worker goroutines to return
        close(done)
    }()

    select {
    case <-done:
        c.cancel()
        return nil // Clean graceful shutdown completed
    case <-time.After(timeout):
        c.cancel() // Force cancel remaining hung tasks
        return errors.New("consumer shutdown timed out waiting for in-flight messages")
    }
}
```
