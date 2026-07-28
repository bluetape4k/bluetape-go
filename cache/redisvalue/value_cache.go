package redisvalue

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/bluetape4k/bluetape-go/serialization"
	"github.com/redis/go-redis/v9"
)

type commandClient interface {
	ReadBounded(context.Context, string, int64) ([]byte, bool, error)
	Set(context.Context, string, any, time.Duration) *redis.StatusCmd
	Del(context.Context, ...string) *redis.IntCmd
	Scan(context.Context, uint64, string, int64) *redis.ScanCmd
	Unlink(context.Context, ...string) *redis.IntCmd
}

type redisCommandClient struct {
	*redis.Client
}

// ReadBounded tiered Redis value cache의 local/remote ownership, TTL, clear coordination 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//   - end: ReadBounded에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (c *redisCommandClient) ReadBounded(ctx context.Context, key string, end int64) ([]byte, bool, error) {
	encoded, err := c.GetRange(ctx, key, 0, end).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if len(encoded) > 0 {
		return encoded, true, nil
	}

	var reread *redis.StringCmd
	var exists *redis.IntCmd
	if _, err := c.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		reread = pipe.GetRange(ctx, key, 0, end)
		exists = pipe.Exists(ctx, key)
		return nil
	}); err != nil {
		return nil, false, err
	}
	encoded, err = reread.Bytes()
	if err != nil {
		return nil, false, err
	}
	present, err := exists.Result()
	if err != nil {
		return nil, false, err
	}
	return encoded, present != 0, nil
}

// ValueOptions tiered Redis value cache의 local/remote ownership, TTL, clear coordination에서 사용하는 구조체다.
type ValueOptions[V any] struct {
	// Client 호출자가 소유한 direct stable writable-primary Redis client다.
	Client *redis.Client
	// Namespace identifies one exclusive tenant, schema, and clear trust domain.
	Namespace string
	// Serializer clone 없이 보관되므로 concurrent call을 지원해야 한다.
	Serializer serialization.Serializer[V]
	// Config 생성 시 복사된다. nil이면 DefaultConfig().Value를 사용한다.
	Config *ValueConfig
}

// ValueCache tiered Redis value cache의 local/remote ownership, TTL, clear coordination에서 사용하는 구조체다.
type ValueCache[V any] struct {
	client     commandClient
	serializer serialization.Serializer[V]
	keys       btredis.KeyBuilder
	namespace  string
	config     ValueConfig
}

// NewValueCache tiered Redis value cache의 local/remote ownership, TTL, clear coordination에 사용할 값을 생성한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func NewValueCache[V any](options ValueOptions[V]) (*ValueCache[V], error) {
	if options.Client == nil || nilInterface(options.Serializer) {
		return nil, newCacheError("configure", ReasonConfiguration, "", nil)
	}
	config := DefaultConfig().Value
	if options.Config != nil {
		config = *options.Config
	}
	if err := validateValueConfig(config); err != nil {
		return nil, err
	}
	builder, err := newValueKeyBuilder(options.Namespace)
	if err != nil {
		return nil, newCacheError("configure", ReasonConfiguration, "", err)
	}
	return &ValueCache[V]{
		client:     &redisCommandClient{Client: options.Client},
		serializer: options.Serializer,
		keys:       builder,
		namespace:  options.Namespace,
		config:     config,
	}, nil
}

// Get tiered Redis value cache의 local/remote ownership, TTL, clear coordination에서 필요한 값을 조회한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - logicalKey: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (c *ValueCache[V]) Get(ctx context.Context, logicalKey string) (V, error) {
	var zero V
	ctx = normalizeContext(ctx)
	if err := c.validateInitialized("get"); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	key, err := c.key(logicalKey)
	if err != nil {
		return zero, err
	}
	encoded, present, err := c.client.ReadBounded(ctx, key.Value, int64(c.config.MaxValueBytes))
	if err != nil {
		return zero, c.providerError("get", key.RedactedID, err, false)
	}
	if !present {
		return zero, cache.ErrCacheMiss
	}
	if encoded == nil {
		encoded = []byte{}
	}
	if len(encoded) > c.config.MaxValueBytes {
		return zero, newCacheError("get", ReasonPayloadTooLarge, key.RedactedID, nil)
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	value, err := c.serializer.Unmarshal(encoded)
	if err != nil {
		return zero, newCacheError("get", ReasonInvalidPayload, key.RedactedID, err)
	}
	return value, nil
}

// Set tiered Redis value cache의 local/remote ownership, TTL, clear coordination의 상태를 변경한다.
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
	if err := validateEntryTTL(ttl); err != nil {
		return err
	}
	encoded, err := c.serializer.Marshal(value)
	if err != nil {
		return newCacheError("set", ReasonSerialization, key.RedactedID, err)
	}
	if len(encoded) > c.config.MaxValueBytes {
		return newCacheError("set", ReasonPayloadTooLarge, key.RedactedID, nil)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := c.client.Set(ctx, key.Value, encoded, normalizeWireTTL(ttl)).Err(); err != nil {
		return c.providerError("set", key.RedactedID, err, true)
	}
	return nil
}

// SetDefault tiered Redis value cache의 local/remote ownership, TTL, clear coordination 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//   - value: 직렬화하거나 cache에 보관할 값이다. nil, zero value, aliasing 의미는 serializer/cache 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (c *ValueCache[V]) SetDefault(ctx context.Context, key string, value V) error {
	if err := c.validateInitialized("set-default"); err != nil {
		return err
	}
	return c.Set(ctx, key, value, c.config.RemoteTTL)
}

// Delete tiered Redis value cache의 local/remote ownership, TTL, clear coordination의 상태를 변경한다.
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
		return c.providerError("delete", key.RedactedID, err, true)
	}
	return nil
}

func (c *ValueCache[V]) key(logicalKey string) (btredis.Key, error) {
	if err := c.validateInitialized("key"); err != nil {
		return btredis.Key{}, err
	}
	if err := validateLogicalKey(logicalKey); err != nil {
		return btredis.Key{}, err
	}
	return c.keys.LogicalKey(logicalKey)
}

func (c *ValueCache[V]) validateInitialized(operation string) error {
	if c == nil || nilInterface(c.client) || nilInterface(c.serializer) || c.namespace == "" {
		return newCacheError(operation, ReasonUninitialized, "", nil)
	}
	return nil
}

func (c *ValueCache[V]) providerError(operation, keyID string, cause error, commitUnknown bool) error {
	opErr := c.operationError(operation, keyID, cause, commitUnknown)
	return newCacheError(operation, ReasonProviderFailure, keyID, opErr)
}

func (c *ValueCache[V]) operationError(operation, keyID string, cause error, commitUnknown bool) error {
	if commitUnknown {
		cause = errors.Join(cause, btredis.ErrCommitUnknown)
	}
	return btredis.NewOpErrorWithRedactedKey(btredis.OpLabels{
		Family:    "redisvalue",
		Operation: operation,
	}, keyID, cause)
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
