package advanced

import (
	"errors"
	"sync"
	"time"
)

var ErrShedded = errors.New("request shed: latency EMA exceeded SLA threshold")

// LatencyEMAShedder computes an exponential moving average (EMA) of response times and sheds traffic when response times degrade.
type LatencyEMAShedder struct {
	mu           sync.RWMutex
	targetSLA    time.Duration
	currentEMA   time.Duration
	decayFactor  float64 // e.g. 0.1
	maxInFlight  int
	currentInFlight int
}

func NewLatencyEMAShedder(targetSLA time.Duration, maxInFlight int) *LatencyEMAShedder {
	if targetSLA <= 0 {
		targetSLA = 100 * time.Millisecond
	}
	if maxInFlight <= 0 {
		maxInFlight = 100
	}

	return &LatencyEMAShedder{
		targetSLA:   targetSLA,
		currentEMA:  targetSLA / 2, // Start healthy
		decayFactor: 0.1,
		maxInFlight: maxInFlight,
	}
}

// Allow evaluates if a request should be admitted based on recent latency EMA and concurrency.
func (s *LatencyEMAShedder) Allow() (done func(), err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If latency EMA is double target SLA or concurrency saturated -> Shed load
	if s.currentEMA > s.targetSLA || s.currentInFlight >= s.maxInFlight {
		return nil, ErrShedded
	}

	s.currentInFlight++
	start := time.Now()

	return func() {
		s.recordLatency(time.Since(start))
	}, nil
}

func (s *LatencyEMAShedder) recordLatency(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentInFlight--

	// EMA formula: EMA_new = (current * decay) + (EMA_old * (1 - decay))
	newEMA := (float64(d) * s.decayFactor) + (float64(s.currentEMA) * (1.0 - s.decayFactor))
	s.currentEMA = time.Duration(newEMA)
}

func (s *LatencyEMAShedder) CurrentEMA() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentEMA
}
