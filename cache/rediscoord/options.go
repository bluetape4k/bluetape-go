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

// Options Redis 조정, stampede 방지, codec envelope에서 사용하는 구조체다.
type Options[V any] struct {
	// Client 필수 Redis coordination backend다.
	Client redis.Cmdable
	// Cache 감쌀 필수 local 또는 near LoadingCache다.
	Cache cache.LoadingCache[string, V]
	// Namespace coordination key 범위를 나눈다.
	Namespace string
	// Codec loader result payload를 encoding/decoding하는 필수 codec이다.
	Codec Codec[V]
	// LockTTL load owner lease 유지 시간이다.
	LockTTL time.Duration
	// ResultTTL 공유 result envelope 보존 시간이다.
	ResultTTL time.Duration
	// PollInterval waiter가 result를 재확인하는 polling 간격이다.
	PollInterval time.Duration
	// MaxResultBytes encoded Redis result envelope byte 상한이다. zero는 제한 없음을 뜻한다.
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
