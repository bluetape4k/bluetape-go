package redisfory

import (
	"context"
	"errors"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

type valueCodec[V any] interface {
	Serialize(V) ([]byte, error)
	Deserialize([]byte) (V, error)
}

type cacheState[V any] struct {
	codec valueCodec[V]
}

type commandClient interface {
	GetRange(context.Context, string, int64, int64) *redis.StringCmd
	Exists(context.Context, ...string) *redis.IntCmd
	Set(context.Context, string, any, time.Duration) *redis.StatusCmd
	Del(context.Context, ...string) *redis.IntCmd
}

// Set Set 공개 API의 동작을 수행하며 Redis 값 캐시의 serialization, TTL, backend ownership 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - logicalKey: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//   - value: 직렬화하거나 cache에 보관할 값이다. nil, zero value, aliasing 의미는 serializer/cache 계약을 따른다.
//   - ttl: cache entry의 유효 시간이다. zero, 음수, 만료 의미는 옵션과 TTL 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (c *ValueCache[V]) Set(ctx context.Context, logicalKey string, value V, ttl time.Duration) error {
	ctx = normalizeContext(ctx)
	if err := c.validateInitialized("set"); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := c.key(logicalKey)
	if err != nil {
		return err
	}
	if err := btredis.ValidateTTL("value ttl", ttl); err != nil {
		return err
	}
	raw, err := c.state.codec.Serialize(value)
	if err != nil {
		return mapRuntimeError("set", c.profile, err)
	}
	if len(raw) > c.maxPayload {
		return newCacheError("set", c.profile, ReasonPayloadTooLarge, nil)
	}
	encoded := wrap(c.profile, c.generation, raw)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := c.client.Set(ctx, key.Value, encoded, ttl).Err(); err != nil {
		return c.operationError(ctx, "set", key.Value)
	}
	return nil
}

// Get Get 공개 API의 동작을 수행하며 Redis 값 캐시의 serialization, TTL, backend ownership 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - logicalKey: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (c *ValueCache[V]) Get(ctx context.Context, logicalKey string) (V, error) {
	var value V
	ctx = normalizeContext(ctx)
	if err := c.validateInitialized("get"); err != nil {
		return value, err
	}
	if err := ctx.Err(); err != nil {
		return value, err
	}
	key, err := c.key(logicalKey)
	if err != nil {
		return value, err
	}
	maxEnvelope := envelopeHeaderSize + c.maxPayload
	encoded, err := c.client.GetRange(ctx, key.Value, 0, int64(maxEnvelope)).Bytes()
	if errors.Is(err, redis.Nil) {
		return value, cache.ErrCacheMiss
	}
	if err != nil {
		return value, c.operationError(ctx, "get", key.Value)
	}
	if len(encoded) == 0 {
		exists, existsErr := c.client.Exists(ctx, key.Value).Result()
		if existsErr != nil {
			return value, c.operationError(ctx, "get", key.Value)
		}
		if exists == 0 {
			return value, cache.ErrCacheMiss
		}
	}
	if len(encoded) > maxEnvelope {
		return value, newCacheError("get", c.profile, ReasonPayloadTooLarge, nil)
	}
	if err := ctx.Err(); err != nil {
		return value, err
	}
	raw, err := unwrap(c.profile, c.generation, encoded, c.maxPayload)
	if err != nil {
		return value, err
	}
	value, err = c.state.codec.Deserialize(raw)
	if err != nil {
		return value, mapRuntimeError("get", c.profile, err)
	}
	return value, nil
}

// Delete Delete 공개 API의 동작을 수행하며 Redis 값 캐시의 serialization, TTL, backend ownership 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - logicalKey: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (c *ValueCache[V]) Delete(ctx context.Context, logicalKey string) error {
	ctx = normalizeContext(ctx)
	if err := c.validateInitialized("delete"); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := c.key(logicalKey)
	if err != nil {
		return err
	}
	if err := c.client.Del(ctx, key.Value).Err(); err != nil {
		return c.operationError(ctx, "delete", key.Value)
	}
	return nil
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func (c *ValueCache[V]) validateInitialized(operation string) error {
	if c == nil || c.client == nil || c.state == nil || c.state.codec == nil {
		profile := Profile("")
		if c != nil {
			profile = c.profile
		}
		return newCacheError(operation, profile, ReasonUninitialized, nil)
	}
	return nil
}

func (c *ValueCache[V]) operationError(ctx context.Context, operation, rawKey string) error {
	cause := errProviderFailed
	if contextErr := ctx.Err(); contextErr != nil {
		cause = errors.Join(errProviderFailed, contextErr)
	}
	return btredis.NewOpError(btredis.OpLabels{
		Family:    "redisfory",
		Operation: operation,
	}, rawKey, cause)
}
