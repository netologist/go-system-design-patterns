# Bounded LRU (Least Recently Used) Cache Pattern

## 1. Overview & Concept
The **Bounded LRU (Least Recently Used) Cache** pattern implements an in-memory key-value store with a strictly enforced **upper bound on item capacity**. When the cache reaches its maximum allocated capacity, inserting a new element automatically triggers the eviction of the least recently accessed item.

In Go backend microservices, in-memory caching is frequently used for hot configuration items, parsed authorization policies, compiled regexes, and database query results. Without an explicit capacity bound and eviction strategy, unbounded `map[string]T` structures grow monotonically until the OS OOM (Out Of Memory) Killer terminates the Go process (`signal: killed`). A Bounded LRU cache provides deterministic memory bounds with constant-time $O(1)$ read, write, and eviction performance.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

### 1. Unbounded In-Memory Map OOM Crashes
A common Go antipattern is caching items in a plain `map[string]V` protected by `sync.RWMutex` without an eviction policy or maximum size limit:
```go
// ANTIPATTERN: Memory leak hazard
var cache = make(map[string]Data)
var mu sync.RWMutex
```
Under high cardinality of user IDs or URL parameters, this map grows indefinitely. Go's runtime garbage collector does not shrink map buckets even after keys are deleted, leading to unbounded heap fragmentation, severe GC pause spikes, and eventual Linux OOM killer termination.

### 2. $O(N)$ Eviction Latency Traps
Naive bounded cache designs that iterate over the entire map to find the oldest entry experience $O(N)$ eviction latency, introducing tail latency spikes (p99 > 100ms) on cache writes under load.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

The classic high-performance LRU architecture combines two data structures:
1. **Hash Map (`map[K]*list.Element`)**: Provides $O(1)$ key lookup to locate the node in memory.
2. **Doubly-Linked List (`container/list.List`)**: Maintains chronological access order. The front of the list holds the Most Recently Used (MRU) item; the back holds the Least Recently Used (LRU) item.

```mermaid
flowchart LR
    subgraph HashMap [Hash Map: map[K]*list.Element]
        K1["Key A"] --> N1
        K2["Key B"] --> N2
        K3["Key C"] --> N3
    end

    subgraph DoublyLinkedList [Doubly Linked List: container/list]
        MRU["[Front / MRU]"] --> N1["Node A (Val A)"]
        N1 <--> N2["Node B (Val B)"]
        N2 <--> N3["Node C (Val C)"]
        N3 --> LRU["[Back / LRU]"]
    end

    EvictAction["At Capacity: Evict Back (Node C)"] -.-> N3
```

### ASCII Mechanism Overview
```text
+-------------------------------------------------------------------------------+
|                       Bounded LRU Cache Mechanism                             |
|                                                                               |
|  Get("B"):                                                                    |
|  1. Locate Node B in map -> O(1)                                              |
|  2. Move Node B to Head of List (MRU) -> O(1)                                 |
|                                                                               |
|  Put("D", ValD) when Capacity = 3:                                            |
|  1. List: [A] <-> [B] <-> [C] (Back is C)                                     |
|  2. Evict Back [C]: Delete from map, remove node from list -> O(1)            |
|  3. Insert [D] at Head: Add to map, push to front -> O(1)                     |
|  4. New List: [D] <-> [A] <-> [B]                                             |
+-------------------------------------------------------------------------------+
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Production Hardening Strategies
1. **Hard Upper Bound Capacity**: Always enforce explicit `capacity` limits (e.g., 10,000 items) during cache initialization.
2. **Generic Type Safety**: Utilize Go generics (`[K comparable, V any]`) to eliminate interface boxing allocations and runtime type assertion overhead.
3. **Shard by Key for Concurrent Throughput**: Under extreme write concurrency, a single `sync.Mutex` across the entire LRU cache can become a lock contention bottleneck. Partition the cache into $N$ shards (e.g., 32 shards using `hash(key) % 32`), each with its own mutex and LRU list.
4. **Memory-Aware Eviction (Optional)**: For variable-sized payloads (e.g., raw image buffers), track total byte size rather than item count to evict based on max megabytes.

### Trade-offs & Pitfalls
- **Mutex Contention on Reads**: Unlike standard read-heavy caches where `RLock` allows concurrent reads, an LRU `Get()` mutates the underlying linked list (`MoveToFront`), requiring an exclusive `Lock()` unless a lock-free queue or channel-based updater is used.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

In `patterns/14_caching/bounded_lru.go`, `LRUCache[K, V]` combines Go generics with `container/list`:

```go
package caching

import (
	"container/list"
	"sync"
)

type lruEntry[K comparable, V any] struct {
	key   K
	value V
}

type LRUCache[K comparable, V any] struct {
	mu        sync.Mutex
	capacity  int
	items     map[K]*list.Element
	evictList *list.List
}

func NewLRUCache[K comparable, V any](capacity int) *LRUCache[K, V] {
	if capacity <= 0 {
		capacity = 100
	}

	return &LRUCache[K, V]{
		capacity:  capacity,
		items:     make(map[K]*list.Element),
		evictList: list.New(),
	}
}

func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.evictList.MoveToFront(elem)
		return elem.Value.(*lruEntry[K, V]).value, true
	}

	var zero V
	return zero, false
}

func (c *LRUCache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.evictList.MoveToFront(elem)
		elem.Value.(*lruEntry[K, V]).value = value
		return
	}

	// Evict oldest if full
	if c.evictList.Len() >= c.capacity {
		oldest := c.evictList.Back()
		if oldest != nil {
			c.evictList.Remove(oldest)
			kv := oldest.Value.(*lruEntry[K, V])
			delete(c.items, kv.key)
		}
	}

	// Insert new element at front
	entry := &lruEntry[K, V]{key: key, value: value}
	elem := c.evictList.PushFront(entry)
	c.items[key] = elem
}

func (c *LRUCache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}
```

### Usage Example
```go
lru := caching.NewLRUCache[string, *SessionToken](5000)

// Stores up to 5,000 tokens in memory:
lru.Put("sess_abc123", &SessionToken{UserID: "u_42"})

// Fetch updates recency:
if token, ok := lru.Get("sess_abc123"); ok {
    // Session token found in O(1)
}
```
