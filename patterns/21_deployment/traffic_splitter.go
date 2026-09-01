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
