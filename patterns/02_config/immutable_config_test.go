package config_test

import (
	"sync"
	"testing"
	"time"

	config "system-design-patterns/patterns/02_config"
)

func TestImmutableConfig_DefensiveCopy(t *testing.T) {
	features := map[string]bool{"beta_feature": true}
	endpoints := []string{"https://api.internal/v1"}

	cfg := config.NewImmutableConfig(config.AppSettings{
		Environment: "prod",
		Port:        8080,
		Features:    features,
		Endpoints:   endpoints,
		Timeout:     5 * time.Second,
	})

	// Mutate original sources
	features["beta_feature"] = false
	endpoints[0] = "mutated"

	// Verify immutable config remains untouched
	if !cfg.FeatureEnabled("beta_feature") {
		t.Errorf("config features map was mutated externally")
	}

	eps := cfg.Endpoints()
	if eps[0] != "https://api.internal/v1" {
		t.Errorf("config endpoints slice was mutated externally")
	}

	// Mutate slice returned from getter
	eps[0] = "mutated_again"
	if cfg.Endpoints()[0] != "https://api.internal/v1" {
		t.Errorf("getter did not return defensive copy")
	}
}

func TestConfigHolder_AtomicSwap(t *testing.T) {
	v1 := config.NewImmutableConfig(config.AppSettings{Port: 8080})
	v2 := config.NewImmutableConfig(config.AppSettings{Port: 9090})

	holder := config.NewConfigHolder(v1)

	var wg sync.WaitGroup
	// Concurrent readers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				curr := holder.Get()
				p := curr.Port()
				if p != 8080 && p != 9090 {
					t.Errorf("unexpected port during concurrent read: %d", p)
				}
			}
		}()
	}

	// Writer swaps
	holder.Swap(v2)
	wg.Wait()

	if holder.Get().Port() != 9090 {
		t.Errorf("expected final port 9090, got: %d", holder.Get().Port())
	}
}
