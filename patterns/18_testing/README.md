# 📂 Testing Patterns, Table-Driven & Failure Injection (`patterns/18_testing`)

> Rigorous Go testing patterns including table-driven unit tests, failure injection via test doubles, and stateful property invariant verifications.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Failure Injection & Chaos Testing Pattern** | [📖 Failure Injection & Chaos Testing Pattern](./failure_injection.md) | [`failure_injection.go`](./failure_injection.go) | [`failure_injection_test.go`](./failure_injection_test.go) |
| **Property-Based & Invariant Verification Pattern** | [📖 Property-Based & Invariant Verification Pattern](./property_invariant.md) | [`property_invariant.go`](./property_invariant.go) | [`property_invariant_test.go`](./property_invariant_test.go) |
| **Table-Driven Testing Pattern** | [📖 Table-Driven Testing Pattern](./table_driven.md) | [`table_driven.go`](./table_driven.go) | [`table_driven_test.go`](./table_driven_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/18_testing/...
go test -race ./patterns/18_testing/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
