package rediscoord

import (
	"fmt"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	"github.com/redis/go-redis/v9"
)

const (
	defaultNamespace    = "default"
	defaultKeyPrefix    = "bluetape:cache:coord"
	defaultLockTTL      = 5 * time.Second
	defaultResultTTL    = time.Second
	defaultPollInterval = 10 * time.Millisecond
	unlockTimeout       = time.Second
)

// Options configures a StampedeCache.
type Options[V any] struct {
	// Client is the required Redis coordination backend.
	Client redis.Cmdable
	// Cache is the required local or near LoadingCache to wrap.
	Cache cache.LoadingCache[string, V]
	// Namespace scopes coordination keys.
	Namespace string
	// Codec is the required loader-result payload codec.
	Codec Codec[V]
	// LockTTL is the load-owner lease duration.
	LockTTL time.Duration
	// ResultTTL is the shared result-envelope retention duration.
	ResultTTL time.Duration
	// PollInterval is the waiter polling interval.
	PollInterval time.Duration
	// MaxResultBytes bounds the encoded Redis result envelope. Zero is unlimited.
	MaxResultBytes int
}

type config[V any] struct {
	client         redis.Cmdable
	cache          cache.LoadingCache[string, V]
	namespace      string
	codec          Codec[V]
	lockTTL        time.Duration
	resultTTL      time.Duration
	pollInterval   time.Duration
	maxResultBytes int
	keyPrefix      string
}

func normalizeOptions[V any](options Options[V]) (config[V], error) {
	if options.Client == nil {
		return config[V]{}, fmt.Errorf("redis client must not be nil")
	}
	if options.Cache == nil {
		return config[V]{}, fmt.Errorf("cache must not be nil")
	}
	if options.Codec == nil {
		return config[V]{}, fmt.Errorf("codec must not be nil")
	}
	if options.MaxResultBytes < 0 {
		return config[V]{}, fmt.Errorf("max result bytes must not be negative")
	}

	namespace := options.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	} else if strings.TrimSpace(namespace) == "" {
		return config[V]{}, fmt.Errorf("namespace must not be blank")
	}

	lockTTL, err := normalizeDuration(options.LockTTL, defaultLockTTL, "lock ttl")
	if err != nil {
		return config[V]{}, err
	}
	resultTTL, err := normalizeDuration(options.ResultTTL, defaultResultTTL, "result ttl")
	if err != nil {
		return config[V]{}, err
	}
	pollInterval, err := normalizeDuration(options.PollInterval, defaultPollInterval, "poll interval")
	if err != nil {
		return config[V]{}, err
	}

	return config[V]{
		client:         options.Client,
		cache:          options.Cache,
		namespace:      namespace,
		codec:          options.Codec,
		lockTTL:        lockTTL,
		resultTTL:      resultTTL,
		pollInterval:   pollInterval,
		maxResultBytes: options.MaxResultBytes,
		keyPrefix:      defaultKeyPrefix + ":" + namespace,
	}, nil
}

func normalizeDuration(value time.Duration, fallback time.Duration, name string) (time.Duration, error) {
	if value < 0 {
		return 0, fmt.Errorf("%s must not be negative", name)
	}
	if value == 0 {
		return fallback, nil
	}
	return value, nil
}
