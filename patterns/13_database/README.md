# 📂 Database Architecture, Splitters & Deadlocks (`patterns/13_database`)

> Relational database patterns covering explicit transaction boundaries, read/write replica splitters, automatic deadlock retries, and batch loading.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Batch Loader (DataLoader) Pattern** | [📖 Batch Loader (DataLoader) Pattern](./batch_loader.md) | [`batch_loader.go`](./batch_loader.go) | [`batch_loader_test.go`](./batch_loader_test.go) |
| **Deadlock Retry Pattern** | [📖 Deadlock Retry Pattern](./deadlock_retry.md) | [`deadlock_retry.go`](./deadlock_retry.go) | [`deadlock_retry_test.go`](./deadlock_retry_test.go) |
| **Expand and Contract (Parallel Run) Migration Pattern** | [📖 Expand and Contract (Parallel Run) Migration Pattern](./expand_contract.md) | [`expand_contract.go`](./expand_contract.go) | [`expand_contract_test.go`](./expand_contract_test.go) |
| **Read-Write Splitter Pattern (with Replica Lag Awareness)** | [📖 Read-Write Splitter Pattern (with Replica Lag Awareness)](./read_write_splitter.md) | [`read_write_splitter.go`](./read_write_splitter.go) | [`read_write_splitter_test.go`](./read_write_splitter_test.go) |
| **Transaction Boundary Pattern** | [📖 Transaction Boundary Pattern](./transaction_boundary.md) | [`transaction_boundary.go`](./transaction_boundary.go) | [`transaction_boundary_test.go`](./transaction_boundary_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/13_database/...
go test -race ./patterns/13_database/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
