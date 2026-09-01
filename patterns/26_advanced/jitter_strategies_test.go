package advanced_test

import (
	"testing"
	"time"

	advanced "system-design-patterns/patterns/26_advanced"
)

func TestJitterStrategies_Bounds(t *testing.T) {
	base := 10 * time.Millisecond
	max := 100 * time.Millisecond

	// 1. Full Jitter
	for i := 1; i <= 10; i++ {
		d := advanced.FullJitter(i, base, max)
		if d < 0 || d > max {
			t.Errorf("full jitter out of bounds: %v", d)
		}
	}

	// 2. Equal Jitter
	for i := 1; i <= 10; i++ {
		d := advanced.EqualJitter(i, base, max)
		if d < 0 || d > max {
			t.Errorf("equal jitter out of bounds: %v", d)
		}
	}

	// 3. Decorrelated Jitter
	prev := base
	for range 10 {
		d := advanced.DecorrelatedJitter(prev, base, max)
		if d < base || d > max {
			t.Errorf("decorrelated jitter out of bounds: %v", d)
		}
		prev = d
	}
}
