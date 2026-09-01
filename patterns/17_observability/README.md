# 📂 Observability, Structured Logging & Tracing (`patterns/17_observability`)

> Production observability patterns implementing structured logging with log/slog, Prometheus metrics collection, and OpenTelemetry distributed tracing.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Metrics Collector (Counters, Gauges & Histograms) Pattern** | [📖 Metrics Collector (Counters, Gauges & Histograms) Pattern](./metrics_collector.md) | [`metrics_collector.go`](./metrics_collector.go) | [`metrics_collector_test.go`](./metrics_collector_test.go) |
| **Structured Logger Pattern (with Context Correlation & Redaction)** | [📖 Structured Logger Pattern (with Context Correlation & Redaction)](./structured_logger.md) | [`structured_logger.go`](./structured_logger.go) | [`structured_logger_test.go`](./structured_logger_test.go) |
| **Distributed Tracer Pattern (Spans & Context Propagation)** | [📖 Distributed Tracer Pattern (Spans & Context Propagation)](./tracer.md) | [`tracer.go`](./tracer.go) | [`tracer_test.go`](./tracer_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/17_observability/...
go test -race ./patterns/17_observability/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
