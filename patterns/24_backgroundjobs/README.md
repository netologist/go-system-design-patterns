# 📂 Background Jobs, Queues & Schedulers (`patterns/24_backgroundjobs`)

> Asynchronous job processing patterns including worker job queues, cron-style recurring schedulers, and resilient retry workers with backoff.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Visibility Job Queue Pattern (Lease-Based Asynchronous Worker Queue)** | [📖 Visibility Job Queue Pattern (Lease-Based Asynchronous Worker Queue)](./job_queue.md) | [`job_queue.go`](./job_queue.go) | [`job_queue_test.go`](./job_queue_test.go) |
| **Job Retry & Quarantine Pattern (Exponential Backoff with Dead-Letter Handling)** | [📖 Job Retry & Quarantine Pattern (Exponential Backoff with Dead-Letter Handling)](./job_retry.md) | [`job_retry.go`](./job_retry.go) | [`job_retry_test.go`](./job_retry_test.go) |
| **Scheduled Task Runner Pattern (Periodic Background Ticker Loop)** | [📖 Scheduled Task Runner Pattern (Periodic Background Ticker Loop)](./scheduled_runner.md) | [`scheduled_runner.go`](./scheduled_runner.go) | [`scheduled_runner_test.go`](./scheduled_runner_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/24_backgroundjobs/...
go test -race ./patterns/24_backgroundjobs/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
