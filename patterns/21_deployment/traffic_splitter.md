# Dynamic Traffic Splitter Pattern (Canary & Blue/Green Router)

## 1. Overview & Concept
The **Dynamic Traffic Splitter Pattern** controls the gradual routing of production user traffic between distinct application deployment versions (such as **Blue** for the stable current version and **Green** for the newly deployed canary candidate). Instead of an all-at-once deployment where 100% of user traffic is instantly switched to new code, traffic splitting enables progressive rollouts (e.g., 1% -> 5% -> 25% -> 100%).

This pattern supports two primary operational capabilities:
1. **Dynamic Canary Weighting:** Real-time, thread-safe adjustment of traffic percentages (0% to 100%) routed to the Green deployment using fast atomic integers (`sync/atomic.Int64`) and random weighted distribution (`rand/v2`).
2. **Explicit Testing Overrides:** Bypassing the random split using request headers (`X-Canary-Test: true`) or cookies, enabling QA engineers, automated smoke tests, and synthetic monitoring probes to target the Green deployment deterministically before public traffic is introduced.
3. **Instant Atomic Cutover & Rollback:** Switching the baseline color (`SwitchActiveColor`) atomically without redeploying proxy instances.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)
Deploying new application versions directly to 100% of production traffic exposes all users to latent bugs, database connection saturation, and unobserved edge-case regressions.

### Failure Scenarios Without This Pattern
- **Catastrophic Blast Radius:** A latent memory leak or unhandled nil pointer exception deployed to 100% of instances immediately crashes the entire service fleet, impacting all active users simultaneously.
- **Undetected Database Query Regressions:** A missing database index in a new release goes unnoticed in staging but saturates production database CPU under full production load.
- **Complex & Slow Rollbacks:** Without traffic splitting, rolling back a failed release requires initiating a full container redeployment, which can take 5 to 15 minutes during a critical outage. With a traffic splitter, rollback to 0% is an atomic sub-millisecond memory update.
- **Inability to Run Internal Smoke Tests in Production:** QA teams cannot verify new builds against real production databases and third-party integrations prior to public release.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)
The `TrafficSplitter` evaluates incoming HTTP requests to resolve the target deployment version:

```
                  [ Incoming HTTP Request ]
                              │
                              ▼
               ┌──────────────────────────────┐
               │    TrafficSplitter.Route     │
               └──────────────┬───────────────┘
                              │
       ┌──────────────────────┴──────────────────────┐
       │ Does request contain "X-Canary-Test: true"? │
       └──────────────────────┬──────────────────────┘
                     │                 │
                   [Yes]              [No]
                     │                 │
                     ▼                 ▼
             [ Return GREEN ]   ┌──────────────────────────────┐
                                │ Load atomic canaryWeight %   │
                                └──────────────┬───────────────┘
                                               │
                                ┌──────────────┴──────────────┐
                                │ weight == 0? -> Return Blue │
                                │ weight == 100? -> Ret Green │
                                └──────────────┬──────────────┘
                                               │
                                               ▼
                                ┌─────────────────────────────┐
                                │ rand.Int64N(100) < weight?  │
                                └──────────────┬──────────────┘
                                       │                 │
                                     [Yes]              [No]
                                       │                 │
                                       ▼                 ▼
                                [ Return GREEN ]  [ Return BLUE ]
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Best Practices & Hardening
- **Zero-Lock Atomic State:** Use `atomic.Int64` for `canaryWeight` and `atomic.Value` for `activeColor`. Route resolution executes in under 10 nanoseconds without mutex contention.
- **User-Consistent Sticky Sessions:** In production user-facing web applications, replace pure per-request randomness with consistent hashing on `userID` or `deviceID` (e.g., `hash(userID) % 100 < weight`). This guarantees that a single user experiences a consistent UI version across consecutive page views.
- **Automated Metric-Driven Canary Analysis:** Integrate the traffic splitter with Prometheus metric queries (e.g., error rate, p99 latency). If the Green version exceeds SLA error thresholds, automatically decrement weight to 0%.
- **Header Sanitization on Public Edge Gateways:** Strip `X-Canary-Test` headers at the public edge reverse proxy (Cloudflare/CloudFront) if internal canary testing should not be accessible by untrusted external users.

### Pitfalls to Avoid
- **Incompatible Database Schema Changes:** If the Green version requires a schema migration that breaks the Blue version, traffic splitting cannot be used safely without applying the **Expand/Contract (Parallel Run) Database Pattern** first.
- **Shared In-Memory State:** If the traffic splitter runs across multiple API gateway replicas, synchronize `canaryWeight` via a distributed configuration store (etcd, Consul, Redis Pub/Sub) or dynamic Kubernetes ConfigMaps.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### Core Implementation in `patterns/21_deployment/traffic_splitter.go`
```go
package deployment

import (
	"math/rand/v2"
	"net/http"
	"sync/atomic"
)

type DeploymentColor string

const (
	ColorBlue  DeploymentColor = "BLUE"
	ColorGreen DeploymentColor = "GREEN"
)

// TrafficSplitter routes traffic between Blue and Green deployments for Canaries and Zero-Downtime cutovers.
type TrafficSplitter struct {
	canaryWeight atomic.Int64 // Percentage 0-100 routed to Green
	activeColor  atomic.Value // DeploymentColor for full cutover
}

func NewTrafficSplitter(initialWeight int64) *TrafficSplitter {
	s := &TrafficSplitter{}
	s.SetCanaryWeight(initialWeight)
	s.activeColor.Store(ColorBlue)
	return s
}

// SetCanaryWeight dynamically adjusts canary percentage (0 to 100).
func (s *TrafficSplitter) SetCanaryWeight(weight int64) {
	if weight < 0 {
		weight = 0
	} else if weight > 100 {
		weight = 100
	}
	s.canaryWeight.Store(weight)
}

// SwitchActiveColor performs instant Blue/Green cutover.
func (s *TrafficSplitter) SwitchActiveColor(c DeploymentColor) {
	s.activeColor.Store(c)
}

// RouteTarget resolves whether request should be served by Blue or Green.
func (s *TrafficSplitter) RouteTarget(r *http.Request) DeploymentColor {
	// 1. Explicit Canary Header Override (for QA / internal testing)
	if r.Header.Get("X-Canary-Test") == "true" {
		return ColorGreen
	}

	weight := s.canaryWeight.Load()
	if weight == 0 {
		return s.activeColor.Load().(DeploymentColor)
	}
	if weight == 100 {
		return ColorGreen
	}

	// 2. Randomized Canary Weight Split
	if rand.Int64N(100) < weight {
		return ColorGreen
	}

	return ColorBlue
}
```

### Production Reverse Proxy Gateway Example
```go
func NewCanaryReverseProxy(
    splitter *deployment.TrafficSplitter,
    blueURL, greenURL *url.URL,
) http.Handler {
    blueProxy := httputil.NewSingleHostReverseProxy(blueURL)
    greenProxy := httputil.NewSingleHostReverseProxy(greenURL)

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        target := splitter.RouteTarget(r)

        // Inject routing telemetry header
        w.Header().Set("X-Deployment-Target", string(target))

        switch target {
        case deployment.ColorGreen:
            greenProxy.ServeHTTP(w, r)
        default:
            blueProxy.ServeHTTP(w, r)
        }
    })
}
```
