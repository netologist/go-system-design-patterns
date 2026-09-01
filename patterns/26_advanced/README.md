# 📂 Advanced Production & High-Throughput Patterns (`patterns/26_advanced`)

> Specialized high-scale patterns including singleflight cache with stale-while-revalidate fallback, multi-tiered bulkheads, and idempotent payment pipelines.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Latency EMA Adaptive Load Shedding Pattern** | [📖 Latency EMA Adaptive Load Shedding Pattern](./adaptive_shedder.md) | [`adaptive_shedder.go`](./adaptive_shedder.go) | [`adaptive_shedder_test.go`](./adaptive_shedder_test.go) |
| **End-to-End Idempotent Payment Pipeline Pattern** | [📖 End-to-End Idempotent Payment Pipeline Pattern](./idempotent_payment_pipeline.md) | [`idempotent_payment_pipeline.go`](./idempotent_payment_pipeline.go) | [`idempotent_payment_pipeline_test.go`](./idempotent_payment_pipeline_test.go) |
| **Advanced Jitter Strategies for Backoff (Full, Equal, Decorrelated)** | [📖 Advanced Jitter Strategies for Backoff (Full, Equal, Decorrelated)](./jitter_strategies.md) | [`jitter_strategies.go`](./jitter_strategies.go) | [`jitter_strategies_test.go`](./jitter_strategies_test.go) |
| **Multi-Workload Isolated Bulkhead Pattern** | [📖 Multi-Workload Isolated Bulkhead Pattern](./multi_bulkhead.md) | [`multi_bulkhead.go`](./multi_bulkhead.go) | [`multi_bulkhead_test.go`](./multi_bulkhead_test.go) |
| **Transactional Outbox with Reliable Relay Publisher** | [📖 Transactional Outbox with Reliable Relay Publisher](./reliable_outbox_publisher.md) | [`reliable_outbox_publisher.go`](./reliable_outbox_publisher.go) | [`reliable_outbox_publisher_test.go`](./reliable_outbox_publisher_test.go) |
| **Singleflight with Stale-While-Revalidate Cache Fallback** | [📖 Singleflight with Stale-While-Revalidate Cache Fallback](./singleflight_cache.md) | [`singleflight_cache.go`](./singleflight_cache.go) | [`singleflight_cache_test.go`](./singleflight_cache_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/26_advanced/...
go test -race ./patterns/26_advanced/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
