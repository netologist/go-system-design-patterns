# 📂 Data Integrity, State Machines & Domain Invariants (`patterns/23_dataintegrity`)

> Domain integrity patterns including domain-driven invariants, strict finite state machines, and pessimistic database locking.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Domain Invariants & Audit Trail Pattern** | [📖 Domain Invariants & Audit Trail Pattern](./domain_invariants.md) | [`domain_invariants.go`](./domain_invariants.go) | [`domain_invariants_test.go`](./domain_invariants_test.go) |
| **Pessimistic Locking Pattern (Keyed Mutex Lock)** | [📖 Pessimistic Locking Pattern (Keyed Mutex Lock)](./pessimistic_lock.md) | [`pessimistic_lock.go`](./pessimistic_lock.go) | [`pessimistic_lock_test.go`](./pessimistic_lock_test.go) |
| **Finite State Machine (FSM) Pattern** | [📖 Finite State Machine (FSM) Pattern](./state_machine.md) | [`state_machine.go`](./state_machine.go) | [`state_machine_test.go`](./state_machine_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/23_dataintegrity/...
go test -race ./patterns/23_dataintegrity/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
