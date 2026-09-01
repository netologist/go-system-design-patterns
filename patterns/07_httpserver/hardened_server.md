# Hardened HTTP Server Configuration

## Overview & Definition

In Go, `http.ListenAndServe(":8080", handler)` instantiates an `http.Server` with zero timeouts by default. A zero timeout means connections never time out while reading headers, reading request bodies, or writing responses. The **Hardened HTTP Server** pattern replaces default configurations with an explicitly tuned `*http.Server` configured with defensive timeout deadlines:
1. `ReadHeaderTimeout`: Caps the time allowed to read incoming HTTP request headers (crucial defense against Slowloris attacks).
2. `ReadTimeout`: Maximum duration for reading the entire request, including the body.
3. `WriteTimeout`: Maximum duration before timing out writes of the response.
4. `IdleTimeout`: Maximum amount of time to wait for the next request when keep-alives are enabled.
5. `MaxHeaderBytes`: Bounds the maximum memory allocated for parsing request headers.

---

## Problem Statement

Deploying Go HTTP servers with default zero timeouts exposes production environments to severe vulnerabilities:

* **Slowloris Attacks:** Attackers open thousands of TCP connections and transmit HTTP header bytes at excruciatingly slow rates (e.g., 1 byte every 10 seconds). Because headers never complete, the default Go HTTP server holds worker goroutines and file descriptors open indefinitely until all file descriptors are exhausted and legitimate clients are refused.
* **Leaked Connections on Slow/Aborted Clients:** Clients on poor mobile networks or malicious actors can pause reading responses or stop sending body chunks. Without `ReadTimeout` and `WriteTimeout`, server goroutines stall indefinitely.
* **Keep-Alive Resource Exhaustion:** Without `IdleTimeout`, idle persistent connections from inactive clients consume memory and socket descriptors, starving the server during traffic surges.
* **Unbounded Header Allocations:** Without `MaxHeaderBytes`, bloated header structures can consume significant heap allocations before routing.

---

## Architectural Mechanism & Flow

```
                               +-----------------------------+
                               |     Client TCP Connect      |
                               +-----------------------------+
                                              |
                                              v
                              +-------------------------------+
                              | ReadHeaderTimeout (e.g. 2s)   |
                              | Reads request line + headers  |
                              +-------------------------------+
                                      /               \
                          [Header timeout]        [Headers received]
                                    /                   \
                                   v                     v
                        +--------------------+  +-------------------------------+
                        | Close TCP / 408    |  | ReadTimeout (e.g. 5s)         |
                        | Request Timeout    |  | Reads request body payload    |
                        +--------------------+  +-------------------------------+
                                                                 |
                                                                 v
                                                +-------------------------------+
                                                | Handler Execution & Response  |
                                                | WriteTimeout (e.g. 10s)       |
                                                +-------------------------------+
                                                                 |
                                                                 v
                                                +-------------------------------+
                                                | Connection Idle / Keep-Alive  |
                                                | IdleTimeout (e.g. 120s)       |
                                                +-------------------------------+
```

### Timeout Details
| Configuration Field | Recommended Production Value | Protected Failure Mode |
|---|---|---|
| `ReadHeaderTimeout` | `2s` – `5s` | Slowloris header attacks, dangling half-open TCP connections |
| `ReadTimeout` | `5s` – `15s` | Stalled upload streams, slow client body transmissions |
| `WriteTimeout` | `10s` – `30s` | Slow client download drains, hung handlers |
| `IdleTimeout` | `60s` – `120s` | Idle keep-alive descriptor exhaustion |
| `MaxHeaderBytes` | `1 << 20` (1 MB) | Header memory bloating |

---

## Production Best Practices & Pitfalls

### Best Practices
* **Always Set `ReadHeaderTimeout`:** Even if `ReadTimeout` is customized or omitted for streaming endpoints (e.g. WebSockets / SSE), `ReadHeaderTimeout` must always be explicitly set.
* **Set `IdleTimeout` Separately from `ReadTimeout`:** If `IdleTimeout` is not explicitly configured, Go defaults to using the value of `ReadTimeout`. For keep-alive connections, `ReadTimeout` is often too short for legitimate idle periods, causing excessive connection churn.
* **Support Graceful Shutdown:** Pair the hardened server with `server.Shutdown(ctx)` listening for `os.Interrupt` and `syscall.SIGTERM` to allow in-flight requests to drain within a bounded deadline.

### Common Pitfalls
* **`WriteTimeout` vs Streaming Responses (SSE / Large Files):** `WriteTimeout` starts from the end of reading the request header and applies to the entire duration of the response write. If serving long-lived Server-Sent Events (SSE) or large multi-gigabyte downloads, a global `WriteTimeout` will terminate active streams prematurely. For streaming routes, use `http.ResponseController` (Go 1.20+) to manage per-request deadlines.
* **Reverse Proxy Mismatches:** If running behind NGINX, Envoy, or AWS ALB, ensure the server's `IdleTimeout` is slightly higher than the upstream proxy's keep-alive timeout to prevent race conditions where the backend closes an idle connection just as the proxy reuses it.

---

## Code Walkthrough & Usage

### 1. Configuration & Factory (`hardened_server.go`)

```go
package httpserver

import (
	"net/http"
	"time"
)

// HardenedServerConfig defines production-grade timeout parameters.
type HardenedServerConfig struct {
	Addr              string
	ReadHeaderTimeout time.Duration // Crucial for Slowloris mitigation
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
}

// DefaultHardenedServerConfig provides production-ready defensive defaults.
func DefaultHardenedServerConfig(addr string) HardenedServerConfig {
	return HardenedServerConfig{
		Addr:              addr,
		ReadHeaderTimeout: 2 * time.Second,   // Fast rejection of slow-reading headers
		ReadTimeout:       5 * time.Second,   // Maximum time reading full request body
		WriteTimeout:      10 * time.Second,  // Maximum time writing response
		IdleTimeout:       120 * time.Second, // Keep-alive idle connection lifetime
		MaxHeaderBytes:    1 << 20,           // 1 MB max header size
	}
}

// NewHardenedHTTPServer creates a standard *http.Server configured with hardened timeouts.
func NewHardenedHTTPServer(cfg HardenedServerConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}
}
```

### 2. Production Server Lifecycle Example

```go
func main() {
    router := http.NewServeMux()
    router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    cfg := DefaultHardenedServerConfig(":8080")
    srv := NewHardenedHTTPServer(cfg, router)

    go func() {
        if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            log.Fatalf("Server failed: %v", err)
        }
    }()

    // Listen for OS shutdown signals
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
    <-stop

    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        log.Fatalf("Server forced to shutdown: %v", err)
    }
}
```
