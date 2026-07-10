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
	Get(context.Context, string) *redis.StringCmd
	Set(context.Context, string, any, time.Duration) *redis.StatusCmd
	Del(context.Context, ...string) *redis.IntCmd
}

// Set serializes value and stores its BTFV envelope with a positive TTL.
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

// Get reads and decodes one BTFV value. Missing keys return cache.ErrCacheMiss.
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
	encoded, err := c.client.Get(ctx, key.Value).Bytes()
	if errors.Is(err, redis.Nil) {
		return value, cache.ErrCacheMiss
	}
	if err != nil {
		return value, c.operationError(ctx, "get", key.Value)
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
