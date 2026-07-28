package redisratelimit

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/ratelimit"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

const consumeScript = `
local requested = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local rate = tonumber(ARGV[3])
local ttl = tonumber(ARGV[4])
local scale = tonumber(ARGV[5])

if requested <= 0 or burst <= 0 or rate <= 0 or ttl <= 0 then
	return redis.error_reply("invalid token bucket arguments")
end

local now = redis.call("TIME")
local now_ms = tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
local values = redis.call("HMGET", KEYS[1], "tokens", "updated_ms")
local tokens = tonumber(values[1])
local updated_ms = tonumber(values[2])

if tokens == nil or updated_ms == nil then
	tokens = burst
	updated_ms = now_ms
end

local elapsed_ms = now_ms - updated_ms
if elapsed_ms < 0 then
	elapsed_ms = 0
end
if elapsed_ms > 0 then
	local refill = math.floor(elapsed_ms * rate / 1000)
	tokens = math.min(burst, tokens + refill)
	updated_ms = now_ms
end

local allowed = 0
if tokens >= requested then
	tokens = tokens - requested
	allowed = 1
end

redis.call("HSET", KEYS[1], "tokens", math.floor(tokens), "updated_ms", updated_ms)
redis.call("PEXPIRE", KEYS[1], ttl)

local retry_after_ms = 0
if allowed == 0 then
	retry_after_ms = math.ceil((requested - tokens) * 1000 / rate)
end
local reset_after_ms = math.ceil((burst - tokens) * 1000 / rate)
local remaining = math.floor(tokens / scale)

return { allowed, remaining, retry_after_ms, reset_after_ms }
`

// Limiter Redis-backed keyed rate limiter다.
type Limiter struct {
	client redis.Cmdable
	opts   options
}

var _ ratelimit.Limiter = (*Limiter)(nil)

// New Redis token bucket limiter를 만든다.
func New(options Options) (*Limiter, error) {
	normalized, err := options.normalize()
	if err != nil {
		return nil, err
	}
	return &Limiter{client: normalized.client, opts: normalized}, nil
}

// Allow Redis bucket에서 token을 소비한다.
//
// ErrCommitUnknown이면 zero Result가 반환되지만 요청이 한 번 debit됐을 수 있다.
// 자동 replay하지 말고 type-first로 오류를 판별한 뒤 보수적으로 full-refill
// (Burst / RatePerSecond) 동안 기다리거나 caller budget에서 한 번의 debit을 흡수한다.
func (l *Limiter) Allow(ctx context.Context, key string, tokens int64) (ratelimit.Result, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return ratelimit.Result{}, err
	}
	if l == nil || l.client == nil {
		return ratelimit.Result{}, fmt.Errorf("redis rate limiter is not initialized")
	}
	key, err := l.normalizeKey(key)
	if err != nil {
		return ratelimit.Result{}, err
	}
	if tokens > l.opts.burst {
		return ratelimit.Result{}, fmt.Errorf("tokens must not exceed burst")
	}
	requestedMicros, err := tokensToMicros(tokens)
	if err != nil {
		return ratelimit.Result{}, err
	}

	bucketKey := l.bucketKey(key)
	values, err := l.client.Eval(
		ctx,
		consumeScript,
		[]string{bucketKey},
		requestedMicros,
		l.opts.burstMicros,
		l.opts.rateMicrosPerSecond,
		l.opts.idleTTL.Milliseconds(),
		tokenScale,
	).Slice()
	if err != nil {
		var before *notDispatchedError
		if errors.As(err, &before) {
			return ratelimit.Result{}, before.Unwrap()
		}
		return ratelimit.Result{}, errors.Join(
			operationError(ctx, "consume", bucketKey, err),
			ratelimit.ErrCommitUnknown,
			btredis.ErrCommitUnknown,
		)
	}
	result, err := parseResult(tokens, values)
	if err != nil {
		return ratelimit.Result{}, errors.Join(
			operationError(ctx, "parse-result", bucketKey, err),
			ratelimit.ErrCommitUnknown,
			btredis.ErrCommitUnknown,
		)
	}
	return result, nil
}

type notDispatchedError struct{ cause error }

func (*notDispatchedError) Error() string { return "redis rate limiter: command not dispatched" }
func (e *notDispatchedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (l *Limiter) normalizeKey(key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("key must not be empty")
	}
	if len([]byte(key)) > l.opts.maxKeyBytes {
		return "", fmt.Errorf("key exceeds max key bytes")
	}
	return key, nil
}

func (l *Limiter) bucketKey(key string) string {
	return defaultKeyPrefix + ":" + l.opts.namespace + ":bucket:" + key
}

func parseResult(requested int64, values []any) (ratelimit.Result, error) {
	if len(values) != 4 {
		return ratelimit.Result{}, fmt.Errorf("unexpected redis script result length: %d", len(values))
	}
	allowed, err := int64Value(values[0])
	if err != nil {
		return ratelimit.Result{}, fmt.Errorf("parse allowed: %w", err)
	}
	remaining, err := int64Value(values[1])
	if err != nil {
		return ratelimit.Result{}, fmt.Errorf("parse remaining: %w", err)
	}
	retryAfterMillis, err := int64Value(values[2])
	if err != nil {
		return ratelimit.Result{}, fmt.Errorf("parse retry after: %w", err)
	}
	resetAfterMillis, err := int64Value(values[3])
	if err != nil {
		return ratelimit.Result{}, fmt.Errorf("parse reset after: %w", err)
	}
	return ratelimit.Result{
		Allowed:    allowed == 1,
		Requested:  requested,
		Remaining:  remaining,
		RetryAfter: time.Duration(retryAfterMillis) * time.Millisecond,
		ResetAfter: time.Duration(resetAfterMillis) * time.Millisecond,
	}, nil
}

func int64Value(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	case []byte:
		return strconv.ParseInt(string(v), 10, 64)
	default:
		return 0, fmt.Errorf("unsupported value type %T", value)
	}
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
		btredis.OpLabels{Family: "rate limiter", Operation: operation},
		rawKey,
		err,
	)
}
