# 📂 Hardened HTTP Server & Ingress (`patterns/07_httpserver`)

> Hardened HTTP server configurations, request body limits, strict JSON decoders, panic recovery middleware, and distributed trace ID injection.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Body Limit and Content-Type Enforcement** | [📖 Body Limit and Content-Type Enforcement](./body_limit.md) | [`body_limit.go`](./body_limit.go) | [`body_limit_test.go`](./body_limit_test.go) |
| **Hardened HTTP Server Configuration** | [📖 Hardened HTTP Server Configuration](./hardened_server.md) | [`hardened_server.go`](./hardened_server.go) | [`hardened_server_test.go`](./hardened_server_test.go) |
| **Panic Recovery and Stack Trace Capture** | [📖 Panic Recovery and Stack Trace Capture](./panic_recovery.md) | [`panic_recovery.go`](./panic_recovery.go) | [`panic_recovery_test.go`](./panic_recovery_test.go) |
| **Request and Correlation ID Propagation** | [📖 Request and Correlation ID Propagation](./request_correlation_id.md) | [`request_correlation_id.go`](./request_correlation_id.go) | [`request_correlation_id_test.go`](./request_correlation_id_test.go) |
| **Strict JSON Decoding** | [📖 Strict JSON Decoding](./strict_json.md) | [`strict_json.go`](./strict_json.go) | [`strict_json_test.go`](./strict_json_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/07_httpserver/...
go test -race ./patterns/07_httpserver/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
