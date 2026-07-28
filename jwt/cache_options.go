package jwt

import (
	"crypto/rand"
	"encoding/hex"
	"reflect"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
)

const (
	defaultProviderCacheMaxTTL    = 5 * time.Minute
	defaultProviderCacheKeyPrefix = "jwt:provider-cache:v1"
	providerCacheScopeBytes       = 16
)

// CacheOption 은 JWT provider cache adapter 설정 option이다.
type CacheOption func(*cacheConfig) error

type cacheConfig struct {
	maxTTL     time.Duration
	keyPrefix  string
	trustScope string
	now        func() time.Time
	customNow  bool
}

func normalizeCacheConfig(options []CacheOption) (cacheConfig, error) {
	cfg := cacheConfig{
		maxTTL:    defaultProviderCacheMaxTTL,
		keyPrefix: defaultProviderCacheKeyPrefix,
		now:       time.Now,
	}
	scope, err := randomCacheTrustScope()
	if err != nil {
		return cfg, err
	}
	cfg.trustScope = scope
	for _, option := range options {
		if option == nil {
			return cfg, OptionError{Option: "cache_option", Err: errorsNew("must not be nil")}
		}
		if err := option(&cfg); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

// WithCacheMaxTTL 은 성공한 JWT parse 결과의 최대 cache TTL을 지정한다.
func WithCacheMaxTTL(ttl time.Duration) CacheOption {
	return func(cfg *cacheConfig) error {
		if ttl <= 0 {
			return OptionError{Option: "cache_max_ttl", Err: errorsNew("must be positive")}
		}
		cfg.maxTTL = ttl
		return nil
	}
}

// WithCacheKeyPrefix cache key의 비밀이 아닌 namespace prefix를 지정한다.
func WithCacheKeyPrefix(prefix string) CacheOption {
	return func(cfg *cacheConfig) error {
		if err := validateCacheText("cache_key_prefix", prefix); err != nil {
			return err
		}
		cfg.keyPrefix = prefix
		return nil
	}
}

// WithCacheTrustScope provider, tenant, key namespace에 대한 명시적 private scope를 지정한다.
func WithCacheTrustScope(scope string) CacheOption {
	return func(cfg *cacheConfig) error {
		if err := validateCacheText("cache_trust_scope", scope); err != nil {
			return err
		}
		cfg.trustScope = scope
		return nil
	}
}

// WithCacheClock 은 cache TTL과 hit 재검증에 사용할 clock을 지정한다.
func WithCacheClock(now func() time.Time) CacheOption {
	return func(cfg *cacheConfig) error {
		if now == nil {
			return OptionError{Option: "cache_clock", Err: errorsNew("must not be nil")}
		}
		cfg.now = now
		cfg.customNow = true
		return nil
	}
}

func validateCacheText(option string, value string) error {
	if value == "" {
		return OptionError{Option: option, Err: errorsNew("must not be empty")}
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return OptionError{Option: option, Err: errorsNew("must not contain control characters")}
		}
	}
	return nil
}

func randomCacheTrustScope() (string, error) {
	buf := make([]byte, providerCacheScopeBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", OptionError{Option: "cache_trust_scope", Err: err}
	}
	return hex.EncodeToString(buf), nil
}

func requireReaderCache(c cache.Cache[string, *Reader]) error {
	if c == nil {
		return OptionError{Option: "cache", Err: errorsNew("must not be nil")}
	}
	value := reflect.ValueOf(c)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return OptionError{Option: "cache", Err: errorsNew("must not be nil")}
		}
	}
	return nil
}

func minPositiveDuration(first time.Duration, rest ...time.Duration) time.Duration {
	best := first
	for _, candidate := range rest {
		if candidate <= 0 {
			return 0
		}
		if candidate < best {
			best = candidate
		}
	}
	return best
}
