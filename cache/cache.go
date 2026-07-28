package cache

import (
	"context"
	"fmt"
	"time"
)

// Loader는 func 공개 타입이며 cache key, miss, TTL, serialization 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Loader[K comparable, V any] func(context.Context, K) (V, error)

// Cache는 interface 공개 타입이며 cache key, miss, TTL, serialization 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Cache[K comparable, V any] interface {
	Get(ctx context.Context, key K) (V, error)
	Set(ctx context.Context, key K, value V, ttl time.Duration) error
	Delete(ctx context.Context, key K) error
	Clear(ctx context.Context) error
}

// LoadingCache는 interface 공개 타입이며 cache key, miss, TTL, serialization 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
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
