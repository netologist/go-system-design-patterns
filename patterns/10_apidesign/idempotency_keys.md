# Idempotency Key Handling and Replay Caching

## Overview & Definition

In distributed HTTP architectures, network drops and client retries can cause non-idempotent mutation requests (e.g. `POST /api/v1/payments`, `POST /api/v1/orders`) to be received multiple times. Without idempotency controls, a payment retry might charge a customer twice or create duplicate database orders.

The **Idempotency Key Handling and Replay Caching** pattern (standardized by Stripe and IETF draft `draft-ietf-httpapi-idempotency-key-header`) provides:
1. **Client-Supplied Key:** The client transmits a unique identifier via the `Idempotency-Key` header.
2. **In-Flight Distributed Locking (`LockKey`):** Atomically locks the key to prevent concurrent duplicate execution while the first request is being processed.
3. **Response Caching (`SaveResponse`):** Caches the HTTP status code, response headers, and response body alongside a cryptographic SHA-256 hash of the request payload (`PayloadHash`).
4. **Replay Mechanism:** Subsequent requests bearing the same key immediately return the exact cached response without executing business logic or triggering side effects.

---

## Problem Statement

Without idempotency management, distributed systems suffer from severe data inconsistency:

* **Double Billing & Duplicate Orders:** If a client submits a payment request and the TCP connection drops during the response write, the client retries. The backend processes a second transaction, double-charging the user.
* **Race Conditions on Concurrent Retries:** If a mobile client fires two concurrent network requests with identical parameters due to a UI debounce bug, both requests execute simultaneously in parallel database transactions.
* **Payload Mutation with Same Key (Tampering):** If a client submits a $10 payment with key `key-123`, and later sends a $10,000 payment with the same key `key-123`, failing to verify payload hashes could result in fraudulent replaying or mismatched accounting.

---

## Architectural Mechanism & Flow

```
                     Incoming HTTP POST /payments
                     [Header: Idempotency-Key: "idemp-001"]
                                    |
                                    v
                     +-------------------------------+
                     |   store.LockKey(key)          |
                     +-------------------------------+
                                    |
            +-----------------------+-----------------------+
            |                                               |
            v                                               v
    [Cached Response Exists?]                        [Key Is Locked?]
            |                                               |
         [Yes]                                           [Yes]
            |                                               |
            v                                               v
    +-------------------------+                     +-------------------------------+
    | 1. Compare Payload Hash |                     | Return HTTP 409 Conflict /    |
    | 2. Replay Cached Status |                     | ErrConcurrentIdempotentOp     |
    |    Headers & Body       |                     +-------------------------------+
    +-------------------------+
            |
            v [No (New Key)]
    +-------------------------------+
    | 1. Acquire In-Flight Lock     |
    | 2. Execute Payment Business   |
    | 3. store.SaveResponse(...)    |
    | 4. Release Lock               |
    | 5. Return HTTP 201 Created    |
    +-------------------------------+
```

---

## Production Best Practices & Pitfalls

### Best Practices
* **Hash Request Payloads:** Always compute `sha256(request_body)` and compare with `cached.PayloadHash`. If the key matches but the payload differs, reject the request with HTTP 422 Unprocessable Entity or HTTP 400 Bad Request to prevent key reuse attacks.
* **Set TTL Expirations on Idempotency Records:** Expire idempotency keys and cached responses after a reasonable window (e.g. 24 to 72 hours) in Redis or DynamoDB.
* **Store Complete Response State:** Cache the HTTP status code, headers (especially `Content-Type`), and body so the replayed response is identical to the original response.

### Common Pitfalls
* **Releasing Locks Before Committing Database Transactions:** Releasing the idempotency lock before the primary transactional commit finishes allows a concurrent retry to slip through and execute duplicate database operations.
* **In-Memory Store in Distributed Clusters:** `MemoryIdempotencyStore` is suitable for single-node instances and unit tests. In multi-node production clusters, back the store with Redis (using `SET key NX EX`) or PostgreSQL (`INSERT ... ON CONFLICT`).

---

## Code Walkthrough & Usage

### 1. Implementation (`idempotency_keys.go`)

```go
package apidesign

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
	"time"
)

var (
	ErrMissingIdempotencyKey  = errors.New("missing Idempotency-Key header")
	ErrConcurrentIdempotentOp = errors.New("concurrent operation with same Idempotency-Key already in flight")
)

const HeaderIdempotencyKey = "Idempotency-Key"

// CachedResponse stores the recorded status, body, and hash of the original request.
type CachedResponse struct {
	StatusCode  int
	Header      http.Header
	Body        []byte
	PayloadHash string
	CreatedAt   time.Time
}

// MemoryIdempotencyStore stores idempotency keys and their cached responses.
type MemoryIdempotencyStore struct {
	mu    sync.RWMutex
	cache map[string]*CachedResponse
	locks map[string]struct{}
}

func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{
		cache: make(map[string]*CachedResponse),
		locks: make(map[string]struct{}),
	}
}

// LockKey attempts to lock a key for an in-flight operation.
func (s *MemoryIdempotencyStore) LockKey(key string) (*CachedResponse, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already completed and cached
	if cached, ok := s.cache[key]; ok {
		return cached, false
	}

	// Check if currently locked
	if _, locked := s.locks[key]; locked {
		return nil, false
	}

	s.locks[key] = struct{}{}
	return nil, true
}

// SaveResponse caches the completed response and releases the lock.
func (s *MemoryIdempotencyStore) SaveResponse(key, payloadHash string, status int, header http.Header, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.locks, key)
	s.cache[key] = &CachedResponse{
		StatusCode:  status,
		Header:      header.Clone(),
		Body:        body,
		PayloadHash: payloadHash,
		CreatedAt:   time.Now(),
	}
}

func HashPayload(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}
```

### 2. HTTP Middleware / Handler Usage

```go
func IdempotentPaymentHandler(store *MemoryIdempotencyStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        key := r.Header.Get(HeaderIdempotencyKey)
        if key == "" {
            http.Error(w, ErrMissingIdempotencyKey.Error(), http.StatusBadRequest)
            return
        }

        body, _ := io.ReadAll(r.Body)
        payloadHash := HashPayload(body)

        cached, acquired := store.LockKey(key)
        if !acquired {
            if cached != nil {
                // Replay cached response
                for k, v := range cached.Header {
                    w.Header()[k] = v
                }
                w.WriteHeader(cached.StatusCode)
                w.Write(cached.Body)
                return
            }
            // Concurrent execution in progress
            http.Error(w, ErrConcurrentIdempotentOp.Error(), http.StatusConflict)
            return
        }

        // Process transaction
        respBody := []byte(`{"status":"success","charge_id":"ch_123"}`)
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        w.Write(respBody)

        // Save for replay
        store.SaveResponse(key, payloadHash, http.StatusCreated, w.Header(), respBody)
    }
}
```
