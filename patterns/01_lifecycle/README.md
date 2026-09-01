# 📂 Application Lifecycle & Process Management (`patterns/01_lifecycle`)

> Patterns governing application startup sequence, signal handling, graceful teardown, and Kubernetes health probes (liveness, readiness, startup).

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Composition Root Pattern** | [📖 Composition Root Pattern](./composition_root.md) | [`composition_root.go`](./composition_root.go) | [`composition_root_test.go`](./composition_root_test.go) |
| **Graceful Shutdown Pattern** | [📖 Graceful Shutdown Pattern](./graceful_shutdown.md) | [`graceful_shutdown.go`](./graceful_shutdown.go) | [`graceful_shutdown_test.go`](./graceful_shutdown_test.go) |
| **Graceful Startup Pattern** | [📖 Graceful Startup Pattern](./graceful_startup.md) | [`graceful_startup.go`](./graceful_startup.go) | [`graceful_startup_test.go`](./graceful_startup_test.go) |
| **Health Probes Pattern (Liveness, Readiness, Startup)** | [📖 Health Probes Pattern (Liveness, Readiness, Startup)](./health_probes.md) | [`health_probes.go`](./health_probes.go) | [`health_probes_test.go`](./health_probes_test.go) |
| **Signal Handling Pattern** | [📖 Signal Handling Pattern](./signal_handling.md) | [`signal_handling.go`](./signal_handling.go) | [`signal_handling_test.go`](./signal_handling_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/01_lifecycle/...
go test -race ./patterns/01_lifecycle/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
