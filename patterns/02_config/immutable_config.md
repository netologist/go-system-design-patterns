# Immutable Config Pattern (Hot-Reload Safe)

## 1. Overview & Concept
The **Immutable Config** pattern ensures that application configuration objects are completely read-only after initialization and cannot be mutated at runtime by concurrent goroutines.

Furthermore, by utilizing Go's lock-free pointer container `atomic.Pointer[T]` inside a `ConfigHolder`, the pattern supports zero-downtime, thread-safe configuration hot-reloading. New configuration snapshots are prepared and validated independently, then atomically swapped into place without requiring read locks or blocking active in-flight HTTP request handlers.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Sharing mutable configuration structures across high-concurrency applications causes critical bugs:

- **Data Races on Dynamic Reads/Writes**: If a background routine attempts to reload a feature flag or update an endpoint list in a shared struct while hundreds of request goroutines are reading it, Go's race detector flags critical data races, leading to memory corruption or crashes.
- **Leaked Pointer Mutations**: Returning raw slice or map references (e.g., `cfg.Endpoints` or `cfg.Features`) allows caller functions to modify the internal slice or map contents directly, corrupting configuration state globally.
- **Lock Contention on Hot Paths**: Protecting configuration reads with a shared `sync.RWMutex` introduces severe CPU cache line bouncing and lock contention when high-throughput services (100k+ RPS) access configuration on every request.
- **Inconsistent Partial Config Updates**: If a configuration update applies changes field by field, readers can observe an inconsistent hybrid state (e.g., new timeout paired with old endpoint address).

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

### Lock-Free Hot-Reload Sequence

```
Background Reloader                     ConfigHolder (atomic.Pointer)              Active Worker Goroutines (100k RPS)
        |                                             |                                             |
        |                                             |<-- Load() Lock-Free [Snapshot V1] ----------| (Uses V1)
        |                                             |                                             |
        |-- 1. Parse & Validate New Settings          |                                             |
        |-- 2. Construct ImmutableConfig (V2)         |                                             |
        |      (Defensively copy maps & slices)       |                                             |
        |                                             |                                             |
        |-- 3. Swap(Snapshot V2) -------------------->|                                             |
        |      (Atomic 64-bit pointer exchange)       |                                             |
        |                                             |<-- Load() Lock-Free [Snapshot V2] ----------| (Uses V2)
        |                                             |                                             |
        |<-- Success (Zero Downtime / No Locks) ------|                                             |
```

### Architectural State Machine (ASCII)

```
 +-------------------------------------------------------------------------+
 |                          IMMUTABLE CONFIG ARCHITECTURE                  |
 |                                                                         |
 |  ImmutableConfig Struct:                                                |
 |  - Unexported fields: env, port, features (map), endpoints ([]string)   |
 |  - Defensive Copying in Constructor:                                    |
 |      Clones map[string]bool and []string to isolate internal memory     |
 |  - Defensive Copying in Getters:                                        |
 |      Endpoints() returns a fresh copy of the slice                      |
 |                                                                         |
 |  ConfigHolder (Lock-Free Hot Reload):                                   |
 |  - current: atomic.Pointer[ImmutableConfig]                             |
 |  +-> Get() -> atomic.Load() [Wait-free, zero allocation, zero locks]    |
 |  +-> Swap(newCfg) -> atomic.Store() [Single atomic pointer flip]        |
 +-------------------------------------------------------------------------+
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Production Hardening Checklist
- **Defensively Copy Collections on Read and Write**: Always clone slices and maps both inside the constructor and within getter methods.
- **Use `atomic.Pointer` for Hot Reloading**: Replace traditional read-write mutexes (`sync.RWMutex`) with `atomic.Pointer` to eliminate lock contention on hot read paths.
- **Validate Before Swapping**: Fully parse and validate the new configuration snapshot before invoking `holder.Swap()`. If validation fails, keep the previous configuration active without disruption.
- **Cache Local Snapshot Pointers per Request**: In request middleware, load `cfg := holder.Get()` once at the start of the request and pass `cfg` throughout the request lifetime to guarantee internal consistency.

### Common Pitfalls & Anti-Patterns
- **Shallow Copying Complex Nested Structs**: Copying structs with inner pointer fields shallowly, allowing callers to mutate nested fields.
- **Holding Pointers Across Long Operations**: Storing a reference to a single `ImmutableConfig` snapshot for the entire lifetime of a long-lived worker rather than loading the latest snapshot via `holder.Get()` per task iteration.
- **Blocking Read Paths with Mutexes**: Using a global mutex on config access in high-throughput services (e.g., 100k RPS), introducing severe CPU cache-line bouncing and lock contention.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

The pattern is implemented in `patterns/02_config/immutable_config.go`.

### Core Types & Signatures

```go
package config

import (
	"sync/atomic"
	"time"
)

type AppSettings struct {
	Environment string
	Port        int
	Features    map[string]bool
	Endpoints   []string
	Timeout     time.Duration
}

type ImmutableConfig struct {
	// unexported fields: env, port, features, endpoints, timeout
}

func NewImmutableConfig(s AppSettings) *ImmutableConfig
func (c *ImmutableConfig) Environment() string
func (c *ImmutableConfig) Port() int
func (c *ImmutableConfig) Timeout() time.Duration
func (c *ImmutableConfig) FeatureEnabled(name string) bool
func (c *ImmutableConfig) Endpoints() []string

type ConfigHolder struct {
	// unexported field: current atomic.Pointer[ImmutableConfig]
}

func NewConfigHolder(initial *ImmutableConfig) *ConfigHolder
func (h *ConfigHolder) Get() *ImmutableConfig
func (h *ConfigHolder) Swap(newConfig *ImmutableConfig)
```

### Complete End-to-End Example

```go
package main

import (
	"log"
	"sync"
	"time"

	"patterns/02_config"
)

func main() {
	// 1. Initialize initial configuration snapshot
	initialSettings := config.AppSettings{
		Environment: "prod",
		Port:        8080,
		Features:    map[string]bool{"dark_mode": true, "beta_pricing": false},
		Endpoints:   []string{"https://api1.internal", "https://api2.internal"},
		Timeout:     5 * time.Second,
	}
	holder := config.NewConfigHolder(config.NewImmutableConfig(initialSettings))

	// 2. Simulate concurrent reader goroutines
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			cfg := holder.Get() // Lock-free atomic load
			log.Printf("[Worker %d] Using port %d, dark_mode=%v",
				workerID, cfg.Port(), cfg.FeatureEnabled("dark_mode"))
		}(i)
	}

	// 3. Hot-reload configuration atomically in background
	newSettings := initialSettings
	newSettings.Port = 9090
	newSettings.Features["beta_pricing"] = true
	holder.Swap(config.NewImmutableConfig(newSettings))

	log.Printf("[Hot-Reload] New config swapped. Active port is now: %d", holder.Get().Port())
	wg.Wait()
}
```
