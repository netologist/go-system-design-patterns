# Graceful Degradation Pattern

## 1. Overview & Concept
The **Graceful Degradation Pattern** enables high-traffic backend services to maintain core user functionality and deliver partial or fallback responses when non-essential dependencies (such as recommendation engines, personalization algorithms, search ranking models, or analytics pipelines) experience high latency, rate limits, or outright outages.

Instead of propagating downstream errors to end-users (which converts a non-critical component failure into a full HTTP 500 Internal Server Error outage), the system automatically switches to static cached defaults, simplified business logic, or cached historical records.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)
In modern microservice architectures, a single user-facing request may fan out to dozens of downstream services. A hard failure or latency spike in any single peripheral service can cascade and take down the entire user experience.

### Failure Scenarios Without This Pattern
- **Total User Experience Collapse:** An e-commerce homepage fails to render because the machine learning personalized recommendation engine is experiencing an outage, resulting in a 500 error on the main landing page.
- **Latency Amplification & Request Queue Saturation:** When a downstream recommendation service slows down from 20ms to 5,000ms, upstream Go HTTP worker goroutines remain blocked waiting for responses. Upstream request queues overflow, exhausting server memory and file descriptors.
- **Cascading Microservice Outages:** Downstream timeouts trigger aggressive client retries, compounding load on an already struggling downstream service.
- **Binary Availability Anti-Pattern:** The system behaves in an all-or-nothing binary fashion, lacking intermediate operational states.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)
The `ResilientRecommendationService` wraps the primary recommendation engine with a tight, bounded `context.WithTimeout`. If the primary call succeeds within the deadline, personalized items are returned. If the primary call fails or times out, the fallback provider returns static top picks tagged with `IsDegraded: true`:

```
 [ Client Request: GetRecommendations(userID) ]
                      │
                      ▼
 ┌──────────────────────────────────────────┐
 │   ResilientRecommendationService         │
 │   - Bounded Timeout (e.g. 100ms)         │
 └────────────────────┬─────────────────────┘
                      │
                      ▼
 ┌──────────────────────────────────────────┐
 │ Primary Engine: GetRecommendations(ctx)  │
 └────────────────────┬─────────────────────┘
                      │
          ┌───────────┴───────────┐
          ▼                       ▼
      [ Success ]         [ Error / Timeout ]
          │                       │
          ▼                       ▼
   [ Personalized Items ]  ┌─────────────────────────────────┐
   [ IsDegraded: false ]   │ FallbackProvider: Static Picks  │
          │                └──────────────┬──────────────────┘
          │                               │
          │                               ▼
          │                        [ Top 10 Static Items ]
          │                        [ IsDegraded: true ]
          │                               │
          └───────────────┬───────────────┘
                          │
                          ▼
             [ Return Recommendations ]
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Best Practices & Hardening
- **Strict, Bounded Downstream Timeouts:** Enforce aggressive timeouts on non-essential dependencies (e.g., 50ms - 100ms) so slow dependencies fail fast rather than stalling user requests.
- **Explicit Degradation Telemetry & Badging:** Mark degraded payloads (`IsDegraded: true`) and increment Prometheus metrics (`degradation_fallback_total{dependency="recommendations"}`). This alerts SRE teams without impacting users.
- **Pre-Warmed Static Fallbacks:** Keep fallback caches (e.g., top-selling products, curated playlists) in local in-memory structures or fast local caches (Redis) with asynchronous background refresh routines.
- **Circuit Breaker Integration:** If downstream failure rates exceed a threshold (e.g., 50% failures over 10 seconds), open the circuit breaker to skip primary calls entirely and serve fallback data with zero network overhead.

### Pitfalls to Avoid
- **Degrading Critical Core Invariants:** Never degrade critical safety, financial, or authentication logic (e.g., you cannot "fall back" to allowing a payment without authorization).
- **Expensive Fallback Computations:** Fallback logic must be cheap ($O(1)$ in-memory lookups). If the fallback itself involves heavy database queries, it will collapse under load during an incident.
- **Silent Degradation Without Alerts:** Serving degraded data without emitting observability signals masks downstream outages from on-call engineers.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### Core Implementation in `patterns/20_reliability/graceful_degradation.go`
```go
package reliability

import (
	"context"
	"errors"
	"time"
)

// Recommendation represents a product recommendation.
type Recommendation struct {
	ProductID  string
	Title      string
	IsDegraded bool // Indicates whether this came from fallback
}

// PersonalizedRecommendationEngine primary dynamic downstream service.
type PersonalizedRecommendationEngine interface {
	GetRecommendations(ctx context.Context, userID string) ([]Recommendation, error)
}

// FallbackProvider static or cached fallback source.
type FallbackProvider interface {
	GetStaticTopPicks(ctx context.Context) []Recommendation
}

// ResilientRecommendationService serves personalized recommendations, falling back to static top picks if downstream fails or times out.
type ResilientRecommendationService struct {
	primary  PersonalizedRecommendationEngine
	fallback FallbackProvider
	timeout  time.Duration
}

func NewResilientRecommendationService(
	primary PersonalizedRecommendationEngine,
	fallback FallbackProvider,
	timeout time.Duration,
) *ResilientRecommendationService {
	if timeout <= 0 {
		timeout = 100 * time.Millisecond
	}
	return &ResilientRecommendationService{
		primary:  primary,
		fallback: fallback,
		timeout:  timeout,
	}
}

// GetRecommendations executes primary engine with timeout; if it errors or times out, falls back to static defaults.
func (s *ResilientRecommendationService) GetRecommendations(ctx context.Context, userID string) ([]Recommendation, error) {
	childCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	recs, err := s.primary.GetRecommendations(childCtx, userID)
	if err == nil && len(recs) > 0 {
		return recs, nil
	}

	// Primary failed or timed out -> Graceful Degradation to fallback
	fallbackRecs := s.fallback.GetStaticTopPicks(ctx)
	if len(fallbackRecs) == 0 {
		return nil, errors.New("both primary and fallback recommendation sources failed")
	}

	// Mark as degraded mode for observability / UI badge
	degraded := make([]Recommendation, len(fallbackRecs))
	for i, r := range fallbackRecs {
		degraded[i] = Recommendation{
			ProductID:  r.ProductID,
			Title:      r.Title,
			IsDegraded: true,
		}
	}

	return degraded, nil
}
```

### Production API Handler Example
```go
func HandleGetFeed(svc *reliability.ResilientRecommendationService) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := r.URL.Query().Get("user_id")
        
        items, err := svc.GetRecommendations(r.Context(), userID)
        if err != nil {
            http.Error(w, "internal server error", http.StatusInternalServerError)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        if len(items) > 0 && items[0].IsDegraded {
            w.Header().Set("X-Degraded-Mode", "true")
        }

        _ = json.NewEncoder(w).Encode(items)
    }
}
```
