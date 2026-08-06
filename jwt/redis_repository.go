package jwt

import (
	"context"
	"errors"
	"fmt"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

// RedisRepository JWT key provider repository에서 caller-visible 상태와 의미를 설명한다.
type RedisRepository struct {
	client redis.Cmdable
	opts   redisRepositoryOptions
}

var _ DistributedKeyChainRepository = (*RedisRepository)(nil)

// NewRedisRepository JWT key provider repository에서 생성과 초기화 계약을 설명한다.
func NewRedisRepository(options RedisRepositoryOptions) (*RedisRepository, error) {
	normalized, err := options.normalize()
	if err != nil {
		return nil, err
	}
	return &RedisRepository{client: normalized.client, opts: normalized}, nil
}

// Current JWT key provider repository에서 반환값과 오류 의미를 설명한다.
func (r *RedisRepository) Current(ctx context.Context, now time.Time) (*KeyChain, error) {
	if err := r.validateReady(ctx); err != nil {
		return nil, err
	}
	kid, err := r.client.Get(ctx, r.opts.currentKey()).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, KeyError{Kind: ErrKeyNotFound, Err: errorsNew("current key not found")}
		}
		return nil, redisRepositoryOperationError(ctx, "current-get", r.opts.currentKey(), err)
	}
	key, err := r.findPayload(ctx, kid, now)
	if err != nil {
		return nil, err
	}
	return key, nil
}

// Find JWT key provider repository에서 반환값과 오류 의미를 설명한다.
func (r *RedisRepository) Find(ctx context.Context, kid string, now time.Time) (*KeyChain, error) {
	if err := r.validateReady(ctx); err != nil {
		return nil, err
	}
	if err := validateLookupKID(kid); err != nil {
		return nil, err
	}
	return r.findPayload(ctx, kid, now)
}

// Rotate JWT key provider repository에서 반환값과 오류 의미를 설명한다.
func (r *RedisRepository) Rotate(ctx context.Context, create func() (*KeyChain, error), now time.Time) (*KeyChain, error) {
	if err := r.validateReady(ctx); err != nil {
		return nil, err
	}
	if create == nil {
		return nil, OptionError{Option: "create", Err: errorsNew("must not be nil")}
	}
	current, observedKID, err := r.currentPayload(ctx, now)
	if err == nil {
		return current, nil
	}
	if !errors.Is(err, ErrKeyNotFound) && !errors.Is(err, ErrInvalidKey) {
		return nil, err
	}
	key, err := createWithContext(ctx, create)()
	if err != nil {
		return nil, err
	}
	return r.storeCAS(ctx, observedKID, key, now)
}

// ForcedRotate JWT key provider repository에서 동작과 caller-visible 계약을 설명한다.
func (r *RedisRepository) ForcedRotate(ctx context.Context, create func() (*KeyChain, error), now time.Time) (*KeyChain, error) {
	if err := r.validateReady(ctx); err != nil {
		return nil, err
	}
	if create == nil {
		return nil, OptionError{Option: "create", Err: errorsNew("must not be nil")}
	}
	key, err := createWithContext(ctx, create)()
	if err != nil {
		return nil, err
	}
	return r.store(ctx, key, now)
}

// DeleteAll JWT key provider repository에서 caller-visible 상태와 의미를 설명한다.
func (r *RedisRepository) DeleteAll(ctx context.Context) error {
	if err := r.validateReady(ctx); err != nil {
		return err
	}
	if err := r.client.Del(ctx, r.opts.metaKey(), r.opts.currentKey(), r.opts.keysKey(), r.opts.orderKey()).Err(); err != nil {
		return redisRepositoryOperationError(ctx, "delete-all", r.opts.metaKey(), err)
	}
	return nil
}

func (r *RedisRepository) validateReady(ctx context.Context) error {
	if r == nil || r.client == nil {
		return OptionError{Option: "repository", Err: errorsNew("must be constructed by a constructor")}
	}
	return requireContext(ctx)
}

func (r *RedisRepository) findPayload(ctx context.Context, kid string, now time.Time) (*KeyChain, error) {
	payload, err := r.client.HGet(ctx, r.opts.keysKey(), kid).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, KeyError{Kind: ErrKeyNotFound, KID: kid, Err: errorsNew("key not found")}
		}
		return nil, redisRepositoryOperationError(ctx, "key-get", r.opts.keysKey(), err)
	}
	key, err := decodeRedisKeyChain(payload, r.opts.maxKeyBytes)
	if err != nil {
		return nil, err
	}
	if key.Expired(now) {
		return nil, KeyError{Kind: ErrInvalidKey, KID: kid, Err: errorsNew("key expired")}
	}
	return key, nil
}

func (r *RedisRepository) currentPayload(ctx context.Context, now time.Time) (*KeyChain, string, error) {
	values, err := r.client.Eval(ctx, redisCurrentScript, []string{r.opts.currentKey(), r.opts.keysKey()}).Slice()
	if err != nil {
		return nil, "", redisRepositoryOperationError(ctx, "current-script", r.opts.currentKey(), err)
	}
	present, payload, kid, err := parseRedisKeyScriptResult(values)
	if err != nil {
		return nil, kid, err
	}
	if !present {
		return nil, kid, KeyError{Kind: ErrKeyNotFound, KID: kid, Err: errorsNew("current key not found")}
	}
	key, err := decodeRedisKeyChain([]byte(payload), r.opts.maxKeyBytes)
	if err != nil {
		return nil, kid, err
	}
	if key.Expired(now) {
		return nil, kid, KeyError{Kind: ErrInvalidKey, KID: kid, Err: errorsNew("key expired")}
	}
	return key, kid, nil
}

func (r *RedisRepository) storeCAS(ctx context.Context, observedKID string, key *KeyChain, now time.Time) (*KeyChain, error) {
	payload, err := r.prepareStorePayload(ctx, key, now)
	if err != nil {
		return nil, err
	}
	values, err := r.client.Eval(
		ctx,
		redisRotateCASScript,
		r.storeScriptKeys(),
		observedKID,
		key.KID(),
		string(payload),
		key.CreatedAt().UnixNano(),
		r.opts.capacity,
		durationMillis(r.opts.keyTTL),
		redisKeyVersion,
		string(key.Algorithm()),
	).Slice()
	if err != nil {
		return nil, redisRepositoryOperationError(ctx, "rotate-script", r.opts.currentKey(), err)
	}
	return r.decodeStoreScriptResult(values, now)
}

func (r *RedisRepository) store(ctx context.Context, key *KeyChain, now time.Time) (*KeyChain, error) {
	payload, err := r.prepareStorePayload(ctx, key, now)
	if err != nil {
		return nil, err
	}
	values, err := r.client.Eval(
		ctx,
		redisStoreScript,
		r.storeScriptKeys(),
		key.KID(),
		string(payload),
		key.CreatedAt().UnixNano(),
		r.opts.capacity,
		durationMillis(r.opts.keyTTL),
		redisKeyVersion,
		string(key.Algorithm()),
	).Slice()
	if err != nil {
		return nil, redisRepositoryOperationError(ctx, "store-script", r.opts.currentKey(), err)
	}
	return r.decodeStoreScriptResult(values, now)
}

func redisRepositoryOperationError(ctx context.Context, operation string, rawKey string, err error) error {
	if ctx != nil && ctx.Err() != nil {
		err = errors.Join(err, ctx.Err())
	}
	return btredis.NewOpError(
		btredis.OpLabels{Family: "jwt repository", Operation: operation},
		rawKey,
		err,
	)
}

func (r *RedisRepository) prepareStorePayload(ctx context.Context, key *KeyChain, now time.Time) ([]byte, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	if key == nil {
		return nil, KeyError{Kind: ErrInvalidKey, Err: errorsNew("key must not be nil")}
	}
	if key.Expired(now) {
		return nil, KeyError{Kind: ErrInvalidKey, KID: key.KID(), Err: errorsNew("key expired")}
	}
	if err := r.validateKeyTTL(key); err != nil {
		return nil, err
	}
	payload, err := encodeRedisKeyChain(key)
	if err != nil {
		return nil, err
	}
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	return payload, nil
}

func (r *RedisRepository) validateKeyTTL(key *KeyChain) error {
	if r.opts.keyTTL <= 0 {
		return nil
	}
	required := key.ExpiresAt().Sub(key.CreatedAt()) + r.opts.retentionLeeway
	if required <= 0 {
		return KeyError{Kind: ErrInvalidKey, KID: key.KID(), Err: errorsNew("key expired")}
	}
	if r.opts.keyTTL < required {
		return OptionError{Option: "key_ttl", Err: fmt.Errorf("must be at least key validity plus retention leeway")}
	}
	return nil
}

func (r *RedisRepository) storeScriptKeys() []string {
	return []string{r.opts.currentKey(), r.opts.keysKey(), r.opts.orderKey(), r.opts.metaKey()}
}

func (r *RedisRepository) decodeStoreScriptResult(values []any, now time.Time) (*KeyChain, error) {
	present, payload, kid, err := parseRedisKeyScriptResult(values)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, KeyError{Kind: ErrKeyNotFound, KID: kid, Err: errorsNew("stored key not found")}
	}
	key, err := decodeRedisKeyChain([]byte(payload), r.opts.maxKeyBytes)
	if err != nil {
		return nil, err
	}
	if key.Expired(now) {
		return nil, KeyError{Kind: ErrInvalidKey, KID: kid, Err: errorsNew("key expired")}
	}
	return key, nil
}

func parseRedisKeyScriptResult(values []any) (bool, string, string, error) {
	if len(values) != 3 {
		return false, "", "", fmt.Errorf("redis jwt script result length = %d", len(values))
	}
	status, err := redisInt(values[0])
	if err != nil {
		return false, "", "", fmt.Errorf("redis jwt script status: %w", err)
	}
	payload, err := redisString(values[1])
	if err != nil {
		return false, "", "", fmt.Errorf("redis jwt script payload: %w", err)
	}
	kid, err := redisString(values[2])
	if err != nil {
		return false, "", "", fmt.Errorf("redis jwt script kid: %w", err)
	}
	return status == 1 || payload != "", payload, kid, nil
}

func redisInt(value any) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case string:
		var parsed int64
		if _, err := fmt.Sscan(v, &parsed); err != nil {
			return 0, err
		}
		return parsed, nil
	case []byte:
		var parsed int64
		if _, err := fmt.Sscan(string(v), &parsed); err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported type %T", value)
	}
}

func redisString(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return "", fmt.Errorf("unsupported type %T", value)
	}
}

func durationMillis(duration time.Duration) int64 {
	if duration <= 0 {
		return 0
	}
	millis := duration.Milliseconds()
	if millis == 0 {
		return 1
	}
	return millis
}
