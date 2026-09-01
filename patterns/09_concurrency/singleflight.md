# Singleflight Request Deduplication Pattern

## Overview & Definition

When a popular cache key expires or a high-traffic endpoint experiences a sudden surge, hundreds or thousands of concurrent requests might attempt to fetch or compute the exact same piece of data simultaneously. This creates the **Cache Stampede (Thundering Herd)** problem.

The **Singleflight Request Deduplication** pattern (inspired by `golang.org/x/sync/singleflight`) ensures that for any given key, **only one execution of a function is in-flight at any given moment**.
* The first caller initiates the execution.
* Concurrent duplicate callers with the same key register themselves as waiting participants (`call.wg.Wait()`).
* When the original execution finishes, all waiting callers unblock simultaneously, receiving the exact same output value, error, and a boolean flag `shared = true`.

---

## Problem Statement

Without request deduplication:

* **Cache Stampede / Thundering Herd:** When a hot Redis/Memcached cache key expires under 10,000 RPS, all 10,000 requests miss the cache simultaneously and slam the primary relational database with identical expensive queries, causing immediate database CPU spikes and connection pool starvation.
* **Redundant Computational Work:** In microservices calculating cryptographic hashes, generating PDFs, or encoding media, executing identical computations in parallel wastes severe CPU capacity.
* **High Memory Pressure from Duplicate Allocations:** Multiple identical in-flight query responses allocate duplicate large object graphs in heap memory.

---

## Architectural Mechanism & Flow

```
                      Caller 1 (Key: "user:123")      Caller 2 (Key: "user:123")
                                |                                |
                                v                                v
                      [Acquire Mutex Lock]            [Acquire Mutex Lock]
                                |                                |
                      [Key NOT in map]                 [Key EXISTS in map]
                                |                                |
                      [Create new call struct]         [Set shared = true]
                      [Add call to map]                [Release Mutex Lock]
                      [call.wg.Add(1)]                           |
                      [Release Mutex Lock]                       v
                                |                         [call.wg.Wait()]
                                v                          (Blocks quietly)
                      [Execute fn()]                             |
                      (Queries Database)                         |
                                |                                |
                      [Assign c.val, c.err]                      |
                      [call.wg.Done()] ------------------------->+
                                |                                |
                      [Acquire Mutex Lock]                       v
                      [Delete key from map]            [Receive c.val, c.err]
                      [Release Mutex Lock]             [shared = true]
                                |
                                v
                      [Return c.val, c.err]
                      [shared = false]
```

### Key Concurrency Invariant
The key is deleted from `g.m` inside a lock *after* `c.wg.Done()` has unblocked all waiting participants. This ensures subsequent requests arriving *after* completion will initiate a fresh execution rather than reusing stale pointers.

---

## Production Best Practices & Pitfalls

### Best Practices
* **Use with Read-Through Caching:** Singleflight is most effective when placed immediately behind a cache check:
  `Check Cache -> Miss -> Singleflight.Do(Fetch & Populate Cache) -> Return`.
* **Inspect the `shared` Flag:** Use `shared` to log or track how many redundant database roundtrips were avoided by the singleflight group.
* **Keep Cache Scopes Granular:** Name keys with precise scoping (e.g. `user:profile:123`, `product:details:456`) rather than broad prefixes to avoid unintentional blocking of distinct data lookups.

### Common Pitfalls
* **Mutating Returned Pointers:** When `fn()` returns a pointer to a struct or slice, all callers receive the *same memory pointer*. If one caller mutates the returned object, it causes data races across all shared callers. Always return immutable types or deep-copy pointers before mutation.
* **Deadlocking with Nested Singleflight Calls:** Avoid calling `group.Do(k1, ...)` from inside `group.Do(k1, ...)`, as this will cause an unrecoverable self-deadlock.

---

## Code Walkthrough & Usage

### 1. Implementation (`singleflight.go`)

```go
package concurrency

import (
	"sync"
)

type call[T any] struct {
	wg     sync.WaitGroup
	val    T
	err    error
	shared bool
}

// SingleflightGroup executes only one in-flight execution for a given key.
type SingleflightGroup[T any] struct {
	mu sync.Mutex
	m  map[string]*call[T]
}

func NewSingleflightGroup[T any]() *SingleflightGroup[T] {
	return &SingleflightGroup[T]{
		m: make(map[string]*call[T]),
	}
}

// Do executes fn and returns the result, ensuring that only one execution is in-flight for a given key.
func (g *SingleflightGroup[T]) Do(key string, fn func() (T, error)) (v T, err error, shared bool) {
	g.mu.Lock()
	if c, ok := g.m[key]; ok {
		c.shared = true
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err, true
	}

	c := new(call[T])
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err, c.shared
}
```

### 2. Cache Stampede Defense Example

```go
type UserService struct {
    cache   *redis.Client
    db      *sql.DB
    sfGroup *SingleflightGroup[User]
}

func (s *UserService) GetUser(ctx context.Context, userID string) (User, error) {
    cacheKey := "user:" + userID

    // 1. Fast path: check distributed cache
    if val, err := s.cache.Get(ctx, cacheKey).Result(); err == nil {
        var user User
        _ = json.Unmarshal([]byte(val), &user)
        return user, nil
    }

    // 2. Cache miss: deduplicate concurrent DB queries
    user, err, _ := s.sfGroup.Do(cacheKey, func() (User, error) {
        // Only 1 goroutine executes this DB query
        u, err := s.queryUserFromDB(ctx, userID)
        if err != nil {
            return User{}, err
        }

        // Populate cache for future callers
        data, _ := json.Marshal(u)
        s.cache.Set(ctx, cacheKey, data, 5*time.Minute)
        return u, nil
    })

    return user, err
}
```
