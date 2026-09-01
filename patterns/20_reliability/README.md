# 📂 Reliability Engineering & Overload Protection (`patterns/20_reliability`)

> Resilience engineering patterns for isolating failure blast radiuses with bulkheads, protecting against CPU saturation, and graceful degradation.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Bulkhead Shield Pattern (Dependency Concurrency Isolation)** | [📖 Bulkhead Shield Pattern (Dependency Concurrency Isolation)](./bulkhead_shield.md) | [`bulkhead_shield.go`](./bulkhead_shield.go) | [`bulkhead_shield_test.go`](./bulkhead_shield_test.go) |
| **Graceful Degradation Pattern** | [📖 Graceful Degradation Pattern](./graceful_degradation.md) | [`graceful_degradation.go`](./graceful_degradation.go) | [`graceful_degradation_test.go`](./graceful_degradation_test.go) |
| **Overload Protection Pattern (Global Concurrency Limiter)** | [📖 Overload Protection Pattern (Global Concurrency Limiter)](./overload_protection.md) | [`overload_protection.go`](./overload_protection.go) | [`overload_protection_test.go`](./overload_protection_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/20_reliability/...
go test -race ./patterns/20_reliability/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
