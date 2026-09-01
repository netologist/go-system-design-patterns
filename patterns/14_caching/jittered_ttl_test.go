package caching_test

import (
	"testing"
	"time"

	caching "system-design-patterns/patterns/14_caching"
)

func TestCalculateJitteredTTL_Spread(t *testing.T) {
	baseTTL := 10 * time.Minute
	ratio := 0.20 // ±20% -> [8m, 12m]

	for range 50 {
		jittered := caching.CalculateJitteredTTL(baseTTL, ratio)
		if jittered < 8*time.Minute || jittered > 12*time.Minute {
			t.Errorf("jittered TTL %v outside expected bounds [8m, 12m]", jittered)
		}
	}
}
