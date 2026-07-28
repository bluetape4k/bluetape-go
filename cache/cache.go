package cache

import (
	"context"
	"fmt"
	"time"
)

// Loader cache miss 값을 계산한다.
type Loader[K comparable, V any] func(context.Context, K) (V, error)

// Cache context-aware key-value cache 계약이다.
type Cache[K comparable, V any] interface {
	Get(ctx context.Context, key K) (V, error)
	Set(ctx context.Context, key K, value V, ttl time.Duration) error
	Delete(ctx context.Context, key K) error
	Clear(ctx context.Context) error
}

// LoadingCache miss 시 로더로 값을 채우는 cache 계약이다.
type LoadingCache[K comparable, V any] interface {
	Cache[K, V]
	GetOrLoad(ctx context.Context, key K, ttl time.Duration, loader Loader[K, V]) (V, error)
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func validateTTL(ttl time.Duration) error {
	if ttl < 0 {
		return fmt.Errorf("ttl must not be negative")
	}
	return nil
}
