# 📂 Concurrency Primitives, Worker Pools & Queues (`patterns/09_concurrency`)

> Core Go concurrency models including worker pools, fan-out/fan-in pipelines, token bucket rate limiters, errgroups, and backpressure queues.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Bounded Queue with Backpressure Overflow Strategies** | [📖 Bounded Queue with Backpressure Overflow Strategies](./backpressure_queue.md) | [`backpressure_queue.go`](./backpressure_queue.go) | [`backpressure_queue_test.go`](./backpressure_queue_test.go) |
| **Channel Ownership, Mutex Discipline, and Atomic Synchronization** | [📖 Channel Ownership, Mutex Discipline, and Atomic Synchronization](./channel_ownership.md) | [`channel_ownership.go`](./channel_ownership.go) | [`channel_ownership_test.go`](./channel_ownership_test.go) |
| **Error Group with Context Cancellation** | [📖 Error Group with Context Cancellation](./errgroup.md) | [`errgroup.go`](./errgroup.go) | [`errgroup_test.go`](./errgroup_test.go) |
| **Fan-Out / Fan-In Pipeline Pattern** | [📖 Fan-Out / Fan-In Pipeline Pattern](./fanout_fanin.md) | [`fanout_fanin.go`](./fanout_fanin.go) | [`fanout_fanin_test.go`](./fanout_fanin_test.go) |
| **Token Bucket Rate Limiter Pattern** | [📖 Token Bucket Rate Limiter Pattern](./rate_limiter.md) | [`rate_limiter.go`](./rate_limiter.go) | [`rate_limiter_test.go`](./rate_limiter_test.go) |
| **Weighted Counting Semaphore Pattern** | [📖 Weighted Counting Semaphore Pattern](./semaphore.md) | [`semaphore.go`](./semaphore.go) | [`semaphore_test.go`](./semaphore_test.go) |
| **Singleflight Request Deduplication Pattern** | [📖 Singleflight Request Deduplication Pattern](./singleflight.md) | [`singleflight.go`](./singleflight.go) | [`singleflight_test.go`](./singleflight_test.go) |
| **Bounded Worker Pool Pattern** | [📖 Bounded Worker Pool Pattern](./worker_pool.md) | [`worker_pool.go`](./worker_pool.go) | [`worker_pool_test.go`](./worker_pool_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/09_concurrency/...
go test -race ./patterns/09_concurrency/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
