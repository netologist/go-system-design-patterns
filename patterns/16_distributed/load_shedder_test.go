package distributed_test

import (
	"errors"
	"testing"

	distributed "system-design-patterns/patterns/16_distributed"
)

func TestAdaptiveLoadShedder_PriorityDegradation(t *testing.T) {
	shedder := distributed.NewAdaptiveLoadShedder(10) // max 10: lowThreshold=7, normalThreshold=9

	var releases []func()
	defer func() {
		for _, r := range releases {
			if r != nil {
				r()
			}
		}
	}()

	// Fill up 7 slots with critical traffic
	for range 7 {
		rel, err := shedder.Allow(distributed.PriorityCritical)
		if err != nil {
			t.Fatalf("unexpected error filling slots: %v", err)
		}
		releases = append(releases, rel)
	}

	// In-flight is now 7 (> lowThreshold 7 for next request)
	// 8th request with PriorityLow should be shed (dropped)
	_, err := shedder.Allow(distributed.PriorityLow)
	if !errors.Is(err, distributed.ErrLoadSheddingDrop) {
		t.Errorf("expected PriorityLow to be dropped above 70%% load, got: %v", err)
	}

	// 8th request with PriorityNormal should still be allowed
	relNormal, err := shedder.Allow(distributed.PriorityNormal)
	if err != nil {
		t.Fatalf("expected PriorityNormal allowed at 80%% load: %v", err)
	}
	releases = append(releases, relNormal)

	// 9th request with PriorityCritical should still be allowed
	relCrit, err := shedder.Allow(distributed.PriorityCritical)
	if err != nil {
		t.Fatalf("expected PriorityCritical allowed: %v", err)
	}
	releases = append(releases, relCrit)
}
