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
	GetRange(context.Context, string, int64, int64) *redis.StringCmd
	Exists(context.Context, ...string) *redis.IntCmd
	Set(context.Context, string, any, time.Duration) *redis.StatusCmd
	Del(context.Context, ...string) *redis.IntCmd
	Scan(context.Context, uint64, string, int64) *redis.ScanCmd
	Unlink(context.Context, ...string) *redis.IntCmd
}

// ValueOptions configures a serialized Redis L2 value cache. The caller keeps
// ownership of Client and Serializer for the cache lifetime.
type ValueOptions[V any] struct {
	// Client is one caller-owned direct stable writable-primary client.
	Client *redis.Client
	// Namespace identifies one exclusive tenant, schema, and clear trust domain.
	Namespace string
	// Serializer is retained without cloning and must support concurrent calls.
	Serializer serialization.Serializer[V]
	// Config is copied during construction. Nil uses DefaultConfig().Value.
	Config *ValueConfig
}

// ValueCache stores bounded serialized values in a caller-owned Redis client.
// Its zero value is not usable; construct it with NewValueCache.
type ValueCache[V any] struct {
	client     commandClient
	serializer serialization.Serializer[V]
	keys       btredis.KeyBuilder
	namespace  string
	config     ValueConfig
}

// NewValueCache constructs a bounded serialized Redis L2 cache. Serializer is
// retained as a caller-owned immutable dependency and must be safe for
// concurrent Marshal and Unmarshal calls.
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
		client:     options.Client,
		serializer: options.Serializer,
		keys:       builder,
		namespace:  options.Namespace,
		config:     config,
	}, nil
}

// Get returns one deserialized value. Missing Redis keys return
// cache.ErrCacheMiss exactly.
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
	encoded, err := c.client.GetRange(ctx, key.Value, 0, int64(c.config.MaxValueBytes)).Bytes()
	if errors.Is(err, redis.Nil) {
		return zero, cache.ErrCacheMiss
	}
	if err != nil {
		return zero, c.providerError("get", key.RedactedID, err, false)
	}
	if len(encoded) == 0 {
		exists, existsErr := c.client.Exists(ctx, key.Value).Result()
		if existsErr != nil {
			return zero, c.providerError("get", key.RedactedID, existsErr, false)
		}
		if exists == 0 {
			return zero, cache.ErrCacheMiss
		}
		if encoded == nil {
			encoded = []byte{}
		}
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

// Set serializes and stores one value using the supplied per-entry Redis TTL.
// A zero TTL means no Redis expiry.
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

// SetDefault stores one value using the copied default Redis TTL.
func (c *ValueCache[V]) SetDefault(ctx context.Context, key string, value V) error {
	if err := c.validateInitialized("set-default"); err != nil {
		return err
	}
	return c.Set(ctx, key, value, c.config.RemoteTTL)
}

// Delete removes one logical key. Deleting an absent key succeeds.
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
