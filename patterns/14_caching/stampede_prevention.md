# Cache Stampede (Thundering Herd) Prevention Pattern

## 1. Overview & Concept
A **Cache Stampede** (also known as a **Thundering Herd** or **Dog-Piling** problem) occurs when a heavily requested ("hot") cache key expires or is invalidated. Under high concurrency, hundreds or thousands of simultaneous incoming requests encounter a cache miss at the exact same instant.

Without synchronization, every single concurrent worker/goroutine independently attempts to query the database, recalculate the expensive computational payload, and write back to the cache. This sudden tidal wave of redundant computations instantly overwhelms the relational database, maxes out CPU cores, exhausts database connection pools, and frequently triggers cascading system-wide outages.

The **Cache Stampede Prevention** pattern uses **Singleflight request deduplication** to collapse concurrent requests for the same key into a single in-flight flight, while all other waiting callers block and receive the shared result.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

### 1. The Hot-Key Expiration Spike
Imagine an e-commerce homepage banner or viral product detail page served at 10,000 requests per second with a 10-minute cache TTL:
- At `09:59:59`: 10,000 req/sec served from cache in 0.2ms.
- At `10:00:00`: Key expires. 10,000 concurrent goroutines miss the cache simultaneously.
- **Result**: 10,000 identical heavy SQL queries hit the database at the same millisecond. DB connection pool maxes out, query latency surges from 5ms to 15s, and the entire backend crashes.

### 2. Cascading Failure Across Services
When the primary database stalls due to a cache stampede, upstream HTTP handlers block waiting for DB connections, exhausting server thread pools and triggering upstream circuit breaker trips.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

```mermaid
sequenceDiagram
    autonumber
    actor G1 as Goroutine 1 (Winner)
    actor G2 as Goroutine 2 (Waiting)
    actor G3 as Goroutine 3 (Waiting)
    participant SP as StampedeProtectedCache[V]
    participant SF as SingleflightGroup
    participant DB as Relational Database

    G1->>SP: GetOrCompute("hot_key")
    G2->>SP: GetOrCompute("hot_key")
    G3->>SP: GetOrCompute("hot_key")

    Note over SP,SF: Cache Miss on all callers
    SP->>SF: Do("hot_key", fetchFn)
    Note over SF: G1 enters; G2 & G3 register as waiters

    SF->>DB: Single DB Query (executed ONLY once!)
    DB-->>SF: Return Payload

    Note over SF: Populate Cache with TTL
    SF-->>G1: Return Payload
    SF-->>G2: Return Shared Payload (0 DB queries)
    SF-->>G3: Return Shared Payload (0 DB queries)
```

### ASCII Mechanism Overview
```text
+-------------------------------------------------------------------------------+
|                       StampedeProtectedCache Flow                             |
|                                                                               |
|  1000 Concurrent Goroutines -> GetOrCompute("hot_product")                    |
|                                        |                                      |
|                               (Cache Miss Detected)                           |
|                                        v                                      |
|                       [SingleflightGroup.Do("hot_product")]                   |
|                                  /           \                                |
|          (Goroutine 1: Winner)  /             \  (999 Goroutines: Waiters)    |
|                                v               v                              |
|                   [Execute fetchFn(ctx)]   [Wait on Channel / sync.WaitGroup] |
|                   [Single SQL Query]                   |                      |
|                   [Write to Cache]                     |                      |
|                                \                       /                      |
|                                 v                     v                       |
|                       All 1000 Goroutines receive identical result!           |
+-------------------------------------------------------------------------------+
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Production Hardening Strategies
1. **Double-Check Cache Inside Singleflight**: Inside the Singleflight closure, check the cache once more before hitting the database. If another instance or replica just populated the cache, you can return immediately without executing the DB query.
2. **Context Deadline Cancellation Handling**: If the winning goroutine's context is canceled, other waiting callers should not fail if their individual contexts are still valid. Standard `golang.org/x/sync/singleflight` or generic implementations manage proper channel broadcasting.
3. **Probabilistic Early Recomputation (XFetch)**: For mission-critical hot keys, background workers can probabilistically recompute values before expiration using the formula:
   $$-\beta \times \delta \times \ln(\text{rand}()) > \text{TTL}_{\text{remaining}}$$
   where $\delta$ is computation time and $\beta > 0$.

### Trade-offs & Pitfalls
- **Distributed Stampedes**: Singleflight deduplicates requests within a **single Go process**. Across a fleet of 50 Kubernetes pods, each pod will execute 1 query (reducing 10,000 queries to 50, which is still a 99.5% reduction). For cluster-wide deduplication, pair with distributed locks or probabilistic background warmers.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

In `patterns/14_caching/stampede_prevention.go`, `StampedeProtectedCache[V]` combines a generic cache with `SingleflightGroup[V]`:

```go
package caching

import (
	"context"
	"time"

	concurrency "system-design-patterns/patterns/09_concurrency"
)

type StampedeProtectedCache[V any] struct {
	cache CacheClient[V]
	group *concurrency.SingleflightGroup[V]
}

func NewStampedeProtectedCache[V any](cache CacheClient[V]) *StampedeProtectedCache[V] {
	return &StampedeProtectedCache[V]{
		cache: cache,
		group: concurrency.NewSingleflightGroup[V](),
	}
}

func (s *StampedeProtectedCache[V]) GetOrCompute(
	ctx context.Context,
	key string,
	ttl time.Duration,
	fetchFn func(ctx context.Context) (V, error),
) (V, error) {
	// 1. Fast path: cache hit
	if val, err := s.cache.Get(ctx, key); err == nil {
		return val, nil
	}

	// 2. Cache miss -> Singleflight collapsed execution
	val, err, _ := s.group.Do(key, func() (V, error) {
		// Double check cache inside singleflight lock
		if v, err := s.cache.Get(ctx, key); err == nil {
			return v, nil
		}

		res, err := fetchFn(ctx)
		if err != nil {
			var zero V
			return zero, err
		}

		_ = s.cache.Set(ctx, key, res, ttl)
		return res, nil
	})

	return val, err
}
```

### Usage Example
```go
cache := caching.NewMemoryCache[ProductDetails]()
protectedCache := caching.NewStampedeProtectedCache[ProductDetails](cache)

// 5,000 goroutines call this simultaneously:
product, err := protectedCache.GetOrCompute(ctx, "hot_item_99", 5*time.Minute, func(ctx context.Context) (ProductDetails, error) {
    return expensiveDatabaseCalculation(ctx, "hot_item_99")
})
```
