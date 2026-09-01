# Bulkhead Isolation Pattern

## Overview & Definition

Named after the watertight compartments in ship hulls that prevent a single hull breach from sinking the entire vessel, the **Bulkhead** pattern isolates system resources (concurrency slots, goroutines, connection pools) into discrete partitions.

The **Bulkhead Isolation** pattern:
1. Allocates dedicated concurrency limits (`PartitionSemaphore`) to specific downstream services, tenants, or workloads.
2. Manages partitions dynamically via a central `BulkheadRegistry`.
3. Rejects requests with `ErrBulkheadFull` when a specific partition reaches maximum capacity, guaranteeing that slow or hung downstream dependencies cannot monopolize shared server resources or starve unrelated operations.

---

## Problem Statement

Without bulkhead partitioning, all outbound requests share a single global pool of goroutines, sockets, and worker threads:

* **Cross-Service Contention:** If the Analytics service experiences 30-second latency, requests to Analytics stall. If 100 worker goroutines are waiting for Analytics, they consume all available database connections, memory, and HTTP client sockets, causing critical Payment and Authentication requests to time out.
* **Tenant Noisy Neighbor Problem:** In multi-tenant systems, a single customer running bulk exports or heavy batch jobs can saturate API concurrency, degrading service for all other tenants.
* **Total Service Collapse from Non-Critical Features:** A minor background feature (such as avatar generation or recommendation lookups) can take down the primary transactional checkout flow.

---

## Architectural Mechanism & Flow

```
                      Incoming Application Workloads
                       /             |             \
                      v              v              v
                 [Payments]      [Emails]      [Analytics]
                      \              |              /
                       v             v             v
             +-----------------------------------------------+
             |               BulkheadRegistry                |
             +-----------------------------------------------+
                     |               |               |
                     v               v               v
             +---------------+ +---------------+ +---------------+
             | Partition A:  | | Partition B:  | | Partition C:  |
             | Payments      | | Emails        | | Analytics     |
             | Concurrency=20| | Concurrency=10| | Concurrency=5 |
             +---------------+ +---------------+ +---------------+
                     |               |               |
               [Saturated!]       [Normal]        [Normal]
                     |               |               |
                     v               v               v
             +---------------+ +---------------+ +---------------+
             | 429/503 Fast  | | 200 OK Exec   | | 200 OK Exec   |
             | Bulkhead Full | | Success       | | Success       |
             +---------------+ +---------------+ +---------------+
```

### Partition Semaphore Mechanism
Each partition maintains a buffered channel initialized with empty structs `make(chan struct{}, maxConcurrency)`:
* **Acquisition:** Reading `<-sem.tokens` claims one slot. If the channel is empty, the partition is saturated.
* **Release:** Writing `sem.tokens <- struct{}{}` returns the slot to the pool.
* **Non-blocking / Context-aware check:** A `select` block attempts slot acquisition, failing immediately with `ErrBulkheadFull` or waiting if bounded by `ctx`.

---

## Production Best Practices & Pitfalls

### Best Practices
* **Tune Capacities Based on SLA and Criticality:** Core transactional flows (e.g. Orders, Payments) should receive higher concurrency allocations than non-essential integrations (e.g. Webhooks, Marketing syncs).
* **Pair with Fast-Fail Fallbacks:** When `registry.Execute` returns `ErrBulkheadFull`, return cached data, queue the job asynchronously, or return a 429/503 status code with a `Retry-After` header.
* **Monitor Partition Saturation:** Expose metrics tracking active tokens (`bulkhead_active_concurrency{partition="payments"}`) and total saturation rejections (`bulkhead_rejected_total{partition="payments"}`).

### Common Pitfalls
* **Setting Unbounded Partition Capacities:** Configuring partitions with arbitrarily large limits (e.g. 5,000 slots) renders the bulkhead ineffective, as OS thread and memory limits will be reached before the bulkhead activates.
* **Deadlocking Nested Partitions:** Avoid calling partition B from inside partition A if partition B might re-enter partition A. Keep partitions hierarchical or flat.

---

## Code Walkthrough & Usage

### 1. Implementation (`bulkhead.go`)

```go
package httpclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrBulkheadFull = errors.New("bulkhead capacity exceeded for partition")

// PartitionSemaphore manages concurrency for an isolated workload.
type PartitionSemaphore struct {
	tokens chan struct{}
}

func newPartitionSemaphore(maxConcurrency int) *PartitionSemaphore {
	tokens := make(chan struct{}, maxConcurrency)
	for range maxConcurrency {
		tokens <- struct{}{}
	}
	return &PartitionSemaphore{tokens: tokens}
}

// BulkheadRegistry coordinates isolated concurrency pools per downstream service / workload.
type BulkheadRegistry struct {
	mu         sync.RWMutex
	partitions map[string]*PartitionSemaphore
}

func NewBulkheadRegistry() *BulkheadRegistry {
	return &BulkheadRegistry{
		partitions: make(map[string]*PartitionSemaphore),
	}
}

// RegisterPartition defines maximum concurrency for a named workload.
func (b *BulkheadRegistry) RegisterPartition(name string, maxConcurrency int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if maxConcurrency <= 0 {
		maxConcurrency = 5
	}
	b.partitions[name] = newPartitionSemaphore(maxConcurrency)
}

// Execute runs fn within the specified bulkhead partition, rejecting if capacity is saturated.
func (b *BulkheadRegistry) Execute(ctx context.Context, partitionName string, fn func(ctx context.Context) error) error {
	b.mu.RLock()
	sem, ok := b.partitions[partitionName]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("unknown bulkhead partition '%s'", partitionName)
	}

	// Try to acquire slot non-blocking or with context
	select {
	case <-sem.tokens:
		defer func() {
			sem.tokens <- struct{}{}
		}()
		return fn(ctx)
	default:
		// Saturated -> Fail fast or wait on context
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: %w", ErrBulkheadFull, ctx.Err())
		case <-sem.tokens:
			defer func() {
				sem.tokens <- struct{}{}
			}()
			return fn(ctx)
		default:
			return fmt.Errorf("%w for partition '%s'", ErrBulkheadFull, partitionName)
		}
	}
}
```

### 2. Multi-Service Isolation Usage

```go
func main() {
    registry := NewBulkheadRegistry()
    registry.RegisterPartition("payments", 50)  // High capacity for core path
    registry.RegisterPartition("analytics", 5)  // Low capacity for heavy stats

    ctx := context.Background()

    // Analytics call will be throttled without affecting Payments
    err := registry.Execute(ctx, "analytics", func(c context.Context) error {
        return sendHeavyAnalyticsPayload(c)
    })
    if errors.Is(err, ErrBulkheadFull) {
        log.Println("Analytics saturated, dropping telemetry event safely")
    }
}
```
