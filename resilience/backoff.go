package resilience

import (
	"math"
	"math/rand"
	"time"
)

const maxDuration = time.Duration(1<<63 - 1)

// Backoff returns the delay before the next attempt. The attempt argument is
// one-based and refers to the attempt that just failed.
type Backoff interface {
	Delay(attempt int) time.Duration
}

// BackoffFunc adapts a function into a Backoff.
type BackoffFunc func(attempt int) time.Duration

// Delay returns fn(attempt).
func (fn BackoffFunc) Delay(attempt int) time.Duration {
	return fn(attempt)
}

// NoBackoff returns zero delay between attempts.
func NoBackoff() Backoff {
	return BackoffFunc(func(int) time.Duration {
		return 0
	})
}

// ConstantBackoff returns the same delay for every retry.
func ConstantBackoff(delay time.Duration) Backoff {
	return BackoffFunc(func(int) time.Duration {
		if delay < 0 {
			return 0
		}
		return delay
	})
}

// ExponentialBackoff returns exponentially growing delays with optional jitter.
type ExponentialBackoff struct {
	InitialDelay time.Duration
	Multiplier   float64
	MaxDelay     time.Duration
	Jitter       float64
	Random       func() float64
}

// Delay returns the delay for a failed one-based attempt.
func (b ExponentialBackoff) Delay(attempt int) time.Duration {
	if attempt <= 0 || b.InitialDelay <= 0 {
		return 0
	}

	multiplier := b.Multiplier
	if multiplier < 1 {
		multiplier = 1
	}

	delay := float64(b.InitialDelay)
	if attempt > 1 {
		delay *= math.Pow(multiplier, float64(attempt-1))
	}

	if b.MaxDelay > 0 && delay > float64(b.MaxDelay) {
		delay = float64(b.MaxDelay)
	}

	jitter := b.Jitter
	if jitter < 0 {
		jitter = 0
	}
	if jitter > 1 {
		jitter = 1
	}
	if jitter > 0 {
		random := b.Random
		if random == nil {
			random = rand.Float64
		}
		randomValue := random()
		if randomValue < 0 {
			randomValue = 0
		}
		if randomValue > 1 {
			randomValue = 1
		}
		factor := 1 - jitter + (2 * jitter * randomValue)
		delay *= factor
	}

	if delay <= 0 {
		return 0
	}
	if delay > float64(maxDuration) {
		return maxDuration
	}
	return time.Duration(delay)
}
