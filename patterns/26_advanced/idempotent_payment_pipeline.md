# End-to-End Idempotent Payment Pipeline Pattern

## 1. Overview & Concept
Payment and checkout systems require bulletproof correctness. In distributed systems, network responses can be dropped even when the payment succeeded on the payment gateway (e.g. Stripe).

The End-to-End Idempotent Payment Pipeline orchestrates:
1. **Idempotency Key Verification**: Check if the operation was already processed; if so, return the cached result.
2. **State Machine Transition**: Move from `PENDING` -> `PROCESSING` -> `SUCCEEDED`.
3. **External Gateway Call Outside DB Transaction**: Execute third-party credit card charge *without* holding a database connection lock.
4. **Atomic DB Finalization & Response Caching**: Store payment records and cache the response under the Idempotency Key for safe replays.

## 2. Production Problem & Failure Modes
1. **Double Charging Customers**: Client sends charge request, payment succeeds on Stripe, but network drops before response reaches client. Client retries, executing a duplicate charge.
2. **Connection Pool Starvation via Network Calls in DB Transactions**: Holding a DB transaction open while waiting 2000ms for a payment gateway HTTP call exhausts the database connection pool.
3. **Invalid State Leaps**: Concurrent race conditions transitioning an already-refunded payment back to processing.

## 3. Architecture & Mechanism

```text
Client Request (Idempotency-Key: "idemp_123")
          |
          v
   Cached Response? ---> YES ---> Return Cached Result (No Double Charge)
          |
          v NO
   State Machine: Transition PENDING -> PROCESSING
          |
          v
   CALL EXTERNAL GATEWAY (Stripe HTTP API) [OUTSIDE DB TRANSACTION]
          |
          v
   State Machine: Transition PROCESSING -> SUCCEEDED
          |
          v
   Save Final Payment & Cache Response under Idempotency-Key
          |
          v
   Return 201 Created Response to Client
```

## 4. Production Hardening & Trade-offs
- **In-Flight Key Locking**: If a duplicate request arrives *while the first is still processing*, reject with a concurrent in-flight conflict rather than executing two parallel charges.
- **Audit Trails**: Log every state transition with transaction IDs and timestamps for financial reconciliation.
- **DB Isolation**: DB transactions must only span local database writes, never remote third-party network I/O.

## 5. Code Walkthrough & Usage
See `idempotent_payment_pipeline.go` and `idempotent_payment_pipeline_test.go`:
- `IdempotentPaymentPipeline`: Manages idempotency cache, in-flight locks, and payment execution.
- Tests prove duplicate requests with identical `IdempotencyKey` never trigger a second gateway charge.
