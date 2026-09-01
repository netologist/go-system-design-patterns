# 📂 Composable HTTP Middleware Chains (`patterns/11_middleware`)

> Composable HTTP middleware pipelines covering rate limiting, timeouts, CORS security policies, structured access logging, and metrics.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Structured Access Logger Middleware** | [📖 Structured Access Logger Middleware](./access_logger.md) | [`access_logger.go`](./access_logger.go) | [`access_logger_test.go`](./access_logger_test.go) |
| **Real-Time HTTP Metrics & Telemetry Middleware** | [📖 Real-Time HTTP Metrics & Telemetry Middleware](./metrics_tracing.md) | [`metrics_tracing.go`](./metrics_tracing.go) | [`metrics_tracing_test.go`](./metrics_tracing_test.go) |
| **Middleware Chain Pattern (Onion Architecture)** | [📖 Middleware Chain Pattern (Onion Architecture)](./middleware_chain.md) | [`middleware_chain.go`](./middleware_chain.go) | [`middleware_chain_test.go`](./middleware_chain_test.go) |
| **Per-Client Rate Limiting Middleware** | [📖 Per-Client Rate Limiting Middleware](./rate_limit_middleware.md) | [`rate_limit_middleware.go`](./rate_limit_middleware.go) | [`rate_limit_middleware_test.go`](./rate_limit_middleware_test.go) |
| **Security Headers & CORS Middleware** | [📖 Security Headers & CORS Middleware](./security_cors.md) | [`security_cors.go`](./security_cors.go) | [`security_cors_test.go`](./security_cors_test.go) |
| **Request Timeout Middleware** | [📖 Request Timeout Middleware](./timeout_middleware.md) | [`timeout_middleware.go`](./timeout_middleware.go) | [`timeout_middleware_test.go`](./timeout_middleware_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/11_middleware/...
go test -race ./patterns/11_middleware/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
