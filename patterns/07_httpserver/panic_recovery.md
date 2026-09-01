# Panic Recovery and Stack Trace Capture

## Overview & Definition

In Go, an unhandled panic in an HTTP handler goroutine causes the standard library's default `http.Server` to recover the panic and log it via the server's logger, but it will drop the connection or write a minimal non-JSON message depending on server settings. If spawned background goroutines panic, they terminate the entire process.

The **Panic Recovery and Stack Trace Capture** pattern wraps HTTP handlers in a defensive `defer / recover()` middleware that:
1. Catches runtime panics (e.g., nil pointer dereferences, index out of range, explicit `panic()` calls).
2. Captures full stack traces using `runtime/debug.Stack()`.
3. Invokes an observability callback (`PanicLogger`) to send structured logs and alerts to telemetry backends (Sentry, Datadog, slog).
4. Safely returns a consistent, structured HTTP 500 JSON error response to the client without leaking sensitive internal details.

---

## Problem Statement

Unhandled runtime panics in backend web servers lead to:

* **Opaque Client Failures:** Without recovery middleware writing a standard HTTP response, clients experience abruptly terminated TCP connections (Connection Reset / EOF) or blank white pages without standard error payloads.
* **Lost Observability Data:** Default Go standard library recovery writes plain text to standard error. Without dedicated middleware, request context, Correlation IDs, client IP addresses, and structured metrics are lost.
* **Accidental Sensitive Data Leaks:** If unhandled panics output raw database connection strings, internal file paths, or private memory dumps directly to HTTP response streams, internal security postures are compromised.

---

## Architectural Mechanism & Flow

```
                      +------------------------------------------+
                      |         Incoming HTTP Request            |
                      +------------------------------------------+
                                           |
                                           v
                      +------------------------------------------+
                      |         PanicRecoveryMiddleware()        |
                      |          defer func() { ... }()          |
                      +------------------------------------------+
                                           |
                                           v
                      +------------------------------------------+
                      |           next.ServeHTTP(w, r)           |
                      |       (Downstream Handler / Business)    |
                      +------------------------------------------+
                                     /            \
                              [Normal Return]   [Runtime Panic]
                                   /                \
                                  v                  v
                   +------------------------+  +-------------------------------+
                   | Normal Response 2xx/4xx|  | recover() catches value      |
                   +------------------------+  | debug.Stack() captures trace  |
                                               +-------------------------------+
                                                               |
                                                               v
                                               +-------------------------------+
                                               | PanicLogger callback invoked  |
                                               | (Logs panic + request context)|
                                               +-------------------------------+
                                                               |
                                                               v
                                               +-------------------------------+
                                               | HTTP 500 Internal Server Error|
                                               | Content-Type: application/json|
                                               +-------------------------------+
```

### Key Mechanism Details
1. `defer func() { if rec := recover(); rec != nil { ... } }()` guarantees execution even when downstream functions panic.
2. `runtime/debug.Stack()` retrieves the current goroutine's stack trace as `[]byte` for diagnostics.
3. The response is written with `http.StatusInternalServerError` and structured JSON format.

---

## Production Best Practices & Pitfalls

### Best Practices
* **Position Near Outer Edge of Middleware Chain:** Place panic recovery immediately after Request/Correlation ID injection so that the panic logger has access to the Request ID and Correlation ID for trace correlation.
* **Sanitize Client Error Messages:** In production, return generic error identifiers (such as `"internal_server_error"` and an incident reference/Request ID) to external clients while logging the complete panic message and stack trace internally.
* **Beware of Hijacked / Committed Responses:** If downstream code already wrote headers (`w.WriteHeader`) or streamed partial body data before panicking, writing a new 500 header will result in an `http: superfluous response.WriteHeader call` warning. The recovery middleware should attempt to write headers only if the stream has not yet committed.

### Common Pitfalls
* **Recover Only Protects the Current Goroutine:** `recover()` in HTTP middleware only catches panics executing on the handler's request goroutine. If a handler launches an unmanaged background goroutine (`go doAsyncWork()`) and that goroutine panics, the entire Go process will crash. Background goroutines must always have their own internal `defer recover()` handler.
* **Recovering `http.ErrAbortHandler`:** Go's `net/http` package uses `panic(http.ErrAbortHandler)` to abort handler execution internally. While standard recovery middleware catches all panics, production systems may check `if rec == http.ErrAbortHandler` and re-panic or suppress logging to respect standard library semantics.

---

## Code Walkthrough & Usage

### 1. Middleware Implementation (`panic_recovery.go`)

```go
package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
)

// PanicLogger is a callback invoked when a panic occurs.
type PanicLogger func(r *http.Request, recovered any, stack []byte)

// PanicRecoveryMiddleware catches any panic in downstream handlers and returns 500.
func PanicRecoveryMiddleware(logger PanicLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := debug.Stack()
					if logger != nil {
						logger(r, rec, stack)
					}

					// Return standard 500 internal server error
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"error":   "internal_server_error",
						"message": fmt.Sprintf("An unexpected panic occurred: %v", rec),
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
```

### 2. Telemetry Logging Example

```go
func structuredPanicLogger(r *http.Request, rec any, stack []byte) {
    reqID := GetRequestIDFromContext(r.Context())
    slog.Error("Unhandled panic in HTTP handler",
        "request_id", reqID,
        "method", r.Method,
        "path", r.URL.Path,
        "error", fmt.Sprintf("%v", rec),
        "stack", string(stack),
    )
}

func main() {
    recovery := PanicRecoveryMiddleware(structuredPanicLogger)
    mux := http.NewServeMux()

    mux.HandleFunc("/risky", func(w http.ResponseWriter, r *http.Request) {
        var ptr *string
        *ptr = "boom" // Triggers nil pointer dereference
    })

    http.ListenAndServe(":8080", recovery(mux))
}
```
