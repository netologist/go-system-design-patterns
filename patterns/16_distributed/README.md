# 📂 Distributed Systems, Sagas & Consensus (`patterns/16_distributed`)

> Distributed systems coordination patterns including distributed locks (Redis/etcd), saga orchestration, leader election, and load shedders.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Distributed Lock (with Fencing Tokens) Pattern** | [📖 Distributed Lock (with Fencing Tokens) Pattern](./distributed_lock.md) | [`distributed_lock.go`](./distributed_lock.go) | [`distributed_lock_test.go`](./distributed_lock_test.go) |
| **Lease-Based Leader Election Pattern** | [📖 Lease-Based Leader Election Pattern](./leader_election.md) | [`leader_election.go`](./leader_election.go) | [`leader_election_test.go`](./leader_election_test.go) |
| **Adaptive Load Shedding Pattern** | [📖 Adaptive Load Shedding Pattern](./load_shedder.md) | [`load_shedder.go`](./load_shedder.go) | [`load_shedder_test.go`](./load_shedder_test.go) |
| **Continuous State Reconciliation (Convergence) Pattern** | [📖 Continuous State Reconciliation (Convergence) Pattern](./reconciliation.md) | [`reconciliation.go`](./reconciliation.go) | [`reconciliation_test.go`](./reconciliation_test.go) |
| **Saga Orchestrator Pattern** | [📖 Saga Orchestrator Pattern](./saga_orchestrator.md) | [`saga_orchestrator.go`](./saga_orchestrator.go) | [`saga_orchestrator_test.go`](./saga_orchestrator_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/16_distributed/...
go test -race ./patterns/16_distributed/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
