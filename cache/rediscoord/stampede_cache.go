package rediscoord

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	redislock "github.com/bluetape4k/bluetape-go/lock/redis"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

// StampedeCache struct 공개 타입이며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type StampedeCache[V any] struct {
	cfg config[V]
}

var _ cache.LoadingCache[string, string] = (*StampedeCache[string])(nil)

// NewStampedeCache NewStampedeCache 공개 API의 동작을 수행하며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
//
// 매개변수:
//   - options: NewStampedeCache 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, 취소, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func NewStampedeCache[V any](options Options[V]) (*StampedeCache[V], error) {
	cfg, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}
	return &StampedeCache[V]{cfg: cfg}, nil
}

// Get Get 공개 API의 동작을 수행하며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, 취소, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (c *StampedeCache[V]) Get(ctx context.Context, key string) (V, error) {
	return c.cfg.cache.Get(normalizeContext(ctx), key)
}

// Set Set 공개 API의 동작을 수행하며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//   - value: 직렬화하거나 cache에 보관할 값이다. nil, zero value, aliasing 의미는 serializer/cache 계약을 따른다.
//   - ttl: cache entry의 유효 시간이다. zero, 음수, 만료 의미는 옵션과 TTL 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, 취소, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (c *StampedeCache[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error {
	return c.cfg.cache.Set(normalizeContext(ctx), key, value, ttl)
}

// Delete Delete 공개 API의 동작을 수행하며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, 취소, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (c *StampedeCache[V]) Delete(ctx context.Context, key string) error {
	return c.cfg.cache.Delete(normalizeContext(ctx), key)
}

// Clear Clear 공개 API의 동작을 수행하며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, 취소, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (c *StampedeCache[V]) Clear(ctx context.Context) error {
	return c.cfg.cache.Clear(normalizeContext(ctx))
}

// GetOrLoad GetOrLoad 공개 API의 동작을 수행하며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - key: cache lookup과 저장에 사용하는 caller-owned key다. 정규화와 namespace 의미는 package 계약을 따른다.
//   - ttl: cache entry의 유효 시간이다. zero, 음수, 만료 의미는 옵션과 TTL 계약을 따른다.
//   - loader: GetOrLoad 동작에 필요한 loader 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, 취소, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
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
		if lease != nil && err != nil {
			return zero, errors.Join(err, c.reconcileUnlock(lease))
		}
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
	resultKey := c.resultKey(key)
	var encoded []byte
	var err error
	if c.cfg.maxResultBytes > 0 {
		encoded, err = c.cfg.client.GetRange(ctx, resultKey, 0, int64(c.cfg.maxResultBytes)).Bytes()
		if err == nil && len(encoded) == 0 {
			return zero, false, nil
		}
	} else {
		encoded, err = c.cfg.client.Get(ctx, resultKey).Bytes()
	}
	if errors.Is(err, redis.Nil) {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, operationError(ctx, "result-get", resultKey, err)
	}
	if c.cfg.maxResultBytes > 0 && len(encoded) > c.cfg.maxResultBytes {
		return zero, false, operationError(ctx, "result-get", resultKey, ErrResultTooLarge)
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
		return "", false, operationError(ctx, "owner-get", c.lockKey(key), err)
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
		return operationError(ctx, "owner-check", lease.Key(), err)
	}
	if token != lease.Token() {
		return fmt.Errorf("redis load lock expired before result publication")
	}
	return nil
}

func (c *StampedeCache[V]) storeResult(ctx context.Context, key string, token string, payload []byte) error {
	if c.cfg.maxResultBytes > 0 {
		size, err := encodedResultSize(token, payload)
		if err != nil {
			return err
		}
		if size > c.cfg.maxResultBytes {
			return operationError(ctx, "result-set", c.resultKey(key), ErrResultTooLarge)
		}
	}
	encoded, err := encodeResult(token, payload)
	if err != nil {
		return err
	}
	if c.cfg.maxResultBytes > 0 && len(encoded) > c.cfg.maxResultBytes {
		return operationError(ctx, "result-set", c.resultKey(key), ErrResultTooLarge)
	}
	if err := c.cfg.client.Set(ctx, c.resultKey(key), encoded, c.cfg.resultTTL).Err(); err != nil {
		return operationError(ctx, "result-set", c.resultKey(key), err)
	}
	return nil
}

type leaseUnlocker interface {
	Unlock(context.Context) (bool, error)
}

func (c *StampedeCache[V]) unlock(lease leaseUnlocker) error {
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

func (c *StampedeCache[V]) reconcileUnlock(lease leaseUnlocker) error {
	if lease == nil {
		return nil
	}
	_, err := unlockOnce(lease)
	if err == nil {
		return nil
	}
	if !errors.Is(err, btredis.ErrCommitUnknown) {
		return err
	}
	_, retryErr := unlockOnce(lease)
	if retryErr == nil {
		return nil
	}
	return errors.Join(err, retryErr)
}

func unlockOnce(lease leaseUnlocker) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), unlockTimeout)
	defer cancel()
	return lease.Unlock(ctx)
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

func operationError(ctx context.Context, operation string, rawKey string, err error) error {
	if ctx != nil && ctx.Err() != nil {
		err = errors.Join(err, ctx.Err())
	}
	return btredis.NewOpError(
		btredis.OpLabels{Family: "cache coordination", Operation: operation},
		rawKey,
		err,
	)
}
