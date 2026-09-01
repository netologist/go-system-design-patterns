package concurrency_test

import (
	"sync"
	"testing"

	concurrency "system-design-patterns/patterns/09_concurrency"
)

func TestGenerateSequence_ChannelOwnership(t *testing.T) {
	ch := concurrency.GenerateSequence(5)
	var items []int
	for val := range ch {
		items = append(items, val)
	}

	if len(items) != 5 {
		t.Fatalf("expected 5 items, got: %d", len(items))
	}
}

func TestCounters_Concurrency(t *testing.T) {
	safe := &concurrency.SafeCounter{}
	atomic := &concurrency.AtomicCounter{}

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				safe.Inc()
				atomic.Inc()
			}
		}()
	}

	wg.Wait()

	if safe.Value() != 5000 {
		t.Errorf("expected SafeCounter=5000, got: %d", safe.Value())
	}
	if atomic.Value() != 5000 {
		t.Errorf("expected AtomicCounter=5000, got: %d", atomic.Value())
	}
}
