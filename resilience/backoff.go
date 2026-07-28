package resilience

import (
	"math"
	"math/rand"
	"time"
)

const maxDuration = time.Duration(1<<63 - 1)

// Backoff interface 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type Backoff interface {
	Delay(attempt int) time.Duration
}

// BackoffFunc func 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type BackoffFunc func(attempt int) time.Duration

// Delay Delay 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - attempt: 현재 시도 번호다.
func (fn BackoffFunc) Delay(attempt int) time.Duration {
	return fn(attempt)
}

// NoBackoff NoBackoff 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
func NoBackoff() Backoff {
	return BackoffFunc(func(int) time.Duration {
		return 0
	})
}

// ConstantBackoff ConstantBackoff 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - delay: 고정 backoff로 사용할 시간이다.
func ConstantBackoff(delay time.Duration) Backoff {
	return BackoffFunc(func(int) time.Duration {
		if delay < 0 {
			return 0
		}
		return delay
	})
}

// ExponentialBackoff struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type ExponentialBackoff struct {
	InitialDelay time.Duration
	Multiplier   float64
	MaxDelay     time.Duration
	Jitter       float64
	Random       func() float64
}

// Delay Delay 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - attempt: 현재 시도 번호다.
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
