# 📂 Messaging, Outbox/Inbox & Event Streaming (`patterns/15_messaging`)

> Reliable event-driven messaging patterns including transactional outbox, deduplicating inbox, dead letter queues (DLQ), and partition message ordering.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Consumer Lifecycle & Graceful Drain Pattern** | [📖 Consumer Lifecycle & Graceful Drain Pattern](./consumer_lifecycle.md) | [`consumer_lifecycle.go`](./consumer_lifecycle.go) | [`consumer_lifecycle_test.go`](./consumer_lifecycle_test.go) |
| **Dead-Letter Queue (DLQ) & Poison Pill Isolation Pattern** | [📖 Dead-Letter Queue (DLQ) & Poison Pill Isolation Pattern](./dead_letter_queue.md) | [`dead_letter_queue.go`](./dead_letter_queue.go) | [`dead_letter_queue_test.go`](./dead_letter_queue_test.go) |
| **Idempotent Consumer Pattern** | [📖 Idempotent Consumer Pattern](./idempotent_consumer.md) | [`idempotent_consumer.go`](./idempotent_consumer.go) | [`idempotent_consumer_test.go`](./idempotent_consumer_test.go) |
| **Transactional Inbox Pattern** | [📖 Transactional Inbox Pattern](./inbox_pattern.md) | [`inbox_pattern.go`](./inbox_pattern.go) | [`inbox_pattern_test.go`](./inbox_pattern_test.go) |
| **Partitioned Message Ordering Pattern** | [📖 Partitioned Message Ordering Pattern](./message_ordering.md) | [`message_ordering.go`](./message_ordering.go) | [`message_ordering_test.go`](./message_ordering_test.go) |
| **Transactional Outbox Pattern** | [📖 Transactional Outbox Pattern](./outbox_pattern.md) | [`outbox_pattern.go`](./outbox_pattern.go) | [`outbox_pattern_test.go`](./outbox_pattern_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/15_messaging/...
go test -race ./patterns/15_messaging/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
