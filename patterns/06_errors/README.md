# 📂 Structured Error Handling & Mapping (`patterns/06_errors`)

> Comprehensive error handling architectures including sentinel error wrapping, typed domain errors, HTTP error mapping, and RFC 7807 responses.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Error Mapping Pattern (HTTP/RPC Translation)** | [📖 Error Mapping Pattern (HTTP/RPC Translation)](./error_mapping.md) | [`error_mapping.go`](./error_mapping.go) | [`error_mapping_test.go`](./error_mapping_test.go) |
| **Sentinel Error Wrapping Pattern** | [📖 Sentinel Error Wrapping Pattern](./sentinel_wrapping.md) | [`sentinel_wrapping.go`](./sentinel_wrapping.go) | [`sentinel_wrapping_test.go`](./sentinel_wrapping_test.go) |
| **Structured Error Response Pattern (RFC 7807 Problem Details)** | [📖 Structured Error Response Pattern (RFC 7807 Problem Details)](./structured_response.md) | [`structured_response.go`](./structured_response.go) | [`structured_response_test.go`](./structured_response_test.go) |
| **Typed Domain Errors Pattern** | [📖 Typed Domain Errors Pattern](./typed_errors.md) | [`typed_errors.go`](./typed_errors.go) | [`typed_errors_test.go`](./typed_errors_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/06_errors/...
go test -race ./patterns/06_errors/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
