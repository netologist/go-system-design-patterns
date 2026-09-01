# Request and Correlation ID Propagation

## Overview & Definition

In distributed microservice architectures, a single user transaction often traverses multiple backend services, databases, and message brokers. The **Request and Correlation ID Propagation** pattern manages two distinct identifiers:
1. **Request ID (`X-Request-ID`):** A unique identifier assigned to a single HTTP request entering a specific service node.
2. **Correlation ID (`X-Correlation-ID`):** A distributed trace identifier that is generated at the system edge (or API Gateway) and propagated across every downstream HTTP request, RPC call, and queue message.

This pattern extracts incoming IDs from headers (or generates secure 16-byte random hex IDs if missing), injects them into the Go `context.Context`, sets them on outbound response headers, and provides accessor functions for downstream loggers and clients.

---

## Problem Statement

Without standardized request and correlation tracking:

* **Impossible Root Cause Analysis in Microservices:** When an error occurs deep within a service call chain (e.g. Service A -> Service B -> Service C), logs across systems cannot be joined without a shared correlation key.
* **Customer Support Friction:** When an end-user encounters an error, there is no shared reference ID to locate the exact request trace across server logs.
* **Log Aggregation Fragmentation:** In Elasticsearch, Loki, or Datadog, log lines from different stages of the same transaction appear disjointed, making distributed debugging time-consuming.

---

## Architectural Mechanism & Flow

```
                      Client / Edge Gateway
                      [Headers: Optional X-Correlation-ID]
                                 |
                                 v
                     +---------------------------------------+
                     | RequestAndCorrelationIDMiddleware     |
                     +---------------------------------------+
                                 |
        +------------------------+-------------------------+
        |                                                  |
        v                                                  v
 [X-Request-ID Present?]                        [X-Correlation-ID Present?]
   /              \                               /                  \
 [Yes]            [No]                          [Yes]                [No]
   |                |                             |                    |
 Preserve      GenerateID()                   Preserve           Use Request ID
   |                |                             |                    |
   +--------+-------+                             +---------+----------+
            |                                               |
            +-----------------------+-----------------------+
                                    |
                                    v
            +-----------------------------------------------+
            | 1. Inject into request context.Context        |
            | 2. Set Response Headers:                      |
            |    - X-Request-ID                             |
            |    - X-Correlation-ID                         |
            +-----------------------------------------------+
                                    |
                                    v
            +-----------------------------------------------+
            | Next HTTP Handler / Outbound HTTP Calls       |
            | (Extracts IDs for Slog & Upstream Headers)   |
            +-----------------------------------------------+
```

### Context Key Safety
To prevent context key collisions across packages, the pattern defines unexported struct types as context keys:
```go
type (
    requestIDCtxKey     struct{}
    correlationIDCtxKey struct{}
)
```

---

## Production Best Practices & Pitfalls

### Best Practices
* **Use Unexported Types for Context Keys:** Never use plain string constants (e.g., `"request_id"`) as context keys, as third-party packages might overwrite them.
* **Propagate on Outbound Client Calls:** When making outbound HTTP requests with `http.Client`, always copy the Correlation ID from the request context to the outbound request's `X-Correlation-ID` header.
* **Attach to Structured Logger:** Use a logging middleware or slog handler that automatically extracts `GetCorrelationIDFromContext(ctx)` and appends it to every log record.

### Common Pitfalls
* **Trusting Unsanitized Client Headers for Security Decisions:** While accepting client-provided correlation IDs is standard for tracing, do not use `X-Request-ID` or `X-Correlation-ID` for authentication, authorization, or idempotency keys.
* **Using Blocking or Slow Random Generators:** Ensure the ID generation routine uses non-blocking cryptographically secure random sources (`crypto/rand`) or UUIDv4/UUIDv7 generators that do not cause lock contention under high concurrency.

---

## Code Walkthrough & Usage

### 1. Implementation (`request_correlation_id.go`)

```go
package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

type (
	requestIDCtxKey     struct{}
	correlationIDCtxKey struct{}
)

const (
	HeaderRequestID     = "X-Request-ID"
	HeaderCorrelationID = "X-Correlation-ID"
)

// GenerateID produces a random 16-byte hex identifier.
func GenerateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RequestAndCorrelationIDMiddleware extracts or generates Request ID and Correlation ID.
func RequestAndCorrelationIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Request ID (unique to this HTTP request)
		reqID := strings.TrimSpace(r.Header.Get(HeaderRequestID))
		if reqID == "" {
			reqID = GenerateID()
		}

		// 2. Correlation ID (propagated across distributed services)
		corrID := strings.TrimSpace(r.Header.Get(HeaderCorrelationID))
		if corrID == "" {
			corrID = reqID // Default to request ID if first service in chain
		}

		// Inject into context
		ctx := context.WithValue(r.Context(), requestIDCtxKey{}, reqID)
		ctx = context.WithValue(ctx, correlationIDCtxKey{}, corrID)

		// Set response headers
		w.Header().Set(HeaderRequestID, reqID)
		w.Header().Set(HeaderCorrelationID, corrID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestIDFromContext extracts request ID from request context.
func GetRequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// GetCorrelationIDFromContext extracts correlation ID from request context.
func GetCorrelationIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(correlationIDCtxKey{}).(string); ok {
		return v
	}
	return ""
}
```

### 2. Outbound Propagation Example

```go
func ForwardRequest(ctx context.Context, targetURL string) (*http.Response, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
    if err != nil {
        return nil, err
    }

    // Propagate correlation trace to downstream service
    if corrID := GetCorrelationIDFromContext(ctx); corrID != "" {
        req.Header.Set(HeaderCorrelationID, corrID)
    }

    return http.DefaultClient.Do(req)
}
```
