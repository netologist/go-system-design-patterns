# Structured Access Logger Middleware

## 1. Overview & Concept

Observability is the foundation of high-availability backend systems. An **Access Logger Middleware** captures structured runtime telemetry for every inbound HTTP transaction, logging essential metadata such as the HTTP method, request path, response status code, payload byte size, client IP / remote address, user agent, and end-to-end execution duration.

In Go's standard library `net/http`, the `http.ResponseWriter` interface does not expose getters to read back the status code or the number of bytes written once emitted to the wire. The **Access Logger Middleware** solves this by wrapping `http.ResponseWriter` in a custom interceptor struct (`responseWriterWrapper`), recording status codes and byte counters transparently as the downstream handler executes, and executing a non-blocking `LogWriter` callback upon completion.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Without a standardized, structured access logging middleware:

* **Blind Spots During Outages (Zero Telemetry):** If a service returns HTTP 500 or 504 errors without logging status codes and execution latencies, SRE and on-call engineers cannot diagnose whether errors stem from specific endpoints, database query degradation, or upstream gateway timeouts.
* **Missing Payload Size Tracking:** Without byte accounting, runaway endpoints streaming multi-megabyte payloads cannot be identified, leading to unexplained bandwidth exhaustion and egress cost spikes.
* **Double Status Header Panic:** In Go, calling `w.WriteHeader(code)` multiple times emits a `http: superfluous response.WriteHeader call` warning and ignores subsequent status codes. A naive wrapper without a `wroteHeader` boolean flag risks corrupting status tracking.
* **Default 200 OK Fallback Blindspot:** Handlers that write data directly via `w.Write([]byte)` without an explicit call to `w.WriteHeader(...)` implicitly send HTTP 200 OK. If the wrapper fails to catch implicit 200s, it records an uninitialized status code (`0`), skewing metrics and alert dashboards.
* **High-Throughput Log Lock Contention:** Naive string formatting (`fmt.Sprintf`) or synchronous file I/O inside the request hot path causes severe lock contention under high concurrency (50k+ RPS), turning logging into the primary latency bottleneck.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

The middleware wraps `http.ResponseWriter` before delegating to the downstream handler stack, intercepting all header and body writes.

```
                      Client HTTP Request
                               │
                               ▼
            ┌──────────────────────────────────────┐
            │       AccessLoggerMiddleware         │
            │  1. Record start = time.Now()        │
            │  2. Wrap w -> responseWriterWrapper  │
            └──────────────────┬───────────────────┘
                               │
                               ▼
            ┌──────────────────────────────────────┐
            │          Downstream Handler          │
            │  - Calls w.WriteHeader(code)        │ ──► Captured by wrapper
            │  - Calls w.Write(payload)            │ ──► Bytes counted by wrapper
            └──────────────────┬───────────────────┘
                               │
                               ▼
            ┌──────────────────────────────────────┐
            │       AccessLoggerMiddleware         │
            │  3. Calculate duration = Since(start)│
            │  4. Emit structured AccessLogEntry   │
            └──────────────────┬───────────────────┘
                               │
                               ▼
                    Structured Log Stream (slog / JSON)
```

### Component Details

```
   responseWriterWrapper
   ┌────────────────────────────────────────────────────────┐
   │ http.ResponseWriter  (Delegated underlying network conn)│
   │ statusCode   : int   (Captured HTTP response code)     │
   │ bytesWritten : int64 (Accumulated byte counter)        │
   │ wroteHeader  : bool  (Guard against duplicate headers) │
   └────────────────────────────────────────────────────────┘
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### 1. Handling Implicit HTTP 200 Status Codes
If a handler writes to the response body without calling `WriteHeader`, `http.ResponseWriter` defaults to HTTP 200. The wrapper mirrors this behavior in `Write()`:
```go
if !w.wroteHeader {
    w.WriteHeader(http.StatusOK)
}
```

### 2. Guarding Against Superfluous Header Invocations
The wrapper tracks `wroteHeader bool` so that only the first `WriteHeader` call sets the status and delegates to the underlying writer, preventing race conditions or log corruption from bad handler implementations.

### 3. Decoupled Log Sinks (`LogWriter` Callback)
Rather than hardcoding standard library `log` or a specific third-party logger, the middleware accepts a generic `LogWriter` function:
```go
type LogWriter func(entry AccessLogEntry)
```
This enables zero-allocation routing to `log/slog`, OpenTelemetry event collectors, or asynchronous ring-buffered log pipelines.

### 4. Sanitizing Request Paths & Sensitive Headers
Never log raw authorization headers (`Bearer ...`), cookie tokens, or unmasked credit card / PII data present in query parameters. Ensure log sinks sanitize query strings where necessary.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### 1. Core Implementation (`patterns/11_middleware/access_logger.go`)

```go
package middleware

import (
	"net/http"
	"time"
)

// AccessLogEntry holds structured metadata about a completed HTTP request.
type AccessLogEntry struct {
	Method       string
	Path         string
	StatusCode   int
	BytesWritten int64
	Duration     time.Duration
	RemoteAddr   string
	UserAgent    string
}

// LogWriter is a callback invoked when an access log entry is generated.
type LogWriter func(entry AccessLogEntry)

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
	wroteHeader  bool
}

func (w *responseWriterWrapper) WriteHeader(code int) {
	if !w.wroteHeader {
		w.statusCode = code
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *responseWriterWrapper) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += int64(n)
	return n, err
}

// AccessLoggerMiddleware produces structured log records for each request.
func AccessLoggerMiddleware(writer LogWriter) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &responseWriterWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // Default if WriteHeader is never called
			}

			next.ServeHTTP(wrapped, r)

			if writer != nil {
				writer(AccessLogEntry{
					Method:       r.Method,
					Path:         r.URL.Path,
					StatusCode:   wrapped.statusCode,
					BytesWritten: wrapped.bytesWritten,
					Duration:     time.Since(start),
					RemoteAddr:   r.RemoteAddr,
					UserAgent:    r.UserAgent(),
				})
			}
		})
	}
}
```

### 2. Integration with `log/slog` in Production

```go
func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    accessLogger := middleware.AccessLoggerMiddleware(func(entry middleware.AccessLogEntry) {
        level := slog.LevelInfo
        if entry.StatusCode >= 500 {
            level = slog.LevelError
        } else if entry.StatusCode >= 400 {
            level = slog.LevelWarn
        }

        logger.Log(context.Background(), level, "http_request",
            slog.String("method", entry.Method),
            slog.String("path", entry.Path),
            slog.Int("status", entry.StatusCode),
            slog.Int64("bytes", entry.BytesWritten),
            slog.Duration("duration_ms", entry.Duration),
            slog.String("remote_ip", entry.RemoteAddr),
            slog.String("user_agent", entry.UserAgent),
        )
    })

    http.Handle("/api/", accessLogger(http.HandlerFunc(apiHandler)))
    _ = http.ListenAndServe(":8080", nil)
}
```
