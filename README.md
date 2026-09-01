# 🏛️ Production System Design & Backend Architecture Patterns in Go

A comprehensive, production-grade reference implementation of **118 backend design patterns, concurrency models, and distributed systems primitives** in idiomatic Go (Go 1.22+ / 1.26+).

Every category contains a dedicated README overview, architectural failure-mode analysis documents (`.md`), thread-safe Go source implementations (`.go`), and full test suites (`_test.go`).

Created and maintained by **Hasan Özgan** ([netologist.org](https://netologist.org) · [@netologist](https://github.com/netologist)).

---

## 🧭 Architecture Categories & Packages

Click on any category to explore its patterns, specifications, and implementations:

| # | Category | Core Patterns & Highlights | Package Documentation |
|:---|:---|:---|:---|
| **01** | [**Application Lifecycle**](patterns/01_lifecycle/README.md) | Graceful Shutdown, Graceful Startup, Signal Handling, Health Probes, Composition Root | [`patterns/01_lifecycle/README.md`](patterns/01_lifecycle/README.md) |
| **02** | [**Configuration Management**](patterns/02_config/README.md) | Typed Config, Immutable Config, Config Validator, Secret Sanitizer | [`patterns/02_config/README.md`](patterns/02_config/README.md) |
| **03** | [**Dependency Injection**](patterns/03_di/README.md) | Constructor Injection, Interface at Consumer, Functional Options, Test Doubles | [`patterns/03_di/README.md`](patterns/03_di/README.md) |
| **04** | [**Persistence & Storage**](patterns/04_persistence/README.md) | Repository Pattern, Unit of Work, Keyset Pagination, Optimistic Locking, Soft Delete, Pool Limiter | [`patterns/04_persistence/README.md`](patterns/04_persistence/README.md) |
| **05** | [**Context & Cancellation**](patterns/05_context/README.md) | Context Propagation, Scoped Timeout, Cancellation Pipeline, Typed Context Values | [`patterns/05_context/README.md`](patterns/05_context/README.md) |
| **06** | [**Error Handling**](patterns/06_errors/README.md) | Typed Errors, Sentinel Wrapping, Error Mapping, Structured Response | [`patterns/06_errors/README.md`](patterns/06_errors/README.md) |
| **07** | [**HTTP Server & Ingress**](patterns/07_httpserver/README.md) | Hardened Server, Request Correlation ID, Strict JSON Decoder, Panic Recovery, Body Limit | [`patterns/07_httpserver/README.md`](patterns/07_httpserver/README.md) |
| **08** | [**HTTP Client & Resilience**](patterns/08_httpclient/README.md) | Pooled Client, Retry Backoff, Circuit Breaker, Bulkhead, Retry Budget, Response Handler | [`patterns/08_httpclient/README.md`](patterns/08_httpclient/README.md) |
| **09** | [**Concurrency & Primitives**](patterns/09_concurrency/README.md) | Worker Pool, Fan-Out / Fan-In, Rate Limiter, Semaphore, Singleflight, ErrGroup, Backpressure Queue, Channel Ownership | [`patterns/09_concurrency/README.md`](patterns/09_concurrency/README.md) |
| **10** | [**API Design & Contracts**](patterns/10_apidesign/README.md) | Response Envelope, API Versioning, ETag Concurrency, Idempotency Keys, Pagination & Filtering, Partial Updates | [`patterns/10_apidesign/README.md`](patterns/10_apidesign/README.md) |
| **11** | [**HTTP Middleware**](patterns/11_middleware/README.md) | Middleware Chain, Rate Limiting, Timeout Middleware, Security & CORS, Access Logger, Metrics & Tracing | [`patterns/11_middleware/README.md`](patterns/11_middleware/README.md) |
| **12** | [**Authentication & AuthZ**](patterns/12_auth/README.md) | Auth Core, JWT Validator, Token Rotation, RBAC / ABAC, Ownership Check | [`patterns/12_auth/README.md`](patterns/12_auth/README.md) |
| **13** | [**Database Architecture**](patterns/13_database/README.md) | Transaction Boundaries, Read/Write Splitter, Deadlock Retry, Batch Loader, Expand/Contract Migrations | [`patterns/13_database/README.md`](patterns/13_database/README.md) |
| **14** | [**Caching Strategies**](patterns/14_caching/README.md) | Cache-Aside, TTL Invalidation, Jittered TTL, Negative Caching, Cache Stampede Prevention, Bounded LRU | [`patterns/14_caching/README.md`](patterns/14_caching/README.md) |
| **15** | [**Messaging & Event Streaming**](patterns/15_messaging/README.md) | Outbox Pattern, Inbox Pattern, Consumer Lifecycle, Dead Letter Queue (DLQ), Idempotent Consumer, Message Ordering | [`patterns/15_messaging/README.md`](patterns/15_messaging/README.md) |
| **16** | [**Distributed Systems**](patterns/16_distributed/README.md) | Distributed Lock, Saga Orchestrator, Leader Election, Reconciliation Loop, Load Shedder | [`patterns/16_distributed/README.md`](patterns/16_distributed/README.md) |
| **17** | [**Observability & Telemetry**](patterns/17_observability/README.md) | Structured Logger (slog), Metrics Collector (Prometheus), Distributed Tracer (OTel) | [`patterns/17_observability/README.md`](patterns/17_observability/README.md) |
| **18** | [**Testing & Verification**](patterns/18_testing/README.md) | Table-Driven Tests, Failure Injection, Property Invariant Testing | [`patterns/18_testing/README.md`](patterns/18_testing/README.md) |
| **19** | [**Application Security**](patterns/19_security/README.md) | Input Validation, SQL Injection Defense, SSRF Protection, CSRF Protection, Password Hashing (Argon2), Path Traversal | [`patterns/19_security/README.md`](patterns/19_security/README.md) |
| **20** | [**Reliability Engineering**](patterns/20_reliability/README.md) | Bulkhead Shield, Overload Protection, Graceful Degradation | [`patterns/20_reliability/README.md`](patterns/20_reliability/README.md) |
| **21** | [**Deployment & Draining**](patterns/21_deployment/README.md) | PreStop Draining, Traffic Splitter | [`patterns/21_deployment/README.md`](patterns/21_deployment/README.md) |
| **22** | [**Resource Management**](patterns/22_resourcemanagement/README.md) | Buffer Pool (`sync.Pool`), Resource Lifecycle | [`patterns/22_resourcemanagement/README.md`](patterns/22_resourcemanagement/README.md) |
| **23** | [**Data Integrity & Domain**](patterns/23_dataintegrity/README.md) | Domain Invariants, State Machine, Pessimistic Locking | [`patterns/23_dataintegrity/README.md`](patterns/23_dataintegrity/README.md) |
| **24** | [**Background Jobs & Crons**](patterns/24_backgroundjobs/README.md) | Job Queue, Scheduled Runner, Job Retry Worker | [`patterns/24_backgroundjobs/README.md`](patterns/24_backgroundjobs/README.md) |
| **25** | [**Code Organization**](patterns/25_codeorganization/README.md) | Layered Architecture, Domain Isolation | [`patterns/25_codeorganization/README.md`](patterns/25_codeorganization/README.md) |
| **26** | [**Advanced Production Patterns**](patterns/26_advanced/README.md) | Singleflight Cache, Multi-Bulkhead, Adaptive Shedder, Jitter Strategies, Reliable Outbox Publisher, Idempotent Payment Pipeline | [`patterns/26_advanced/README.md`](patterns/26_advanced/README.md) |

---

## 🚀 Getting Started

### Prerequisites
- **Go 1.22+** (tested up to Go 1.26+)

### Running All Tests
```bash
# Run all unit and concurrency test suites across all 26 packages
go test ./...

# Run all tests with race detector enabled
go test -race ./...
```

---

## 🌐 Digital Garden Notes & Architecture Deep Dives

Interactive mental models, detailed trade-off analysis, and system design thinking frameworks are published on:
👉 **[netologist.org](https://netologist.org)** — Digital Garden & Software Engineering Notes

---

## 📄 License

This repository is licensed under the [MIT License](LICENSE).
