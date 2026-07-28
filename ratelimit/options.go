package ratelimit

import (
	"fmt"
	"math"
	"time"
)

const minDefaultIdleTTL = time.Minute

// Options struct 공개 타입이며 token bucket, limiter option, HTTP boundary, result quota 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Options struct {
	// RatePerSecond 초당 채워지는 token 수다.
	RatePerSecond float64
	// Burst bucket 최대 token 수다.
	Burst int64
	// IdleTTL은 쓰지 않는 key 상태를 제거하는 시간이다.
	IdleTTL time.Duration
}

type options struct {
	ratePerSecond float64
	burst         int64
	idleTTL       time.Duration
}

func (o Options) normalize() (options, error) {
	if o.RatePerSecond <= 0 || math.IsNaN(o.RatePerSecond) || math.IsInf(o.RatePerSecond, 0) {
		return options{}, fmt.Errorf("rate per second must be positive")
	}
	if o.Burst <= 0 {
		return options{}, fmt.Errorf("burst must be positive")
	}
	if o.IdleTTL < 0 {
		return options{}, fmt.Errorf("idle ttl must not be negative")
	}

	fullRefill := refillDuration(o.Burst, o.RatePerSecond)
	idleTTL := o.IdleTTL
	if idleTTL == 0 {
		idleTTL = fullRefill * 2
		if idleTTL < minDefaultIdleTTL {
			idleTTL = minDefaultIdleTTL
		}
	}
	if idleTTL < fullRefill {
		return options{}, fmt.Errorf("idle ttl must be at least one full refill duration")
	}

	return options{
		ratePerSecond: o.RatePerSecond,
		burst:         o.Burst,
		idleTTL:       idleTTL,
	}, nil
}

func refillDuration(tokens int64, ratePerSecond float64) time.Duration {
	if tokens <= 0 || ratePerSecond <= 0 {
		return 0
	}
	seconds := float64(tokens) / ratePerSecond
	if seconds >= float64(math.MaxInt64)/float64(time.Second) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(math.Ceil(seconds * float64(time.Second)))
}
