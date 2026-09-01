# 📂 Context Lifecycle & Cancellation Pipelines (`patterns/05_context`)

> Patterns for propagating deadlines, scoped timeouts, cancellation trees, and type-safe request-scoped metadata across service boundaries.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Cancellation Pipeline Pattern (Concurrent Worker Pool)** | [📖 Cancellation Pipeline Pattern (Concurrent Worker Pool)](./cancellation_pipeline.md) | [`cancellation_pipeline.go`](./cancellation_pipeline.go) | [`cancellation_pipeline_test.go`](./cancellation_pipeline_test.go) |
| **Context Propagation Pattern** | [📖 Context Propagation Pattern](./context_propagation.md) | [`context_propagation.go`](./context_propagation.go) | [`context_propagation_test.go`](./context_propagation_test.go) |
| **Scoped Timeout & Budget Management Pattern** | [📖 Scoped Timeout & Budget Management Pattern](./scoped_timeout.md) | [`scoped_timeout.go`](./scoped_timeout.go) | [`scoped_timeout_test.go`](./scoped_timeout_test.go) |
| **Typed Context Values Pattern** | [📖 Typed Context Values Pattern](./typed_context_values.md) | [`typed_context_values.go`](./typed_context_values.go) | [`typed_context_values_test.go`](./typed_context_values_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/05_context/...
go test -race ./patterns/05_context/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
