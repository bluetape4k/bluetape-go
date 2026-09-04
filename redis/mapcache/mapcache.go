package redismap

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

// Client MapCache가 사용하는 caller-owned Redis command의 최소 surface다.
type Client interface {
	Get(context.Context, string) *redis.StringCmd
	Set(context.Context, string, any, time.Duration) *redis.StatusCmd
	SetNX(context.Context, string, any, time.Duration) *redis.BoolCmd
	Del(context.Context, ...string) *redis.IntCmd
	Eval(context.Context, string, []string, ...any) *redis.Cmd
}

// Options typed MapCache를 설정하며 입력값의 소유권을 가져가지 않는다.
type Options[V any] struct {
	Namespace  string
	HashTag    string
	Serializer serialization.Serializer[V]
	Logger     *slog.Logger
}

// MapCache 독립 entry TTL을 사용하는 key-per-entry Redis map primitive다.
type MapCache[V any] struct {
	client     Client
	serializer serialization.Serializer[V]
	keys       btredis.KeyBuilder
	logger     *slog.Logger
}

const (
	getAndDeleteScript = `local value = redis.call("GET", KEYS[1])
if not value then return {0} end
redis.call("DEL", KEYS[1])
return {1, value}`
	compareAndSetScript = `local current = redis.call("GET", KEYS[1])
if not current or current ~= ARGV[1] then return 0 end
if ARGV[3] == "0" then
  redis.call("SET", KEYS[1], ARGV[2])
else
  redis.call("SET", KEYS[1], ARGV[2], "PX", ARGV[3])
end
return 1`
)

// New caller-owned Redis client와 serializer 위에 MapCache를 생성한다.
func New[V any](client Client, options Options[V]) (*MapCache[V], error) {
	if isNil(client) {
		return nil, ErrInvalidClient
	}
	if isNil(options.Serializer) {
		return nil, ErrInvalidOptions
	}
	builder, err := btredis.NewKeyBuilder(options.Namespace)
	if err != nil {
		return nil, err
	}
	builder, err = builder.Structural("map")
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
	return &MapCache[V]{client: client, serializer: options.Serializer, keys: builder, logger: logger}, nil
}

// Get map entry 하나를 읽고 logical key 존재 여부를 반환한다.
func (m *MapCache[V]) Get(ctx context.Context, logicalKey string) (V, bool, error) {
	var zero V
	if err := m.ready(); err != nil {
		return zero, false, err
	}
	if err := validateContext(ctx); err != nil {
		return zero, false, err
	}
	key, err := m.key(logicalKey)
	if err != nil {
		return zero, false, err
	}
	cmd := m.client.Get(ctx, key.Value)
	if cmd == nil {
		return zero, false, newError("get", key.RedactedID, ErrMalformedResult)
	}
	payload, err := cmd.Bytes()
	if ctxErr := ctx.Err(); ctxErr != nil {
		if err != nil && !errors.Is(err, redis.Nil) {
			wrapped := errors.Join(providerError("get", key.RedactedID, err, false), ctxErr)
			m.logFailure(ctx, "get", key.RedactedID, wrapped)
			return zero, false, wrapped
		}
		return zero, false, ctxErr
	}
	if errors.Is(err, redis.Nil) {
		return zero, false, nil
	}
	if err != nil {
		wrapped := providerError("get", key.RedactedID, err, false)
		m.logFailure(ctx, "get", key.RedactedID, wrapped)
		return zero, false, wrapped
	}
	value, err := m.serializer.Unmarshal(append([]byte(nil), payload...))
	if err != nil {
		wrapped := codecError("get", key.RedactedID, ErrInvalidPayload, err)
		m.logFailure(ctx, "get", key.RedactedID, wrapped)
		return zero, false, wrapped
	}
	return value, true, nil
}

// Set map entry 하나를 직렬화해 persistent 또는 정규화된 TTL로 저장한다.
func (m *MapCache[V]) Set(ctx context.Context, logicalKey string, value V, ttl time.Duration) error {
	key, payload, normalized, err := m.prepareWrite(ctx, "set", logicalKey, value, ttl)
	if err != nil {
		return err
	}
	cmd := m.client.Set(ctx, key.Value, payload, normalized)
	if cmd == nil {
		wrapped := malformedMutation("set", key.RedactedID)
		m.logFailure(ctx, "set", key.RedactedID, wrapped)
		return wrapped
	}
	_, dispatchErr := cmd.Result()
	return m.finishMutation(ctx, "set", key.RedactedID, dispatchErr)
}

// SetIfAbsent logical key가 없을 때만 map entry 하나를 저장한다.
func (m *MapCache[V]) SetIfAbsent(ctx context.Context, logicalKey string, value V, ttl time.Duration) (bool, error) {
	key, payload, normalized, err := m.prepareWrite(ctx, "set-if-absent", logicalKey, value, ttl)
	if err != nil {
		return false, err
	}
	cmd := m.client.SetNX(ctx, key.Value, payload, normalized)
	if cmd == nil {
		wrapped := malformedMutation("set-if-absent", key.RedactedID)
		m.logFailure(ctx, "set-if-absent", key.RedactedID, wrapped)
		return false, wrapped
	}
	ok, dispatchErr := cmd.Result()
	if dispatchErr != nil {
		return false, m.finishMutation(ctx, "set-if-absent", key.RedactedID, dispatchErr)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return ok, nil
}

// GetAndDelete map entry 하나를 atomic하게 반환하고 삭제한다.
func (m *MapCache[V]) GetAndDelete(ctx context.Context, logicalKey string) (V, bool, error) {
	var zero V
	if err := m.ready(); err != nil {
		return zero, false, err
	}
	if err := validateContext(ctx); err != nil {
		return zero, false, err
	}
	key, err := m.key(logicalKey)
	if err != nil {
		return zero, false, err
	}
	cmd := m.client.Eval(ctx, getAndDeleteScript, []string{key.Value})
	if cmd == nil {
		wrapped := malformedMutation("get-and-delete", key.RedactedID)
		m.logFailure(ctx, "get-and-delete", key.RedactedID, wrapped)
		return zero, false, wrapped
	}
	result, dispatchErr := cmd.Result()
	if dispatchErr != nil {
		return zero, false, m.finishMutation(ctx, "get-and-delete", key.RedactedID, dispatchErr)
	}
	if err := ctx.Err(); err != nil {
		return zero, false, err
	}
	status, payload, hit, ok := parseGetAndDeleteResult(result)
	if !ok {
		_ = status
		wrapped := malformedMutation("get-and-delete", key.RedactedID)
		m.logFailure(ctx, "get-and-delete", key.RedactedID, wrapped)
		return zero, false, wrapped
	}
	if !hit {
		return zero, false, nil
	}
	value, err := m.serializer.Unmarshal(payload)
	if err != nil {
		wrapped := codecError("get-and-delete", key.RedactedID, ErrInvalidPayload, err)
		m.logFailure(ctx, "get-and-delete", key.RedactedID, wrapped)
		return zero, false, wrapped
	}
	return value, true, nil
}

// CompareAndSet serialized expected value가 일치할 때만 entry를 atomic하게 교체한다.
func (m *MapCache[V]) CompareAndSet(ctx context.Context, logicalKey string, expected, replacement V, ttl time.Duration) (bool, error) {
	if err := m.ready(); err != nil {
		return false, err
	}
	if err := validateContext(ctx); err != nil {
		return false, err
	}
	key, err := m.key(logicalKey)
	if err != nil {
		return false, err
	}
	normalized, err := normalizeTTL(ttl)
	if err != nil {
		return false, err
	}
	expectedPayload, err := m.serializer.Marshal(expected)
	if err != nil {
		wrapped := codecError("compare-and-set", key.RedactedID, ErrSerialization, err)
		m.logFailure(ctx, "compare-and-set", key.RedactedID, wrapped)
		return false, wrapped
	}
	replacementPayload, err := m.serializer.Marshal(replacement)
	if err != nil {
		wrapped := codecError("compare-and-set", key.RedactedID, ErrSerialization, err)
		m.logFailure(ctx, "compare-and-set", key.RedactedID, wrapped)
		return false, wrapped
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	cmd := m.client.Eval(ctx, compareAndSetScript, []string{key.Value}, payloadOrEmpty(expectedPayload), payloadOrEmpty(replacementPayload), strconv.FormatInt(normalized.Milliseconds(), 10))
	if cmd == nil {
		wrapped := malformedMutation("compare-and-set", key.RedactedID)
		m.logFailure(ctx, "compare-and-set", key.RedactedID, wrapped)
		return false, wrapped
	}
	result, dispatchErr := cmd.Int64()
	if dispatchErr != nil {
		return false, m.finishMutation(ctx, "compare-and-set", key.RedactedID, dispatchErr)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	switch result {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		wrapped := malformedMutation("compare-and-set", key.RedactedID)
		m.logFailure(ctx, "compare-and-set", key.RedactedID, wrapped)
		return false, wrapped
	}
}

// Delete map entry 하나를 idempotently 삭제한다.
func (m *MapCache[V]) Delete(ctx context.Context, logicalKey string) error {
	if err := m.ready(); err != nil {
		return err
	}
	if err := validateContext(ctx); err != nil {
		return err
	}
	key, err := m.key(logicalKey)
	if err != nil {
		return err
	}
	cmd := m.client.Del(ctx, key.Value)
	if cmd == nil {
		wrapped := malformedMutation("delete", key.RedactedID)
		m.logFailure(ctx, "delete", key.RedactedID, wrapped)
		return wrapped
	}
	_, dispatchErr := cmd.Result()
	return m.finishMutation(ctx, "delete", key.RedactedID, dispatchErr)
}

func (m *MapCache[V]) prepareWrite(ctx context.Context, operation, logicalKey string, value V, ttl time.Duration) (btredis.Key, []byte, time.Duration, error) {
	var zero btredis.Key
	if err := m.ready(); err != nil {
		return zero, nil, 0, err
	}
	if err := validateContext(ctx); err != nil {
		return zero, nil, 0, err
	}
	key, err := m.key(logicalKey)
	if err != nil {
		return zero, nil, 0, err
	}
	normalized, err := normalizeTTL(ttl)
	if err != nil {
		return zero, nil, 0, err
	}
	payload, err := m.serializer.Marshal(value)
	if err != nil {
		return zero, nil, 0, codecError(operation, key.RedactedID, ErrSerialization, err)
	}
	if err := ctx.Err(); err != nil {
		return zero, nil, 0, err
	}
	return key, payloadOrEmpty(payload), normalized, nil
}

func (m *MapCache[V]) finishMutation(ctx context.Context, operation, keyID string, dispatchErr error) error {
	if dispatchErr != nil {
		wrapped := providerError(operation, keyID, dispatchErr, true)
		if ctxErr := ctx.Err(); ctxErr != nil {
			wrapped = errors.Join(wrapped, ctxErr)
		}
		m.logFailure(ctx, operation, keyID, wrapped)
		return wrapped
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return nil
}

func (m *MapCache[V]) logFailure(ctx context.Context, operation, keyID string, err error) {
	if m != nil && m.logger != nil {
		m.logger.WarnContext(ctx, "redis mapcache operation failed", "operation", operation, "key_id", keyID, "error_type", causeType(err))
	}
}

func (m *MapCache[V]) key(logicalKey string) (btredis.Key, error) {
	return m.keys.LogicalKey(logicalKey)
}

func (m *MapCache[V]) ready() error {
	if m == nil || isNil(m.client) || isNil(m.serializer) || reflect.ValueOf(m.keys).IsZero() {
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

func payloadOrEmpty(payload []byte) []byte {
	if payload == nil {
		return []byte{}
	}
	return append([]byte(nil), payload...)
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
	status, ok := scriptInt(values[:1])
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
