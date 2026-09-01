# 📂 REST API Design, Idempotency & ETag (`patterns/10_apidesign`)

> RESTful API engineering patterns covering response envelopes, API versioning strategies, ETag concurrency controls, and idempotency keys.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Multi-Strategy API Versioning** | [📖 Multi-Strategy API Versioning](./api_versioning.md) | [`api_versioning.go`](./api_versioning.go) | [`api_versioning_test.go`](./api_versioning_test.go) |
| **ETag Generation and Optimistic Concurrency Control** | [📖 ETag Generation and Optimistic Concurrency Control](./etag_concurrency.md) | [`etag_concurrency.go`](./etag_concurrency.go) | [`etag_concurrency_test.go`](./etag_concurrency_test.go) |
| **Idempotency Key Handling and Replay Caching** | [📖 Idempotency Key Handling and Replay Caching](./idempotency_keys.md) | [`idempotency_keys.go`](./idempotency_keys.go) | [`idempotency_keys_test.go`](./idempotency_keys_test.go) |
| **Safe Pagination, Filtering, and Sorting Parameter Parsing** | [📖 Safe Pagination, Filtering, and Sorting Parameter Parsing](./pagination_filtering.md) | [`pagination_filtering.go`](./pagination_filtering.go) | [`pagination_filtering_test.go`](./pagination_filtering_test.go) |
| **Partial Update (PATCH) with Pointer Semantics** | [📖 Partial Update (PATCH) with Pointer Semantics](./partial_update.md) | [`partial_update.go`](./partial_update.go) | [`partial_update_test.go`](./partial_update_test.go) |
| **Response Envelope, Pagination Metadata, and Output Filtering** | [📖 Response Envelope, Pagination Metadata, and Output Filtering](./response_envelope.md) | [`response_envelope.go`](./response_envelope.go) | [`response_envelope_test.go`](./response_envelope_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/10_apidesign/...
go test -race ./patterns/10_apidesign/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
