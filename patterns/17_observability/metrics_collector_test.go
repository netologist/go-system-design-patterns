package observability_test

import (
	"testing"

	observability "system-design-patterns/patterns/17_observability"
)

func TestMetrics_CounterAndGauge(t *testing.T) {
	counter := &observability.Counter{}
	counter.Inc()
	counter.Add(4)
	if counter.Value() != 5 {
		t.Errorf("expected counter=5, got: %d", counter.Value())
	}

	gauge := &observability.Gauge{}
	gauge.Set(100)
	gauge.Inc()
	gauge.Dec()
	if gauge.Value() != 100 {
		t.Errorf("expected gauge=100, got: %d", gauge.Value())
	}
}

func TestMetrics_HistogramPercentiles(t *testing.T) {
	hist := observability.NewHistogram()

	// Insert 100 samples: 1.0 to 100.0
	for i := 1; i <= 100; i++ {
		hist.Observe(float64(i))
	}

	p50 := hist.Percentile(50.0)
	if p50 < 49.0 || p50 > 51.0 {
		t.Errorf("expected p50 around 50.0, got: %f", p50)
	}

	p95 := hist.Percentile(95.0)
	if p95 < 94.0 || p95 > 96.0 {
		t.Errorf("expected p95 around 95.0, got: %f", p95)
	}

	p99 := hist.Percentile(99.0)
	if p99 < 98.0 || p99 > 100.0 {
		t.Errorf("expected p99 around 99.0, got: %f", p99)
	}
}
