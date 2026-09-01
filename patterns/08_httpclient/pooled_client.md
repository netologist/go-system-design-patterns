# Pooled HTTP Client Configuration

## Overview & Definition

In Go, `http.DefaultClient` and `http.Client{}` use `http.DefaultTransport`, which sets `MaxIdleConnsPerHost = 2`. In high-throughput microservices issuing hundreds of concurrent outbound HTTP calls to the same downstream hostname, this default causes extreme connection churn: only 2 idle TCP connections are kept alive per host, and every other connection is aggressively closed and reopened.

The **Pooled HTTP Client** pattern customizes the underlying `http.Transport` with tuned connection limits:
1. `MaxIdleConns`: Maximum total idle (keep-alive) connections across all hosts.
2. `MaxIdleConnsPerHost`: Maximum idle connections maintained per downstream host.
3. `MaxConnsPerHost`: Hard limit on total active and idle connections per host, protecting downstream services from connection storms.
4. `IdleConnTimeout`: Lifetime for idle connections in the pool before closure.
5. Explicit timeouts for connection dialing (`DialContext`), TLS handshakes, and overall request life cycle.

---

## Problem Statement

Relying on default Go `http.Client` configurations in production causes several severe issues:

* **TCP Connection Churn & Port Exhaustion (TIME_WAIT):** With `MaxIdleConnsPerHost = 2`, any concurrency above 2 causes Go to open new TCP sockets and close them immediately after reading the response. Sockets enter the TCP `TIME_WAIT` state (typically 60 seconds), consuming ephemeral ports until the OS kernel runs out of source ports (`bind: address already in use`).
* **High Latency from Repeated TLS Handshakes:** Every new TCP connection requires a 3-way TCP handshake followed by a multi-roundtrip TLS 1.3 handshake. Reusing established connections cuts p99 latency from ~50ms to <2ms in internal networks.
* **Downstream Overload from Unbounded Concurrency:** Without `MaxConnsPerHost`, a sudden surge in upstream traffic can cause a service to open thousands of concurrent TCP sockets to a single database or downstream microservice, triggering cascading outages.
* **Hung Requests without Overall Timeout:** If `http.Client.Timeout` is 0 (default), slow downstreams or half-open TCP connections can hang goroutines indefinitely.

---

## Architectural Mechanism & Flow

```
                      Goroutine Request Pool (e.g. 50 concurrent calls)
                                     |
                                     v
                        +---------------------------+
                        |      *http.Client         |
                        | (OverallTimeout: 10s)     |
                        +---------------------------+
                                     |
                                     v
                        +---------------------------+
                        |     *http.Transport       |
                        +---------------------------+
                                     |
                   +-----------------+-----------------+
                   |                                   |
                   v                                   v
        [Check Idle Pool for Host]           [No Idle Connection]
                   |                                   |
         +---------+---------+               +---------+---------+
         | Idle Conn Exists  |               | Total Conns <     |
         | & Not Expired     |               | MaxConnsPerHost?  |
         +---------+---------+               +---------+---------+
                   |                                   |
                   v                         [Yes]           [No]
          [Reuse Connection]                   |               |
          (No TLS/TCP Handshake)     +---------+-------+  +----+----+
                                     | DialContext     |  | Block / |
                                     | + TLS Handshake |  | Queue   |
                                     +-----------------+  +---------+
```

### Configuration Comparison
| Parameter | Default Go Value | Production Pooled Value | Purpose |
|---|---|---|---|
| `MaxIdleConns` | `100` | `200` – `1000` | Global connection reuse ceiling |
| `MaxIdleConnsPerHost` | `2` *(Bottleneck!)* | `50` – `200` | High-concurrency reuse to single hosts |
| `MaxConnsPerHost` | `0` (Unbounded) | `100` – `500` | Downstream connection flood protection |
| `IdleConnTimeout` | `90s` | `90s` | Idle socket lifecycle cleanup |
| `TLSHandshakeTimeout` | `10s` | `5s` | Prevent stalled TLS negotiations |
| `Client.Timeout` | `0` (No timeout) | `5s` – `15s` | Hard boundary on entire request lifecycle |

---

## Production Best Practices & Pitfalls

### Best Practices
* **Share Client Instances as Singletons:** `*http.Client` is safe for concurrent use across multiple goroutines. Create one client instance per target service or lifecycle and reuse it throughout the application lifetime. Creating `http.Client` per request defeats connection pooling entirely.
* **Always Drain and Close Response Bodies:** If `resp.Body.Close()` is skipped, or if the body is not drained to `io.Discard`, the underlying TCP connection cannot be returned to the pool and is discarded.
* **Enable HTTP/2 Multiplexing:** Ensure `ForceAttemptHTTP2: true` is set so multiple concurrent streams share single TCP sockets for HTTP/2 and gRPC downstreams.

### Common Pitfalls
* **Creating `&http.Client{}` inside Handlers:** Instantiating a new client or transport for every request creates a new connection pool for every call, causing catastrophic memory leaks and file descriptor exhaustion.
* **Ignoring Proxy Configurations:** When overriding `http.Transport`, forgetting `Proxy: http.ProxyFromEnvironment` will break corporate proxy environments (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`).

---

## Code Walkthrough & Usage

### 1. Implementation (`pooled_client.go`)

```go
package httpclient

import (
	"net"
	"net/http"
	"time"
)

// PooledClientConfig encapsulates connection pooling parameters.
type PooledClientConfig struct {
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	MaxConnsPerHost     int
	IdleConnTimeout     time.Duration
	ConnectTimeout      time.Duration
	TLSHandshakeTimeout time.Duration
	OverallTimeout      time.Duration
}

// DefaultPooledClientConfig provides hardened production defaults.
func DefaultPooledClientConfig() PooledClientConfig {
	return PooledClientConfig{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 50,
		MaxConnsPerHost:     100, // Protect against downstream connection storms
		IdleConnTimeout:     90 * time.Second,
		ConnectTimeout:      5 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
		OverallTimeout:      10 * time.Second,
	}
}

// NewPooledHTTPClient creates an *http.Client configured for high-concurrency reuse.
func NewPooledHTTPClient(cfg PooledClientConfig) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   cfg.ConnectTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:       cfg.MaxConnsPerHost,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   cfg.OverallTimeout,
	}
}
```

### 2. Singleton Service Injection

```go
type PaymentGatewayService struct {
    client  *http.Client
    baseURL string
}

func NewPaymentGatewayService(baseURL string) *PaymentGatewayService {
    cfg := DefaultPooledClientConfig()
    cfg.MaxConnsPerHost = 50
    cfg.OverallTimeout = 3 * time.Second

    return &PaymentGatewayService{
        client:  NewPooledHTTPClient(cfg),
        baseURL: baseURL,
    }
}
```
