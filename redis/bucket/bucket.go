package redisbucket

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/bluetape4k/bluetape-go/serialization"
	"github.com/redis/go-redis/v9"
)

// Client Bucket이 사용하는 caller-owned Redis command의 최소 surface다.
type Client interface {
	Set(context.Context, string, any, time.Duration) *redis.StatusCmd
	SetNX(context.Context, string, any, time.Duration) *redis.BoolCmd
	Del(context.Context, ...string) *redis.IntCmd
	Eval(context.Context, string, []string, ...any) *redis.Cmd
}

// Options typed Bucket을 설정하며 입력값의 소유권을 가져가지 않는다.
type Options[V any] struct {
	Namespace       string
	HashTag         string
	Serializer      serialization.Serializer[V]
	Logger          *slog.Logger
	MaxPayloadBytes int
}

// Bucket caller가 선택한 encoding으로 single-key Redis value를 다루는 primitive다.
type Bucket[V any] struct {
	client          Client
	serializer      serialization.Serializer[V]
	keys            btredis.KeyBuilder
	logger          *slog.Logger
	maxPayloadBytes int
}

const (
	// DefaultMaxPayloadBytes 기본 serialized value 크기 상한이다.
	DefaultMaxPayloadBytes = 1 << 20
	// MaxPayloadBytesLimit New가 허용하는 serialized value 크기 상한의 최대값이다.
	MaxPayloadBytesLimit = 64 << 20

	boundedGetScript = `local value = redis.call("GETRANGE", KEYS[1], 0, ARGV[1])
if value == "" and redis.call("EXISTS", KEYS[1]) == 0 then return {0} end
if string.len(value) > tonumber(ARGV[2]) then return {2} end
return {1, value}`
	getAndDeleteScript = `local value = redis.call("GETRANGE", KEYS[1], 0, ARGV[1])
if value == "" and redis.call("EXISTS", KEYS[1]) == 0 then return {0} end
if string.len(value) > tonumber(ARGV[1]) then return {2} end
redis.call("DEL", KEYS[1])
return {1, value}`
	compareAndSetScript = `local current = redis.call("GETRANGE", KEYS[1], 0, ARGV[4])
if current == "" and redis.call("EXISTS", KEYS[1]) == 0 then return 0 end
if string.len(current) > tonumber(ARGV[4]) then return 2 end
if current ~= ARGV[1] then return 0 end
if ARGV[3] == "0" then
  redis.call("SET", KEYS[1], ARGV[2])
else
  redis.call("SET", KEYS[1], ARGV[2], "PX", ARGV[3])
end
return 1`
)

var _ interface {
	Get(context.Context, string) (string, bool, error)
} = (*Bucket[string])(nil)

// New caller-owned Redis client와 serializer 위에 Bucket을 생성한다.
func New[V any](client Client, options Options[V]) (*Bucket[V], error) {
	if isNil(client) {
		return nil, ErrInvalidClient
	}
	if isNil(options.Serializer) {
		return nil, ErrInvalidOptions
	}
	maxPayloadBytes, err := normalizeMaxPayloadBytes(options.MaxPayloadBytes)
	if err != nil {
		return nil, err
	}
	builder, err := btredis.NewKeyBuilder(options.Namespace)
	if err != nil {
		return nil, err
	}
	builder, err = builder.Structural("bucket")
	if err != nil {
		return nil, err
	}
	if options.HashTag != "" {
		builder, err = builder.WithHashTag(options.HashTag)
		if err != nil {
			return nil, err
		}
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Bucket[V]{
		client:          client,
		serializer:      options.Serializer,
		keys:            builder,
		logger:          logger,
		maxPayloadBytes: maxPayloadBytes,
	}, nil
}

// Get value 하나를 읽고 missing key와 serialized zero value를 구분한다.
func (b *Bucket[V]) Get(ctx context.Context, logicalKey string) (V, bool, error) {
	var zero V
	if err := b.ready(); err != nil {
		return zero, false, err
	}
	if err := validateContext(ctx); err != nil {
		return zero, false, err
	}
	key, err := b.key(logicalKey)
	if err != nil {
		return zero, false, err
	}
	limit := strconv.FormatInt(int64(b.maxPayloadBytes), 10)
	cmd := b.client.Eval(ctx, boundedGetScript, []string{key.Value}, limit, limit)
	if cmd == nil {
		return zero, false, newError("get", key.RedactedID, ErrMalformedResult)
	}
	result, err := cmd.Result()
	if ctxErr := ctx.Err(); ctxErr != nil {
		if err != nil {
			wrapped := errors.Join(providerError("get", key.RedactedID, err, false), ctxErr)
			b.logFailure(ctx, "get", key.RedactedID, wrapped)
			return zero, false, wrapped
		}
		return zero, false, ctxErr
	}
	if err != nil {
		wrapped := providerError("get", key.RedactedID, err, false)
		b.logFailure(ctx, "get", key.RedactedID, wrapped)
		return zero, false, wrapped
	}
	status, payload, hit, ok := parseBoundedGetResult(result)
	if !ok {
		wrapped := newError("get", key.RedactedID, ErrMalformedResult)
		b.logFailure(ctx, "get", key.RedactedID, wrapped)
		return zero, false, wrapped
	}
	if status == 2 {
		wrapped := newError("get", key.RedactedID, ErrPayloadTooLarge)
		b.logFailure(ctx, "get", key.RedactedID, wrapped)
		return zero, false, wrapped
	}
	if !hit {
		return zero, false, nil
	}
	if len(payload) > b.maxPayloadBytes {
		wrapped := newError("get", key.RedactedID, ErrPayloadTooLarge)
		b.logFailure(ctx, "get", key.RedactedID, wrapped)
		return zero, false, wrapped
	}
	value, err := b.serializer.Unmarshal(payloadOrEmpty(payload))
	if err != nil {
		wrapped := codecError("get", key.RedactedID, ErrInvalidPayload, err)
		b.logFailure(ctx, "get", key.RedactedID, wrapped)
		return zero, false, wrapped
	}
	return value, true, nil
}

// Set value 하나를 직렬화해 persistent 또는 정규화된 TTL로 저장한다.
func (b *Bucket[V]) Set(ctx context.Context, logicalKey string, value V, ttl time.Duration) error {
	key, payload, normalized, err := b.prepareWrite(ctx, "set", logicalKey, value, ttl)
	if err != nil {
		return err
	}
	cmd := b.client.Set(ctx, key.Value, payload, normalized)
	if cmd == nil {
		wrapped := malformedMutation("set", key.RedactedID)
		b.logFailure(ctx, "set", key.RedactedID, wrapped)
		return wrapped
	}
	_, dispatchErr := cmd.Result()
	return b.finishMutation(ctx, "set", key.RedactedID, dispatchErr)
}

// SetIfAbsent logical key가 없을 때만 value 하나를 저장한다.
func (b *Bucket[V]) SetIfAbsent(ctx context.Context, logicalKey string, value V, ttl time.Duration) (bool, error) {
	key, payload, normalized, err := b.prepareWrite(ctx, "set-if-absent", logicalKey, value, ttl)
	if err != nil {
		return false, err
	}
	cmd := b.client.SetNX(ctx, key.Value, payload, normalized)
	if cmd == nil {
		wrapped := malformedMutation("set-if-absent", key.RedactedID)
		b.logFailure(ctx, "set-if-absent", key.RedactedID, wrapped)
		return false, wrapped
	}
	ok, dispatchErr := cmd.Result()
	if dispatchErr != nil {
		return false, b.finishMutation(ctx, "set-if-absent", key.RedactedID, dispatchErr)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return ok, nil
}

// GetAndDelete serialized value 하나를 atomic하게 반환하고 삭제한다.
func (b *Bucket[V]) GetAndDelete(ctx context.Context, logicalKey string) (V, bool, error) {
	var zero V
	if err := b.ready(); err != nil {
		return zero, false, err
	}
	if err := validateContext(ctx); err != nil {
		return zero, false, err
	}
	key, err := b.key(logicalKey)
	if err != nil {
		return zero, false, err
	}
	limit := strconv.FormatInt(int64(b.maxPayloadBytes), 10)
	cmd := b.client.Eval(ctx, getAndDeleteScript, []string{key.Value}, limit)
	if cmd == nil {
		wrapped := malformedMutation("get-and-delete", key.RedactedID)
		b.logFailure(ctx, "get-and-delete", key.RedactedID, wrapped)
		return zero, false, wrapped
	}
	result, dispatchErr := cmd.Result()
	if dispatchErr != nil {
		return zero, false, b.finishMutation(ctx, "get-and-delete", key.RedactedID, dispatchErr)
	}
	if err := ctx.Err(); err != nil {
		return zero, false, err
	}
	status, payload, hit, ok := parseGetAndDeleteResult(result)
	if !ok {
		wrapped := malformedMutation("get-and-delete", key.RedactedID)
		b.logFailure(ctx, "get-and-delete", key.RedactedID, wrapped)
		return zero, false, wrapped
	}
	if status == 2 {
		wrapped := newError("get-and-delete", key.RedactedID, ErrPayloadTooLarge)
		b.logFailure(ctx, "get-and-delete", key.RedactedID, wrapped)
		return zero, false, wrapped
	}
	if !hit {
		return zero, false, nil
	}
	value, err := b.serializer.Unmarshal(payloadOrEmpty(payload))
	if err != nil {
		wrapped := codecError("get-and-delete", key.RedactedID, ErrInvalidPayload, err)
		b.logFailure(ctx, "get-and-delete", key.RedactedID, wrapped)
		return zero, false, wrapped
	}
	return value, true, nil
}

// CompareAndSet serialized expected value가 일치할 때만 value를 atomic하게 교체한다.
func (b *Bucket[V]) CompareAndSet(ctx context.Context, logicalKey string, expected, replacement V, ttl time.Duration) (bool, error) {
	if err := b.ready(); err != nil {
		return false, err
	}
	if err := validateContext(ctx); err != nil {
		return false, err
	}
	key, err := b.key(logicalKey)
	if err != nil {
		return false, err
	}
	normalized, err := normalizeTTL(ttl)
	if err != nil {
		return false, err
	}
	expectedPayload, err := b.serializer.Marshal(expected)
	if err != nil {
		wrapped := codecError("compare-and-set", key.RedactedID, ErrSerialization, err)
		b.logFailure(ctx, "compare-and-set", key.RedactedID, wrapped)
		return false, wrapped
	}
	if len(expectedPayload) > b.maxPayloadBytes {
		wrapped := newError("compare-and-set", key.RedactedID, ErrPayloadTooLarge)
		b.logFailure(ctx, "compare-and-set", key.RedactedID, wrapped)
		return false, wrapped
	}
	replacementPayload, err := b.serializer.Marshal(replacement)
	if err != nil {
		wrapped := codecError("compare-and-set", key.RedactedID, ErrSerialization, err)
		b.logFailure(ctx, "compare-and-set", key.RedactedID, wrapped)
		return false, wrapped
	}
	if len(replacementPayload) > b.maxPayloadBytes {
		wrapped := newError("compare-and-set", key.RedactedID, ErrPayloadTooLarge)
		b.logFailure(ctx, "compare-and-set", key.RedactedID, wrapped)
		return false, wrapped
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	cmd := b.client.Eval(ctx, compareAndSetScript, []string{key.Value}, payloadOrEmpty(expectedPayload), payloadOrEmpty(replacementPayload), strconv.FormatInt(normalized.Milliseconds(), 10), strconv.FormatInt(int64(b.maxPayloadBytes), 10))
	if cmd == nil {
		wrapped := malformedMutation("compare-and-set", key.RedactedID)
		b.logFailure(ctx, "compare-and-set", key.RedactedID, wrapped)
		return false, wrapped
	}
	result, dispatchErr := cmd.Int64()
	if dispatchErr != nil {
		return false, b.finishMutation(ctx, "compare-and-set", key.RedactedID, dispatchErr)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	switch result {
	case 0:
		return false, nil
	case 1:
		return true, nil
	case 2:
		wrapped := newError("compare-and-set", key.RedactedID, ErrPayloadTooLarge)
		b.logFailure(ctx, "compare-and-set", key.RedactedID, wrapped)
		return false, wrapped
	default:
		wrapped := malformedMutation("compare-and-set", key.RedactedID)
		b.logFailure(ctx, "compare-and-set", key.RedactedID, wrapped)
		return false, wrapped
	}
}

// Delete logical key를 idempotently 삭제한다.
func (b *Bucket[V]) Delete(ctx context.Context, logicalKey string) error {
	if err := b.ready(); err != nil {
		return err
	}
	if err := validateContext(ctx); err != nil {
		return err
	}
	key, err := b.key(logicalKey)
	if err != nil {
		return err
	}
	cmd := b.client.Del(ctx, key.Value)
	if cmd == nil {
		wrapped := malformedMutation("delete", key.RedactedID)
		b.logFailure(ctx, "delete", key.RedactedID, wrapped)
		return wrapped
	}
	_, dispatchErr := cmd.Result()
	return b.finishMutation(ctx, "delete", key.RedactedID, dispatchErr)
}

func (b *Bucket[V]) prepareWrite(ctx context.Context, operation, logicalKey string, value V, ttl time.Duration) (btredis.Key, []byte, time.Duration, error) {
	var zero btredis.Key
	if err := b.ready(); err != nil {
		return zero, nil, 0, err
	}
	if err := validateContext(ctx); err != nil {
		return zero, nil, 0, err
	}
	key, err := b.key(logicalKey)
	if err != nil {
		return zero, nil, 0, err
	}
	normalized, err := normalizeTTL(ttl)
	if err != nil {
		return zero, nil, 0, err
	}
	payload, err := b.serializer.Marshal(value)
	if err != nil {
		return zero, nil, 0, codecError(operation, key.RedactedID, ErrSerialization, err)
	}
	if len(payload) > b.maxPayloadBytes {
		wrapped := newError(operation, key.RedactedID, ErrPayloadTooLarge)
		b.logFailure(ctx, operation, key.RedactedID, wrapped)
		return zero, nil, 0, wrapped
	}
	if err := ctx.Err(); err != nil {
		return zero, nil, 0, err
	}
	return key, payloadOrEmpty(payload), normalized, nil
}

func (b *Bucket[V]) finishMutation(ctx context.Context, operation, keyID string, dispatchErr error) error {
	if dispatchErr != nil {
		wrapped := providerError(operation, keyID, dispatchErr, true)
		if ctxErr := ctx.Err(); ctxErr != nil {
			wrapped = errors.Join(wrapped, ctxErr)
		}
		b.logFailure(ctx, operation, keyID, wrapped)
		return wrapped
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return nil
}

func (b *Bucket[V]) logFailure(ctx context.Context, operation, keyID string, err error) {
	if b != nil && b.logger != nil {
		b.logger.WarnContext(ctx, "redis bucket operation failed", "operation", operation, "key_id", keyID, "error_type", causeType(err))
	}
}

func (b *Bucket[V]) key(logicalKey string) (btredis.Key, error) {
	return b.keys.LogicalKey(logicalKey)
}

func (b *Bucket[V]) ready() error {
	if b == nil || isNil(b.client) || isNil(b.serializer) || reflect.ValueOf(b.keys).IsZero() {
		return ErrUninitialized
	}
	return nil
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return ErrInvalidContext
	}
	return ctx.Err()
}

func normalizeTTL(ttl time.Duration) (time.Duration, error) {
	if ttl < 0 {
		return 0, fmt.Errorf("%w: negative ttl", btredis.ErrInvalidTTL)
	}
	if ttl == 0 {
		return 0, nil
	}
	if ttl < time.Millisecond {
		return time.Millisecond, nil
	}
	return ttl.Truncate(time.Millisecond), nil
}

func normalizeMaxPayloadBytes(maxPayloadBytes int) (int, error) {
	if maxPayloadBytes == 0 {
		return DefaultMaxPayloadBytes, nil
	}
	if maxPayloadBytes < 1 || maxPayloadBytes > MaxPayloadBytesLimit {
		return 0, fmt.Errorf("%w: max payload bytes must be between 1 and %d", ErrInvalidOptions, MaxPayloadBytesLimit)
	}
	return maxPayloadBytes, nil
}

func payloadOrEmpty(payload []byte) []byte {
	if payload == nil {
		return []byte{}
	}
	return append([]byte{}, payload...)
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func parseGetAndDeleteResult(value any) (int64, []byte, bool, bool) {
	values, ok := value.([]interface{})
	if !ok || len(values) == 0 {
		return 0, nil, false, false
	}
	status, ok := scriptInt(values[0:1])
	if !ok {
		return 0, nil, false, false
	}
	switch status {
	case 0:
		return status, nil, false, len(values) == 1
	case 1:
		if len(values) != 2 {
			return status, nil, false, false
		}
		payload, ok := resultBytes(values[1])
		return status, payload, ok, ok
	case 2:
		return status, nil, false, len(values) == 1
	default:
		return status, nil, false, false
	}
}

func parseBoundedGetResult(value any) (int64, []byte, bool, bool) {
	values, ok := value.([]interface{})
	if !ok || len(values) == 0 {
		return 0, nil, false, false
	}
	status, ok := scriptInt(values[:1])
	if !ok {
		return 0, nil, false, false
	}
	switch status {
	case 0, 2:
		return status, nil, false, len(values) == 1
	case 1:
		if len(values) != 2 {
			return status, nil, false, false
		}
		payload, ok := resultBytes(values[1])
		return status, payload, true, ok
	default:
		return status, nil, false, false
	}
}

func scriptInt(values []interface{}) (int64, bool) {
	if len(values) != 1 {
		return 0, false
	}
	switch value := values[0].(type) {
	case int64:
		return value, true
	case int:
		return int64(value), true
	case string:
		parsed, err := strconv.ParseInt(value, 10, 64)
		return parsed, err == nil
	case []byte:
		parsed, err := strconv.ParseInt(string(value), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func resultBytes(value any) ([]byte, bool) {
	switch typed := value.(type) {
	case []byte:
		return append([]byte(nil), typed...), true
	case string:
		return []byte(typed), true
	default:
		return nil, false
	}
}
