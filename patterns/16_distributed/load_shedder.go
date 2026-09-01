package distributed

import (
	"errors"
	"sync/atomic"
)

var ErrLoadSheddingDrop = errors.New("request dropped by load shedder: server capacity saturated")

type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityCritical
)

// AdaptiveLoadShedder protects system stability by rejecting non-critical traffic under high load.
type AdaptiveLoadShedder struct {
	maxInFlight      int64
	currentInFlight  atomic.Int64
	lowThreshold     int64 // Above this, drop PriorityLow
	normalThreshold  int64 // Above this, drop PriorityNormal
}

func NewAdaptiveLoadShedder(maxInFlight int64) *AdaptiveLoadShedder {
	if maxInFlight <= 0 {
		maxInFlight = 100
	}

	return &AdaptiveLoadShedder{
		maxInFlight:     maxInFlight,
		lowThreshold:    int64(float64(maxInFlight) * 0.70), // 70% capacity
		normalThreshold: int64(float64(maxInFlight) * 0.90), // 90% capacity
	}
}

// Allow evaluates if a request with given priority can proceed under current load.
func (s *AdaptiveLoadShedder) Allow(p Priority) (releaseFunc func(), err error) {
	current := s.currentInFlight.Add(1)

	shouldDrop := false
	switch p {
	case PriorityLow:
		if current > s.lowThreshold {
			shouldDrop = true
		}
	case PriorityNormal:
		if current > s.normalThreshold {
			shouldDrop = true
		}
	case PriorityCritical:
		if current > s.maxInFlight {
			shouldDrop = true
		}
	}

	if shouldDrop {
		s.currentInFlight.Add(-1)
		return nil, ErrLoadSheddingDrop
	}

	return func() {
		s.currentInFlight.Add(-1)
	}, nil
}

func (s *AdaptiveLoadShedder) InFlight() int64 {
	return s.currentInFlight.Load()
}
