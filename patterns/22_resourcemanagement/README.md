# 📂 Resource Management & Buffer Pooling (`patterns/22_resourcemanagement`)

> High-throughput memory allocation optimizations using sync.Pool buffer reuse and strict deterministic resource lifecycle management.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Buffer Pool Pattern (Bounded Memory Reuse with `sync.Pool`)** | [📖 Buffer Pool Pattern (Bounded Memory Reuse with `sync.Pool`)](./buffer_pool.md) | [`buffer_pool.go`](./buffer_pool.go) | [`buffer_pool_test.go`](./buffer_pool_test.go) |
| **Resource Lifecycle Management Pattern** | [📖 Resource Lifecycle Management Pattern](./resource_lifecycle.md) | [`resource_lifecycle.go`](./resource_lifecycle.go) | [`resource_lifecycle_test.go`](./resource_lifecycle_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/22_resourcemanagement/...
go test -race ./patterns/22_resourcemanagement/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
