package redisratelimit

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultNamespace   = "default"
	defaultKeyPrefix   = "bluetape:ratelimit"
	defaultMaxKeyBytes = 512
	minDefaultIdleTTL  = time.Minute
	tokenScale         = int64(1_000_000)
)

// Options Redis token bucket 설정이다.
type Options struct {
	// Client caller-owned Redis client다.
	Client redis.Cmdable
	// Namespace Redis key scope다.
	Namespace string
	// RatePerSecond 초당 채워지는 token 수다.
	RatePerSecond float64
	// Burst bucket 최대 token 수다.
	Burst int64
	// IdleTTL은 쓰지 않는 Redis bucket key 만료 시간이다.
	IdleTTL time.Duration
	// MaxKeyBytes logical key 최대 byte 수다.
	MaxKeyBytes int
}

type options struct {
	client              redis.Cmdable
	namespace           string
	ratePerSecond       float64
	rateMicrosPerSecond int64
	burst               int64
	burstMicros         int64
	idleTTL             time.Duration
	maxKeyBytes         int
}

func (o Options) normalize() (options, error) {
	if o.Client == nil {
		return options{}, fmt.Errorf("redis client must not be nil")
	}
	namespace := strings.TrimSpace(o.Namespace)
	if o.Namespace == "" {
		namespace = defaultNamespace
	}
	if namespace == "" {
		return options{}, fmt.Errorf("namespace must not be blank")
	}
	if o.RatePerSecond <= 0 || math.IsNaN(o.RatePerSecond) || math.IsInf(o.RatePerSecond, 0) {
		return options{}, fmt.Errorf("rate per second must be positive")
	}
	rateMicros, err := rateToMicros(o.RatePerSecond)
	if err != nil {
		return options{}, err
	}
	if o.Burst <= 0 {
		return options{}, fmt.Errorf("burst must be positive")
	}
	burstMicros, err := tokensToMicros(o.Burst)
	if err != nil {
		return options{}, fmt.Errorf("burst: %w", err)
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

	maxKeyBytes := o.MaxKeyBytes
	if maxKeyBytes == 0 {
		maxKeyBytes = defaultMaxKeyBytes
	}
	if maxKeyBytes < 0 {
		return options{}, fmt.Errorf("max key bytes must not be negative")
	}

	return options{
		client:              o.Client,
		namespace:           namespace,
		ratePerSecond:       o.RatePerSecond,
		rateMicrosPerSecond: rateMicros,
		burst:               o.Burst,
		burstMicros:         burstMicros,
		idleTTL:             idleTTL,
		maxKeyBytes:         maxKeyBytes,
	}, nil
}

func rateToMicros(ratePerSecond float64) (int64, error) {
	if ratePerSecond > float64(math.MaxInt64)/float64(tokenScale) {
		return 0, fmt.Errorf("rate per second is too large")
	}
	micros := int64(math.Round(ratePerSecond * float64(tokenScale)))
	if micros <= 0 {
		return 0, fmt.Errorf("rate per second is too small")
	}
	return micros, nil
}

func tokensToMicros(tokens int64) (int64, error) {
	if tokens <= 0 {
		return 0, fmt.Errorf("tokens must be positive")
	}
	if tokens > math.MaxInt64/tokenScale {
		return 0, fmt.Errorf("tokens are too large")
	}
	return tokens * tokenScale, nil
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
