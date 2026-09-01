package advanced

import (
	"math"
	"math/rand/v2"
	"time"
)

// FullJitter: sleep = rand(0, min(max, base * 2^attempt))
func FullJitter(attempt int, base, max time.Duration) time.Duration {
	if attempt <= 0 {
		return 0
	}
	capVal := float64(base) * math.Pow(2, float64(attempt-1))
	if capVal > float64(max) {
		capVal = float64(max)
	}
	return time.Duration(rand.Float64() * capVal)
}

// EqualJitter: half = cap / 2; sleep = half + rand(0, half)
func EqualJitter(attempt int, base, max time.Duration) time.Duration {
	if attempt <= 0 {
		return 0
	}
	capVal := float64(base) * math.Pow(2, float64(attempt-1))
	if capVal > float64(max) {
		capVal = float64(max)
	}
	half := capVal / 2.0
	return time.Duration(half + rand.Float64()*half)
}

// DecorrelatedJitter: sleep = min(max, rand(base, prevSleep * 3))
func DecorrelatedJitter(prevSleep, base, max time.Duration) time.Duration {
	if prevSleep < base {
		prevSleep = base
	}
	low := float64(base)
	high := float64(prevSleep * 3)

	res := low + rand.Float64()*(high-low)
	if res > float64(max) {
		res = float64(max)
	}
	return time.Duration(res)
}
