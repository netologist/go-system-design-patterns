package observability

import (
	"sort"
	"sync"
	"sync/atomic"
)

// Counter represents a monotonically increasing 64-bit integer counter.
type Counter struct {
	val atomic.Int64
}

func (c *Counter) Inc()                  { c.val.Add(1) }
func (c *Counter) Add(delta int64)       { c.val.Add(delta) }
func (c *Counter) Value() int64          { return c.val.Load() }

// Gauge represents a value that can go up and down.
type Gauge struct {
	val atomic.Int64
}

func (g *Gauge) Set(val int64)           { g.val.Store(val) }
func (g *Gauge) Inc()                    { g.val.Add(1) }
func (g *Gauge) Dec()                    { g.val.Add(-1) }
func (g *Gauge) Value() int64            { return g.val.Load() }

// Histogram tracks distribution of floating point samples and computes percentiles.
type Histogram struct {
	mu      sync.RWMutex
	samples []float64
}

func NewHistogram() *Histogram {
	return &Histogram{
		samples: make([]float64, 0, 1000),
	}
}

func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.samples = append(h.samples, value)
}

// Percentile calculates the p-th percentile (e.g. 50.0 for p50, 95.0 for p95, 99.0 for p99).
func (h *Histogram) Percentile(p float64) float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.samples) == 0 {
		return 0
	}

	sorted := make([]float64, len(h.samples))
	copy(sorted, h.samples)
	sort.Float64s(sorted)

	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}

	idx := int(float64(len(sorted)-1) * (p / 100.0))
	return sorted[idx]
}
