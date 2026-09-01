# 🏛️ Production System Design & Backend Architecture Patterns in Go

A comprehensive, production-grade reference implementation of **118 backend design patterns, concurrency models, and distributed systems primitives** in idiomatic Go (Go 1.22+ / 1.26+).

Every pattern includes:
- 📖 **Architectural Specification & Failure Mode Analysis** (`.md`)
- 💻 **Thread-Safe, Production-Hardened Go Implementation** (`.go`)
- 🧪 **Comprehensive Unit & Concurrency Test Suites** (`_test.go`)

Created and maintained by **Hasan Özgan** ([netologist.org](https://netologist.org) · [@netologist](https://github.com/netologist)).

---

## 🧭 Table of Contents

| # | Category | Core Patterns | Package Path |
|---|---|---|---|
| **01** | [Application Lifecycle](#01-lifecycle) | Graceful Shutdown, Graceful Startup, Signal Handling, Health Probes, Composition Root | `patterns/01_lifecycle` |
| **02** | [Configuration](#02-config) | Typed Config, Immutable Config, Config Validator, Secret Sanitizer | `patterns/02_config` |
| **03** | [Dependency Injection](#03-di) | Constructor Injection, Interface at Consumer, Functional Options, Test Doubles | `patterns/03_di` |
| **04** | [Persistence & ORM](#04-persistence) | Repository Pattern, Unit of Work, Keyset Pagination, Optimistic Locking, Soft Delete, Connection Pool Limiter | `patterns/04_persistence` |
| **05** | [Context & Cancellation](#05-context) | Context Propagation, Scoped Timeout, Cancellation Pipeline, Typed Context Values | `patterns/05_context` |
| **06** | [Error Handling](#06-errors) | Typed Errors, Sentinel Wrapping, Error Mapping, Structured Response | `patterns/06_errors` |
| **07** | [HTTP Server](#07-httpserver) | Hardened Server, Request Correlation ID, Strict JSON Decoder, Panic Recovery, Body Limit | `patterns/07_httpserver` |
| **08** | [HTTP Client & Resilience](#08-httpclient) | Pooled Client, Retry Backoff, Circuit Breaker, Bulkhead, Retry Budget, Response Handler | `patterns/08_httpclient` |
| **09** | [Concurrency & Channels](#09-concurrency) | Worker Pool, Fan-Out / Fan-In, Rate Limiter, Semaphore, Singleflight, ErrGroup, Backpressure Queue, Channel Ownership | `patterns/09_concurrency` |
| **10** | [API Design](#10-apidesign) | Response Envelope, API Versioning, ETag Concurrency, Idempotency Keys, Pagination & Filtering, Partial Updates | `patterns/10_apidesign` |
| **11** | [HTTP Middleware](#11-middleware) | Middleware Chain, Rate Limiting, Timeout Middleware, Security & CORS, Access Logger, Metrics & Tracing | `patterns/11_middleware` |
| **12** | [Authentication & Security](#12-auth) | Auth Core, JWT Validator, Token Rotation, RBAC / ABAC, Ownership Check | `patterns/12_auth` |
| **13** | [Database Architecture](#13-database) | Transaction Boundaries, Read/Write Splitter, Deadlock Retry, Batch Loader, Expand/Contract Migrations | `patterns/13_database` |
| **14** | [Caching Strategies](#14-caching) | Cache-Aside, TTL Invalidation, Jittered TTL, Negative Caching, Cache Stampede Prevention, Bounded LRU | `patterns/14_caching` |
| **15** | [Messaging & Event-Driven](#15-messaging) | Outbox Pattern, Inbox Pattern, Consumer Lifecycle, Dead Letter Queue (DLQ), Idempotent Consumer, Message Ordering | `patterns/15_messaging` |
| **16** | [Distributed Systems](#16-distributed) | Distributed Lock, Saga Orchestrator, Leader Election, Reconciliation Loop, Load Shedder | `patterns/16_distributed` |
| **17** | [Observability](#17-observability) | Structured Logger (slog), Metrics Collector (Prometheus), Distributed Tracer (OTel) | `patterns/17_observability` |
| **18** | [Testing Patterns](#18-testing) | Table-Driven Tests, Failure Injection, Property Invariant Testing | `patterns/18_testing` |
| **19** | [Application Security](#19-security) | Input Validation, SQL Injection Defense, SSRF Protection, CSRF Protection, Password Hashing (Argon2), Path Traversal | `patterns/19_security` |
| **20** | [Reliability Engineering](#20-reliability) | Bulkhead Shield, Overload Protection, Graceful Degradation | `patterns/20_reliability` |
| **21** | [Deployment & Orchestration](#21-deployment) | PreStop Draining, Traffic Splitter | `patterns/21_deployment` |
| **22** | [Resource Management](#22-resourcemanagement) | Buffer Pool (`sync.Pool`), Resource Lifecycle | `patterns/22_resourcemanagement` |
| **23** | [Data Integrity & Domain](#23-dataintegrity) | Domain Invariants, State Machine, Pessimistic Locking | `patterns/23_dataintegrity` |
| **24** | [Background Jobs](#24-backgroundjobs) | Job Queue, Scheduled Runner, Job Retry Worker | `patterns/24_backgroundjobs` |
| **25** | [Code Organization](#25-codeorganization) | Layered Architecture, Domain Isolation | `patterns/25_codeorganization` |
| **26** | [Advanced Patterns](#26-advanced) | Singleflight Cache, Multi-Bulkhead, Adaptive Shedder, Jitter Strategies, Reliable Outbox Publisher, Idempotent Payment Pipeline | `patterns/26_advanced` |

---

## 🚀 Getting Started

### Prerequisites
- **Go 1.22+** (tested up to Go 1.26+)

### Running All Tests
```bash
# Run all unit and concurrency test suites
go test ./...

# Run tests with race detection enabled
go test -race ./...
```

---

## 🌐 Digital Garden Notes & Interactive Architecture

Deep-dive explanations, architectural diagrams, and mental models for these patterns are published on:
👉 **[netologist.org](https://netologist.org)** — Digital Garden & Software Engineering Notes

---

## 📄 License

This repository is licensed under the [MIT License](LICENSE).
