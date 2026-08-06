package sqlratelimit

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	defaultNamespace         = "default"
	defaultMaxKeyBytes       = 512
	maxMaxKeyBytes           = 1024
	maxNamespaceBytes        = 128
	minDefaultIdleTTL        = time.Minute
	tokenScale         int64 = 1_000_000
)

// Options configures a PostgreSQL token bucket.
type Options struct {
	Namespace     string
	RatePerSecond float64
	Burst         int64
	IdleTTL       time.Duration
	MaxKeyBytes   int
}

type options struct {
	namespace           []byte
	rateMicrosPerSecond int64
	burst               int64
	burstMicros         int64
	idleTTLMicros       int64
	maxKeyBytes         int
}

func (o Options) normalize() (options, error) {
	namespace := strings.TrimSpace(o.Namespace)
	if o.Namespace == "" {
		namespace = defaultNamespace
	}
	if namespace == "" {
		return options{}, errors.New("namespace must not be blank")
	}
	if len(namespace) > maxNamespaceBytes {
		return options{}, errors.New("namespace exceeds maximum bytes")
	}
	rateMicros, err := rateToMicros(o.RatePerSecond)
	if err != nil {
		return options{}, err
	}
	burstMicros, err := tokensToMicros(o.Burst)
	if err != nil {
		return options{}, fmt.Errorf("burst: %w", err)
	}
	if o.IdleTTL < 0 {
		return options{}, errors.New("idle ttl must not be negative")
	}
	fullRefill := refillDuration(o.Burst, o.RatePerSecond)
	idleTTL := o.IdleTTL
	if idleTTL == 0 {
		idleTTL = saturatedDouble(fullRefill)
		if idleTTL < minDefaultIdleTTL {
			idleTTL = minDefaultIdleTTL
		}
	}
	if idleTTL < fullRefill {
		return options{}, errors.New("idle ttl must be at least one full refill duration")
	}
	idleTTLMicros, err := durationMicrosCeil(idleTTL)
	if err != nil {
		return options{}, fmt.Errorf("idle ttl: %w", err)
	}
	maxKeyBytes := o.MaxKeyBytes
	if maxKeyBytes == 0 {
		maxKeyBytes = defaultMaxKeyBytes
	}
	if maxKeyBytes < 1 || maxKeyBytes > maxMaxKeyBytes {
		return options{}, fmt.Errorf("max key bytes must be between 1 and %d", maxMaxKeyBytes)
	}
	return options{
		namespace:           []byte(namespace),
		rateMicrosPerSecond: rateMicros,
		burst:               o.Burst,
		burstMicros:         burstMicros,
		idleTTLMicros:       idleTTLMicros,
		maxKeyBytes:         maxKeyBytes,
	}, nil
}

func (o options) normalizeKey(key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", errors.New("key must not be blank")
	}
	if len(key) > o.maxKeyBytes {
		return "", errors.New("key exceeds maximum bytes")
	}
	return key, nil
}

func rateToMicros(rate float64) (int64, error) {
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return 0, errors.New("rate per second must be positive and finite")
	}
	if rate > float64(math.MaxInt64)/float64(tokenScale) {
		return 0, errors.New("rate per second is too large")
	}
	micros := int64(math.Round(rate * float64(tokenScale)))
	if micros <= 0 {
		return 0, errors.New("rate per second is too small")
	}
	return micros, nil
}

func tokensToMicros(tokens int64) (int64, error) {
	if tokens <= 0 {
		return 0, errors.New("tokens must be positive")
	}
	if tokens > math.MaxInt64/tokenScale {
		return 0, errors.New("tokens are too large")
	}
	return tokens * tokenScale, nil
}

func refillDuration(tokens int64, rate float64) time.Duration {
	seconds := float64(tokens) / rate
	if seconds >= float64(math.MaxInt64)/float64(time.Second) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(math.Ceil(seconds * float64(time.Second)))
}

func saturatedDouble(value time.Duration) time.Duration {
	if value > time.Duration(math.MaxInt64)/2 {
		return time.Duration(math.MaxInt64)
	}
	return value * 2
}

func durationMicrosCeil(value time.Duration) (int64, error) {
	if value <= 0 {
		return 0, errors.New("duration must be positive")
	}
	micros := value / time.Microsecond
	if value%time.Microsecond != 0 {
		micros++
	}
	return int64(micros), nil
}
