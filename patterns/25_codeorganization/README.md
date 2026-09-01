# 📂 Code Organization & Layered Architecture (`patterns/25_codeorganization`)

> Architectural structuring patterns including clean layered architecture and domain-driven bounded context isolation.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Domain Isolation & DTO Mapping Pattern** | [📖 Domain Isolation & DTO Mapping Pattern](./domain_isolation.md) | [`domain_isolation.go`](./domain_isolation.go) | [`domain_isolation_test.go`](./domain_isolation_test.go) |
| **Layered Architecture Pattern (Clean / Hexagonal Layer Separation)** | [📖 Layered Architecture Pattern (Clean / Hexagonal Layer Separation)](./layered_architecture.md) | [`layered_architecture.go`](./layered_architecture.go) | [`layered_architecture_test.go`](./layered_architecture_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/25_codeorganization/...
go test -race ./patterns/25_codeorganization/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
