package caching

import (
	"math/rand/v2"
	"time"
)

// CalculateJitteredTTL adds a random variation percentage to baseTTL.
// e.g. baseTTL = 10m, jitterRatio = 0.20 -> result between [8m, 12m]
// This prevents thousands of cache keys initialized at startup from expiring simultaneously (Thundering Herd).
func CalculateJitteredTTL(baseTTL time.Duration, jitterRatio float64) time.Duration {
	if baseTTL <= 0 {
		return 0
	}
	if jitterRatio <= 0 {
		return baseTTL
	}
	if jitterRatio > 0.5 {
		jitterRatio = 0.5 // Bound max jitter to ±50%
	}

	maxDelta := float64(baseTTL) * jitterRatio
	// Random float between [-maxDelta, +maxDelta]
	delta := (rand.Float64()*2.0 - 1.0) * maxDelta

	jittered := float64(baseTTL) + delta
	if jittered < float64(time.Second) {
		return time.Second
	}

	return time.Duration(jittered)
}
