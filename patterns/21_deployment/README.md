# 📂 Deployment, Draining & Traffic Splitting (`patterns/21_deployment`)

> Zero-downtime deployment patterns including Kubernetes PreStop hook connection draining and canary/blue-green traffic splitting.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Kubernetes PreStop Draining & Zero-Downtime Termination Pattern** | [📖 Kubernetes PreStop Draining & Zero-Downtime Termination Pattern](./prestop_draining.md) | [`prestop_draining.go`](./prestop_draining.go) | [`prestop_draining_test.go`](./prestop_draining_test.go) |
| **Dynamic Traffic Splitter Pattern (Canary & Blue/Green Router)** | [📖 Dynamic Traffic Splitter Pattern (Canary & Blue/Green Router)](./traffic_splitter.md) | [`traffic_splitter.go`](./traffic_splitter.go) | [`traffic_splitter_test.go`](./traffic_splitter_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/21_deployment/...
go test -race ./patterns/21_deployment/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
