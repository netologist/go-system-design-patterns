# Body Limit and Content-Type Enforcement

## Overview & Definition

In production Go HTTP services, handling untrusted client input requires strict defensive boundaries before allocating memory or invoking expensive serialization routines. The **Body Limit and Content-Type Enforcement** pattern provides middleware layers that:
1. Cap the maximum allowable byte size of an incoming HTTP request body using `http.MaxBytesReader`.
2. Enforce expected MIME types (e.g., `application/json`) via Content-Type header validation while ignoring body-less HTTP methods (`GET`, `HEAD`, `OPTIONS`).

This pattern acts as a gatekeeper at the ingress edge of an HTTP handler pipeline, ensuring the server rejects malicious payloads, runaway uploads, and mismatched media types before downstream handlers waste CPU, memory, or thread capacity.

---

## Problem Statement

Without body size and media type validation, HTTP servers are susceptible to several critical failure modes:

* **Denial of Service via Memory Exhaustion (OOM):** If a handler uses `io.ReadAll(r.Body)` or feeds an unconstrained stream to `json.NewDecoder(r.Body)`, an attacker can stream gigabytes of garbage data. The server buffers or decodes the stream until the Go runtime triggers an Out-Of-Memory (OOM) panic or Kubernetes kills the container.
* **Slowloris & Resource Starvation:** Attackers can transmit infinite streams byte-by-byte, tying up server worker goroutines and socket buffers indefinitely.
* **MIME Confusion & Parser Vulnerabilities:** Accepting arbitrary media types (or requests lacking a `Content-Type`) can cause backend parsers to misinterpret formats, trigger unexpected exceptions, or expose parser-level vulnerabilities.
* **Unnecessary Downstream Computation:** Processing, logging, and attempting to parse requests that don't match the required contract consumes database connections, CPU cycles, and lock contention.

---

## Architectural Mechanism & Flow

```
                           +----------------------------------------+
                           |         Incoming HTTP Request          |
                           +----------------------------------------+
                                               |
                                               v
                        +----------------------------------------------+
                        |           RequireContentType()               |
                        | (Checks Content-Type for POST/PUT/PATCH/etc) |
                        +----------------------------------------------+
                                         /            \
                       [Invalid / Missing]            [Valid MIME / Body-less]
                                       /                \
                                      v                  v
                   +------------------------+  +-----------------------------------+
                   | 415 Unsupported Media  |  |       BodyLimitMiddleware()       |
                   | Type (Terminate early) |  | Wraps r.Body with MaxBytesReader  |
                   +------------------------+  +-----------------------------------+
                                                                 |
                                                                 v
                                               +-----------------------------------+
                                               |         Downstream Handler        |
                                               | Reads up to maxBytes; if exceeded |
                                               | MaxBytesReader throws MaxBytesError|
                                               +-----------------------------------+
                                                                 |
                                                                 v
                                               +-----------------------------------+
                                               |    413 Request Entity Too Large   |
                                               |        or 200 OK Execution        |
                                               +-----------------------------------+
```

### Key Mechanism Details
1. `http.MaxBytesReader(w, r.Body, maxBytes)` wraps the underlying `io.ReadCloser`. It tracks the number of bytes read through `Read()` calls.
2. If the stream exceeds `maxBytes`, `MaxBytesReader` returns an error (`*http.MaxBytesError`), sets an internal flag on `http.ResponseWriter` to close the connection after serving, and truncates further reading.
3. `mime.ParseMediaType` extracts the primary media type (stripping parameters like `; charset=utf-8`) and ensures case-insensitive equality.

---

## Production Best Practices & Pitfalls

### Best Practices
* **Apply Body Limits Early:** Wrap `r.Body` as close to the root router/middleware chain as possible before any logging, telemetry, or parsing occurs.
* **Distinguish Route Limits:** General JSON APIs typically need small limits (e.g., 64 KB – 1 MB), whereas multipart file upload endpoints require larger, dedicated limits (e.g., 10 MB – 50 MB). Do not apply a global 50 MB limit to lightweight CRUD routes.
* **Handle `*http.MaxBytesError` in Handlers:** When reading or decoding fails with `*http.MaxBytesError`, return HTTP status `413 Request Entity Too Large` rather than a generic `500 Internal Server Error`.
* **Normalize MIME Types:** Always use `mime.ParseMediaType` instead of raw string comparison (`r.Header.Get("Content-Type") == "application/json"`) to gracefully handle valid variants like `application/json; charset=utf-8`.

### Common Pitfalls
* **Reading `r.Body` Before Wrapping:** If any intermediate middleware reads `r.Body` (e.g., for request logging or signature validation) prior to `MaxBytesReader`, memory exhaustion can occur before the limiter executes.
* **Leaking Connections on HTTP/1.1:** When an oversized request is received on HTTP/1.1 without consuming or closing the body, the client connection might remain out of sync. `http.MaxBytesReader` automatically marks the connection for closure upon limit breach.
* **Blocking Safe Body-less Requests:** Forcing `Content-Type` checks on `GET` or `DELETE` requests breaks standard HTTP conventions and standard client libraries.

---

## Code Walkthrough & Usage

### 1. Middleware Definition (`body_limit.go`)

```go
package httpserver

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"
)

var (
	ErrBodyTooLarge       = errors.New("request body exceeds maximum allowed size")
	ErrInvalidContentType = errors.New("invalid or unsupported Content-Type header")
	ErrMissingContentType = errors.New("missing Content-Type header")
)

// BodyLimitMiddleware enforces maximum request body size using http.MaxBytesReader.
func BodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireContentType verifies that incoming requests match the expected media type (e.g. application/json).
func RequireContentType(expectedMediaType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip check on body-less requests
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			contentType := r.Header.Get("Content-Type")
			if strings.TrimSpace(contentType) == "" {
				http.Error(w, ErrMissingContentType.Error(), http.StatusUnsupportedMediaType)
				return
			}

			mediaType, _, err := mime.ParseMediaType(contentType)
			if err != nil || !strings.EqualFold(mediaType, expectedMediaType) {
				http.Error(
					w,
					fmt.Sprintf("%s (expected: %s, got: %s)", ErrInvalidContentType.Error(), expectedMediaType, contentType),
					http.StatusUnsupportedMediaType,
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
```

### 2. Integration Example

```go
func main() {
    mux := http.NewServeMux()

    userHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Read bounded body
        data, err := io.ReadAll(r.Body)
        if err != nil {
            var maxBytesErr *http.MaxBytesError
            if errors.As(err, &maxBytesErr) {
                http.Error(w, "Payload too large", http.StatusRequestEntityTooLarge)
                return
            }
            http.Error(w, "Failed to read body", http.StatusBadRequest)
            return
        }
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Processed successfully"))
    })

    // Chain: Content-Type Check -> 1MB Body Limit -> Handler
    chain := RequireContentType("application/json")(
        BodyLimitMiddleware(1 << 20)(userHandler),
    )

    mux.Handle("POST /api/v1/users", chain)
    http.ListenAndServe(":8080", mux)
}
```
