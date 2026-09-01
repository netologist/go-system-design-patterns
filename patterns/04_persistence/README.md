# 📂 Persistence, Repositories & Locking (`patterns/04_persistence`)

> Production patterns for data storage abstractions, unit of work transactions, keyset pagination, optimistic locking, and soft deletes.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Idempotent Soft Delete & Upsert Pattern** | [📖 Idempotent Soft Delete & Upsert Pattern](./idempotent_soft_delete.md) | [`idempotent_soft_delete.go`](./idempotent_soft_delete.go) | [`idempotent_soft_delete_test.go`](./idempotent_soft_delete_test.go) |
| **Keyset Pagination Pattern (Cursor-Based Pagination)** | [📖 Keyset Pagination Pattern (Cursor-Based Pagination)](./keyset_pagination.md) | [`keyset_pagination.go`](./keyset_pagination.go) | [`keyset_pagination_test.go`](./keyset_pagination_test.go) |
| **Optimistic Locking Pattern** | [📖 Optimistic Locking Pattern](./optimistic_locking.md) | [`optimistic_locking.go`](./optimistic_locking.go) | [`optimistic_locking_test.go`](./optimistic_locking_test.go) |
| **Connection Pool Limiter Pattern** | [📖 Connection Pool Limiter Pattern](./pool_limiter.md) | [`pool_limiter.go`](./pool_limiter.go) | [`pool_limiter_test.go`](./pool_limiter_test.go) |
| **Repository Pattern** | [📖 Repository Pattern](./repository_pattern.md) | [`repository_pattern.go`](./repository_pattern.go) | [`repository_pattern_test.go`](./repository_pattern_test.go) |
| **Unit of Work Pattern** | [📖 Unit of Work Pattern](./unit_of_work.md) | [`unit_of_work.go`](./unit_of_work.go) | [`unit_of_work_test.go`](./unit_of_work_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/04_persistence/...
go test -race ./patterns/04_persistence/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
