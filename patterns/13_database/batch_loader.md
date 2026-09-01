# Batch Loader (DataLoader) Pattern

## 1. Overview & Concept
The **Batch Loader** (commonly known as the **DataLoader** pattern) solves the ubiquitous **N+1 query problem** in concurrent backend systems and GraphQL resolvers. Instead of dispatching immediate, individual SQL queries or RPC requests for every single entity lookup across concurrent goroutines or nested entity resolvers, the Batch Loader aggregates lookups across a micro-window of time (e.g., 2–5ms) or up to a maximum batch size into a single, efficient bulk operation (`WHERE id IN (?, ?, ...)`).

In Go, generic type parameters (`BatchLoader[K comparable, V any]`) allow building high-performance, type-safe batch loaders that seamlessly collect requests from concurrent goroutines, coalesce duplicate keys, execute a single batch fetch, and dispatch individual results back to the original callers via channels.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

### 1. The N+1 Query Cascade
Consider rendering an activity feed of 100 posts, where each post requires resolving its author:
- **Without Batching**: 1 query for posts + 100 individual queries for authors = **101 round-trips** to the database.
- **Latency Impact**: At 2ms network round-trip per query, serial execution takes >200ms; even parallel goroutines hammer the database connection pool, leading to queue delays, socket starvation, and excessive database context switching.

### 2. Redundant Duplicate Lookups
Multiple concurrent entities frequently reference the same parent object (e.g., 50 posts authored by the same 3 users). Without deduplication at the loader layer, the database processes identical queries repeatedly, burning CPU and buffer pool cache.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

```mermaid
sequenceDiagram
    autonumber
    actor G1 as Goroutine 1 (Item A)
    actor G2 as Goroutine 2 (Item B)
    actor G3 as Goroutine 3 (Item A - Duplicate)
    participant BL as BatchLoader[K, V]
    participant DB as Relational Database

    G1->>BL: Load(ctx, Key="101")
    Note over BL: Append to pending queue; start 2ms timer
    G2->>BL: Load(ctx, Key="102")
    Note over BL: Append to pending queue
    G3->>BL: Load(ctx, Key="101")
    Note over BL: Append to pending queue (Key 101 duplicated)

    Note over BL: Timer fires OR maxBatchSize reached -> flush()
    BL->>BL: Deduplicate keys -> ["101", "102"]
    BL->>DB: Single Batch Query: SELECT * FROM items WHERE id IN (101, 102)
    DB-->>BL: Return Map[101: ValueA, 102: ValueB]

    BL-->>G1: Channel send -> ValueA
    BL-->>G2: Channel send -> ValueB
    BL-->>G3: Channel send -> ValueA (Duplicate served from same batch)
```

### ASCII Mechanism Overview
```text
+-------------------------------------------------------------------------------+
|                       BatchLoader[K, V] Concurrent Queue                      |
|                                                                               |
|  Goroutine 1: Load(Key="A") ---\                                              |
|  Goroutine 2: Load(Key="B") ----+--> [Pending Buffer: A, B, A, C]              |
|  Goroutine 3: Load(Key="A") ---/      |                                       |
|  Goroutine 4: Load(Key="C") --/       | (Timer 2ms fires OR Size >= 100)      |
|                                       v                                       |
|                                [Deduplicate Keys] -> ["A", "B", "C"]          |
|                                       |                                       |
|                                       v                                       |
|                            [Single DB Query: IN (A, B, C)]                    |
|                                       |                                       |
|                                       v                                       |
|                            [Distribute via Buffered Channels]                 |
|                             -> G1: A, G2: B, G3: A, G4: C                     |
+-------------------------------------------------------------------------------+
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Production Hardening Strategies
1. **Scope Per Request Context**: In HTTP/GraphQL servers, instantiate the `BatchLoader` **per request context** (or attach it to `r.Context()`) so caching and batching apply across the request lifecycle without leaking cross-tenant data.
2. **Buffer Sizing for Channels**: Allocate single-element buffered channels (`make(chan V, 1)`) for result delivery. This prevents goroutine leaks if a caller's context times out and abandons reading from the channel.
3. **Database Parameter Limits**: Relational databases enforce limits on SQL query parameters (e.g., PostgreSQL default `65,535`, SQLite `999`). Configure `maxBatchSize` (e.g., 100–500) to keep `IN (...)` queries within database limits.
4. **Key Deduplication**: Coalesce identical keys before passing them to the user-supplied batch function (`batchFn`) to minimize payload size over the wire.

### Trade-offs & Pitfalls
- **Micro-Latency Trade-off**: An individual lookup waits up to `waitDuration` (e.g., 2ms) before dispatching, trading a negligible single-lookup micro-delay for massive global throughput gains.
- **Error Distribution**: If the underlying batch query fails, the error is distributed to all callers participating in that specific batch.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

In `patterns/13_database/batch_loader.go`, `BatchLoader[K, V]` uses Go generics, channels, and a timed queue:

```go
package database

import (
	"context"
	"errors"
	"sync"
	"time"
)

type loadRequest[K comparable, V any] struct {
	key    K
	result chan V
	err    chan error
}

type BatchLoader[K comparable, V any] struct {
	mu           sync.Mutex
	batchFn      func(ctx context.Context, keys []K) (map[K]V, error)
	waitDuration time.Duration
	maxBatchSize int
	pending      []loadRequest[K, V]
	timer        *time.Timer
}

func NewBatchLoader[K comparable, V any](
	waitDuration time.Duration,
	maxBatchSize int,
	batchFn func(ctx context.Context, keys []K) (map[K]V, error),
) *BatchLoader[K, V] {
	if waitDuration <= 0 {
		waitDuration = 2 * time.Millisecond
	}
	if maxBatchSize <= 0 {
		maxBatchSize = 100
	}

	return &BatchLoader[K, V]{
		batchFn:      batchFn,
		waitDuration: waitDuration,
		maxBatchSize: maxBatchSize,
	}
}

func (b *BatchLoader[K, V]) Load(ctx context.Context, key K) (V, error) {
	req := loadRequest[K, V]{
		key:    key,
		result: make(chan V, 1),
		err:    make(chan error, 1),
	}

	b.mu.Lock()
	b.pending = append(b.pending, req)

	if len(b.pending) >= b.maxBatchSize {
		if b.timer != nil {
			b.timer.Stop()
			b.timer = nil
		}
		pendingToFlush := b.pending
		b.pending = nil
		b.mu.Unlock()
		go b.flush(ctx, pendingToFlush)
	} else {
		if b.timer == nil {
			b.timer = time.AfterFunc(b.waitDuration, func() {
				b.mu.Lock()
				pendingToFlush := b.pending
				b.pending = nil
				b.timer = nil
				b.mu.Unlock()
				if len(pendingToFlush) > 0 {
					b.flush(ctx, pendingToFlush)
				}
			})
		}
		b.mu.Unlock()
	}

	select {
	case <-ctx.Done():
		var zero V
		return zero, ctx.Err()
	case err := <-req.err:
		var zero V
		return zero, err
	case val := <-req.result:
		return val, nil
	}
}
```

### Usage Example
```go
userLoader := database.NewBatchLoader[string, *User](
    2*time.Millisecond,
    100,
    func(ctx context.Context, userIDs []string) (map[string]*User, error) {
        return queryUsersByIDs(ctx, userIDs) // SELECT * FROM users WHERE id IN (...)
    },
)

// Called concurrently from multiple HTTP handler goroutines:
user, err := userLoader.Load(ctx, "user_101")
```
