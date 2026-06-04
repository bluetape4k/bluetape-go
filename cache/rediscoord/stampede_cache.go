package rediscoord

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	redislock "github.com/bluetape4k/bluetape-go/lock/redis"
	"github.com/redis/go-redis/v9"
)

// StampedeCache 는 Redis로 cross-process load burst를 조정한다.
type StampedeCache[V any] struct {
	cfg config[V]
}

var _ cache.LoadingCache[string, string] = (*StampedeCache[string])(nil)

// NewStampedeCache 는 Redis coordination wrapper를 만든다.
func NewStampedeCache[V any](options Options[V]) (*StampedeCache[V], error) {
	cfg, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}
	return &StampedeCache[V]{cfg: cfg}, nil
}

// Get 은 감싼 cache에서 값을 읽는다.
func (c *StampedeCache[V]) Get(ctx context.Context, key string) (V, error) {
	return c.cfg.cache.Get(normalizeContext(ctx), key)
}

// Set 은 감싼 cache에 값을 쓴다.
func (c *StampedeCache[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error {
	return c.cfg.cache.Set(normalizeContext(ctx), key, value, ttl)
}

// Delete 는 감싼 cache에서 key를 제거한다.
func (c *StampedeCache[V]) Delete(ctx context.Context, key string) error {
	return c.cfg.cache.Delete(normalizeContext(ctx), key)
}

// Clear 는 감싼 cache를 비운다.
func (c *StampedeCache[V]) Clear(ctx context.Context) error {
	return c.cfg.cache.Clear(normalizeContext(ctx))
}

// GetOrLoad 는 cold miss burst에서 한 process의 loader 결과를 공유한다.
func (c *StampedeCache[V]) GetOrLoad(
	ctx context.Context,
	key string,
	ttl time.Duration,
	loader cache.Loader[string, V],
) (V, error) {
	var zero V
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if loader == nil {
		return zero, fmt.Errorf("loader must not be nil")
	}
	if ttl < 0 {
		return zero, fmt.Errorf("ttl must not be negative")
	}

	value, err := c.cfg.cache.Get(ctx, key)
	if err == nil {
		return value, nil
	}
	if !errors.Is(err, cache.ErrCacheMiss) {
		return zero, err
	}

	for {
		if err := ctx.Err(); err != nil {
			return zero, err
		}

		lease, err := c.tryAcquire(ctx, key)
		if err == nil {
			return c.loadAsOwner(ctx, key, ttl, loader, lease)
		}
		if !errors.Is(err, redislock.ErrNotAcquired) {
			return zero, err
		}

		value, ok, err := c.waitForOwnerResult(ctx, key, ttl)
		if err != nil {
			return zero, err
		}
		if ok {
			return value, nil
		}
	}
}

func (c *StampedeCache[V]) tryAcquire(ctx context.Context, key string) (*redislock.Lease, error) {
	mutex, err := redislock.New(c.cfg.client, redislock.Options{
		Key: c.lockKey(key),
		TTL: c.cfg.lockTTL,
	})
	if err != nil {
		return nil, err
	}
	return mutex.TryLock(ctx)
}

func (c *StampedeCache[V]) loadAsOwner(
	ctx context.Context,
	key string,
	ttl time.Duration,
	loader cache.Loader[string, V],
	lease *redislock.Lease,
) (V, error) {
	var zero V
	var payload []byte
	var payloadReady bool

	wrappedLoader := func(ctx context.Context, key string) (V, error) {
		loaded, err := loader(ctx, key)
		if err != nil {
			return zero, err
		}
		encoded, err := c.cfg.codec.Marshal(loaded)
		if err != nil {
			return zero, fmt.Errorf("marshal load result: %w", err)
		}
		payload = encoded
		payloadReady = true
		return loaded, nil
	}

	value, err := c.cfg.cache.GetOrLoad(ctx, key, ttl, wrappedLoader)
	if err != nil {
		return zero, errors.Join(err, c.unlock(lease))
	}
	if !payloadReady {
		payload, err = c.cfg.codec.Marshal(value)
		if err != nil {
			return zero, errors.Join(fmt.Errorf("marshal cache result: %w", err), c.unlock(lease))
		}
	}
	if err := c.ensureOwner(ctx, lease); err != nil {
		return zero, errors.Join(err, c.unlock(lease))
	}
	if err := c.storeResult(ctx, key, lease.Token(), payload); err != nil {
		return zero, errors.Join(err, c.unlock(lease))
	}
	if err := c.unlock(lease); err != nil {
		return zero, err
	}
	return value, nil
}

func (c *StampedeCache[V]) waitForOwnerResult(ctx context.Context, key string, ttl time.Duration) (V, bool, error) {
	var zero V
	var ownerToken string

	ticker := time.NewTicker(c.cfg.pollInterval)
	defer ticker.Stop()

	for {
		if ownerToken != "" {
			value, ok, err := c.readOwnerResult(ctx, key, ttl, ownerToken)
			if err != nil {
				return zero, false, err
			}
			if ok {
				return value, true, nil
			}
		}

		token, exists, err := c.ownerToken(ctx, key)
		if err != nil {
			return zero, false, err
		}
		if !exists {
			if ownerToken != "" {
				value, ok, err := c.readOwnerResult(ctx, key, ttl, ownerToken)
				if err != nil || ok {
					return value, ok, err
				}
			}
			return zero, false, nil
		}
		ownerToken = token

		select {
		case <-ctx.Done():
			return zero, false, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *StampedeCache[V]) readOwnerResult(
	ctx context.Context,
	key string,
	ttl time.Duration,
	ownerToken string,
) (V, bool, error) {
	var zero V
	encoded, err := c.cfg.client.Get(ctx, c.resultKey(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, fmt.Errorf("redis load result get: %w", err)
	}

	payload, ok, err := decodeResult(encoded, ownerToken)
	if err != nil || !ok {
		return zero, false, err
	}
	value, err := c.cfg.codec.Unmarshal(payload)
	if err != nil {
		return zero, false, fmt.Errorf("unmarshal load result: %w", err)
	}
	filled, err := c.cfg.cache.GetOrLoad(ctx, key, ttl, func(context.Context, string) (V, error) {
		return value, nil
	})
	if err != nil {
		return zero, false, err
	}
	return filled, true, nil
}

func (c *StampedeCache[V]) ownerToken(ctx context.Context, key string) (string, bool, error) {
	token, err := c.cfg.client.Get(ctx, c.lockKey(key)).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("redis load lock owner get: %w", err)
	}
	return token, true, nil
}

func (c *StampedeCache[V]) ensureOwner(ctx context.Context, lease *redislock.Lease) error {
	if lease == nil {
		return fmt.Errorf("redis load lock lease is nil")
	}
	token, err := c.cfg.client.Get(ctx, lease.Key()).Result()
	if errors.Is(err, redis.Nil) {
		return fmt.Errorf("redis load lock expired before result publication")
	}
	if err != nil {
		return fmt.Errorf("redis load lock owner get: %w", err)
	}
	if token != lease.Token() {
		return fmt.Errorf("redis load lock expired before result publication")
	}
	return nil
}

func (c *StampedeCache[V]) storeResult(ctx context.Context, key string, token string, payload []byte) error {
	encoded, err := encodeResult(token, payload)
	if err != nil {
		return err
	}
	if err := c.cfg.client.Set(ctx, c.resultKey(key), encoded, c.cfg.resultTTL).Err(); err != nil {
		return fmt.Errorf("redis load result set: %w", err)
	}
	return nil
}

func (c *StampedeCache[V]) unlock(lease *redislock.Lease) error {
	if lease == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), unlockTimeout)
	defer cancel()

	released, err := lease.Unlock(ctx)
	if err != nil {
		return err
	}
	if !released {
		return fmt.Errorf("redis load lock was not released by owner")
	}
	return nil
}

func (c *StampedeCache[V]) lockKey(key string) string {
	return c.cfg.keyPrefix + ":lock:" + key
}

func (c *StampedeCache[V]) resultKey(key string) string {
	return c.cfg.keyPrefix + ":result:" + key
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
