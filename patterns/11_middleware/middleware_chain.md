# Middleware Chain Pattern (Onion Architecture)

## 1. Overview & Concept

In production HTTP services built with Go, cross-cutting concerns—such as request tracing, access logging, panic recovery, authentication, rate limiting, and metrics collection—must execute in a predictable, decoupled, and composable sequence around core business logic handlers.

The **Middleware Chain** pattern (often referred to as the **Onion Architecture** or **Russian Doll / Decorator** pattern) provides an immutable, composable pipeline for chaining multiple `http.Handler` interceptors. It standardizes on Go's idiomatic middleware function signature:

```go
type Middleware func(http.Handler) http.Handler
```

By wrapping handlers recursively, each middleware in the chain executes its "pre-processing" logic on the way in (in outer-to-inner order), passes control to the next handler via `next.ServeHTTP(w, r)`, and executes its "post-processing" logic on the way out (in inner-to-outer order).

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Without a formalized, immutable middleware chain abstraction, enterprise Go web services encounter several major failure modes:

* **Deeply Nested Spaghetti Callbacks ("Pyramid of Doom"):** Manually nesting 10+ middleware layers (`m1(m2(m3(m4(m5(handler))))))`) makes route configuration unreadable, error-prone, and painful to maintain or reorder.
* **Order-of-Execution Inversion Bugs:** Certain middlewares must execute before others. For instance:
  - `PanicRecovery` must sit outside `AccessLogger` and application handlers so panicking requests still emit valid 500 status codes and access logs.
  - `CorrelationID` must precede logging and metrics so trace identifiers are attached to log records.
  - `Authentication` must precede `Authorization` and business handlers.
  Without an explicit chain construct, developers accidentally misorder handlers, leading to missing trace IDs, silent panic crashes, or unauthenticated route leakage.
* **Slice Mutation & Race Conditions:** If middleware chains are implemented via mutable global slices, concurrent route registrations or dynamic sub-router extensions can mutate shared slices during runtime, leading to data races and panics.
* **Allocations & Route Execution Overhead:** Naive middleware wrappers that perform dynamic heap allocations or interface conversions on every request degrade high-throughput (100k+ RPS) API gateways.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

The middleware chain follows the Onion Architecture model. Handlers are evaluated in reverse order during construction (`Then`), so the first declared middleware wraps the outermost layer of the HTTP execution stack.

```
                  Incoming HTTP Request
                            │
                            ▼
              ┌───────────────────────────┐
              │  Middleware 1 (Outer)     │ ◄── Pre-processing (e.g., Trace ID, Panic Recovery)
              │  ┌─────────────────────┐  │
              │  │ Middleware 2        │  │ ◄── Pre-processing (e.g., Rate Limiter, Auth)
              │  │  ┌───────────────┐  │  │
              │  │  │ Core Handler  │  │  │ ◄── Business Logic (r.Context, DB query)
              │  │  └───────────────┘  │  │
              │  │        │            │  │
              │  │        ▼            │  │
              │  │   Post-Process      │  │ ◄── Post-processing (e.g., Auth token refresh)
              │  └─────────────────────┘  │
              │           │               │
              │           ▼               │
              │      Post-Process         │ ◄── Post-processing (e.g., Access Log, Latency Metric)
              └───────────────────────────┘
                            │
                            ▼
                  Outbound HTTP Response
```

### Chain Execution Flow

```text
1. Client sends request -> Middleware 1 (enters)
2. Middleware 1 calls next.ServeHTTP() -> Middleware 2 (enters)
3. Middleware 2 calls next.ServeHTTP() -> Core Business Handler
4. Core Business Handler writes response and returns
5. Middleware 2 receives control, runs defer/post-logic, and returns
6. Middleware 1 receives control, records total latency, and completes
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### 1. Immutability & Copy-on-Append
The `Chain` struct must be value-oriented and copy its underlying slice when appending new middlewares via `Append()`. This guarantees that extending a base chain for sub-routers or specific endpoint groups (e.g., `adminChain = baseChain.Append(AdminOnlyMiddleware)`) does not mutate or corrupt the `baseChain`.

### 2. Nil Handler Safety
In `Then(finalHandler http.Handler)`, if `finalHandler` is passed as `nil`, the chain should gracefully fall back to `http.DefaultServeMux` to avoid nil pointer dereference panics when `ServeHTTP` is invoked.

### 3. ResponseWriter Sniffing and Hijacking
Middlewares that wrap `http.ResponseWriter` (e.g., to record status codes or bytes written) must be careful not to obscure optional HTTP interfaces like `http.Flusher`, `http.Hijacker`, or `http.Pusher`. If WebSockets or Server-Sent Events (SSE) are used, wrappers should implement these interfaces conditionally or delegate them properly.

### 4. Memory Allocations
The construction of `Chain.Then` happens once during application startup / routing initialization. Zero runtime allocation overhead occurs during request servicing beyond the standard closure call stack.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### 1. Core Chain Implementation (`patterns/11_middleware/middleware_chain.go`)

```go
package middleware

import "net/http"

// Middleware defines standard HTTP middleware signature.
type Middleware func(http.Handler) http.Handler

// Chain coordinates sequential execution of multiple middlewares (Onion Architecture).
type Chain struct {
	middlewares []Middleware
}

// NewChain creates an empty middleware chain with defensive slice copying.
func NewChain(middlewares ...Middleware) Chain {
	return Chain{
		middlewares: append(([]Middleware)(nil), middlewares...),
	}
}

// Append creates a new chain with added middlewares without mutating the original.
func (c Chain) Append(middlewares ...Middleware) Chain {
	newMiddlewares := make([]Middleware, 0, len(c.middlewares)+len(middlewares))
	newMiddlewares = append(newMiddlewares, c.middlewares...)
	newMiddlewares = append(newMiddlewares, middlewares...)
	return Chain{middlewares: newMiddlewares}
}

// Then wraps the final http.Handler with all middlewares in outer-to-inner order.
func (c Chain) Then(finalHandler http.Handler) http.Handler {
	if finalHandler == nil {
		finalHandler = http.DefaultServeMux
	}

	for i := len(c.middlewares) - 1; i >= 0; i-- {
		finalHandler = c.middlewares[i](finalHandler)
	}

	return finalHandler
}
```

### 2. Composition Example in Production Service

```go
func SetupRouter() http.Handler {
    // 1. Define global perimeter middlewares
    standardChain := middleware.NewChain(
        RecoveryMiddleware,
        CorrelationIDMiddleware,
        AccessLoggerMiddleware(logger),
        MetricsMiddleware(metrics),
    )

    // 2. Define protected sub-chain with authentication and rate limiting
    protectedChain := standardChain.Append(
        RateLimitMiddleware,
        AuthMiddleware,
    )

    mux := http.NewServeMux()
    
    // Public health probe endpoint
    mux.Handle("/healthz", standardChain.Then(http.HandlerFunc(HealthHandler)))

    // Protected API endpoint
    mux.Handle("/api/v1/orders", protectedChain.Then(http.HandlerFunc(CreateOrderHandler)))

    return mux
}
```
