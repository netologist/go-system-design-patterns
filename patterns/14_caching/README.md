# 📂 Caching Strategies, Invalidation & Stampedes (`patterns/14_caching`)

> Multi-tier caching architectures featuring cache-aside, proactive TTL invalidation, jittered TTL, negative caching, and stampede prevention.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Bounded LRU (Least Recently Used) Cache Pattern** | [📖 Bounded LRU (Least Recently Used) Cache Pattern](./bounded_lru.md) | [`bounded_lru.go`](./bounded_lru.go) | [`bounded_lru_test.go`](./bounded_lru_test.go) |
| **Cache-Aside (Lazy Loading) Pattern** | [📖 Cache-Aside (Lazy Loading) Pattern](./cache_aside.md) | [`cache_aside.go`](./cache_aside.go) | [`cache_aside_test.go`](./cache_aside_test.go) |
| **Jittered TTL (Cache Expiration Desynchronization) Pattern** | [📖 Jittered TTL (Cache Expiration Desynchronization) Pattern](./jittered_ttl.md) | [`jittered_ttl.go`](./jittered_ttl.go) | [`jittered_ttl_test.go`](./jittered_ttl_test.go) |
| **Negative Caching (Cache Penetration Defense) Pattern** | [📖 Negative Caching (Cache Penetration Defense) Pattern](./negative_caching.md) | [`negative_caching.go`](./negative_caching.go) | [`negative_caching_test.go`](./negative_caching_test.go) |
| **Cache Stampede (Thundering Herd) Prevention Pattern** | [📖 Cache Stampede (Thundering Herd) Prevention Pattern](./stampede_prevention.md) | [`stampede_prevention.go`](./stampede_prevention.go) | [`stampede_prevention_test.go`](./stampede_prevention_test.go) |
| **TTL and Tag-Based Cache Invalidation Pattern** | [📖 TTL and Tag-Based Cache Invalidation Pattern](./ttl_invalidation.md) | [`ttl_invalidation.go`](./ttl_invalidation.go) | [`ttl_invalidation_test.go`](./ttl_invalidation_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/14_caching/...
go test -race ./patterns/14_caching/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
