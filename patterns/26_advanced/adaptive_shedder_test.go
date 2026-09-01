package advanced_test

import (
	"errors"
	"testing"
	"time"

	advanced "system-design-patterns/patterns/26_advanced"
)

func TestLatencyEMAShedder_ShedsUnderHighLatency(t *testing.T) {
	shedder := advanced.NewLatencyEMAShedder(50*time.Millisecond, 20)

	// 1. Initial requests pass
	done, err := shedder.Allow()
	if err != nil {
		t.Fatalf("expected initial request allowed, got: %v", err)
	}

	// Record several very slow requests (500ms) to drive up latency EMA
	for range 15 {
		d, err := shedder.Allow()
		if err == nil {
			time.Sleep(5 * time.Millisecond)
			d()
		}
	}
	done()

	// Simulate sudden massive latency spike
	doneSpike, err := shedder.Allow()
	if err == nil {
		time.Sleep(600 * time.Millisecond) // Slow
		doneSpike()
	}

	// Now EMA exceeds 50ms target -> Subsequent requests should be shed
	_, err = shedder.Allow()
	if !errors.Is(err, advanced.ErrShedded) {
		t.Errorf("expected ErrShedded after latency degradation, got: %v (current EMA: %v)",
			err, shedder.CurrentEMA())
	}
}
