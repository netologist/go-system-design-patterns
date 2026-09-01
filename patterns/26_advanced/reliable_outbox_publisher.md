# Transactional Outbox with Reliable Relay Publisher

## 1. Overview & Concept
The Transactional Outbox pattern solves the **dual-write problem** in distributed event-driven systems: ensuring that state changes in the local database and corresponding domain events published to a message broker (e.g. Kafka, RabbitMQ, SQS) either **both succeed or both fail together atomically**.

A background **Reliable Outbox Publisher** continuously polls pending outbox records from the database, publishes them to the message broker with backoff retries, and marks them as `PUBLISHED` upon successful delivery.

## 2. Production Problem & Failure Modes
Without Transactional Outbox:
1. **Lost Events (DB Commit succeeds, Broker Publish fails)**: Order is inserted into DB, but Kafka is unreachable. The event is lost forever; downstream inventory and shipping services are never notified.
2. **Ghost Events (Broker Publish succeeds, DB Commit fails)**: Event is sent to Kafka, but database transaction aborts due to constraint violation. Downstream services process an event for an order that does not exist in the database.

## 3. Architecture & Mechanism

```text
HTTP Request ---> [ Database Transaction (Atomic) ]
                        |
                        +---> 1. INSERT orders (Business Data)
                        +---> 2. INSERT outbox_events (Status: PENDING)
                        |
                        v COMMIT

                  [ Background Relay Publisher ]
                        |
                        +---> Poll PENDING outbox_events
                        +---> Publish to Kafka / RabbitMQ
                        +---> UPDATE outbox_events SET Status = 'PUBLISHED'
```

## 4. Production Hardening & Trade-offs
- **At-Least-Once Delivery**: The publisher guarantees at-least-once delivery; consumers must be idempotent (e.g. using Inbox pattern or deduplication keys).
- **Batch Processing**: Poll and publish outbox records in bounded batches (e.g. 50-100 events per poll) to minimize database round-trips.
- **Cleanup & Archival**: Periodically purge or archive published events older than N days to prevent unbounded outbox table growth.

## 5. Code Walkthrough & Usage
See `reliable_outbox_publisher.go` and `reliable_outbox_publisher_test.go`:
- `ReliableOutboxPublisher`: Asynchronously polls `TransactionalOutboxStore` and dispatches to `BrokerMessageProducer`.
