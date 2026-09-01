# 📂 Resilient HTTP Client, Retries & Bulkheads (`patterns/08_httpclient`)

> Client-side resilience primitives including connection pooling, exponential backoff, circuit breakers, bulkhead isolation, and retry budgets.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Bulkhead Isolation Pattern** | [📖 Bulkhead Isolation Pattern](./bulkhead.md) | [`bulkhead.go`](./bulkhead.go) | [`bulkhead_test.go`](./bulkhead_test.go) |
| **Circuit Breaker Pattern** | [📖 Circuit Breaker Pattern](./circuit_breaker.md) | [`circuit_breaker.go`](./circuit_breaker.go) | [`circuit_breaker_test.go`](./circuit_breaker_test.go) |
| **Pooled HTTP Client Configuration** | [📖 Pooled HTTP Client Configuration](./pooled_client.md) | [`pooled_client.go`](./pooled_client.go) | [`pooled_client_test.go`](./pooled_client_test.go) |
| **Safe Response Body Handling and Status Code Validation** | [📖 Safe Response Body Handling and Status Code Validation](./response_handler.md) | [`response_handler.go`](./response_handler.go) | [`response_handler_test.go`](./response_handler_test.go) |
| **Exponential Backoff with Full Jitter and Context-Aware Retry** | [📖 Exponential Backoff with Full Jitter and Context-Aware Retry](./retry_backoff.md) | [`retry_backoff.go`](./retry_backoff.go) | [`retry_backoff_test.go`](./retry_backoff_test.go) |
| **Retry Budget** | [📖 Retry Budget](./retry_budget.md) | [`retry_budget.go`](./retry_budget.go) | [`retry_budget_test.go`](./retry_budget_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/08_httpclient/...
go test -race ./patterns/08_httpclient/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
