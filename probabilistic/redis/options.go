package redisbloom

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/probabilistic"
	"github.com/redis/go-redis/v9"
)

const redisMaxBitOffset = uint64(1) << 32

// Options struct 공개 타입이며 Redis Bloom/HyperLogLog key, TTL, script, backend compatibility 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Options[T any] struct {
	// Client 호출자가 소유한 Redis backend client다. 연결 종료와 lifecycle은 호출자가 관리한다.
	Client redis.Cmdable
	// Namespace Redis Bloom/HyperLogLog key를 구분하는 논리 namespace다.
	Namespace string
	// Config capacity, false-positive rate, bit size, hash count compatibility를 정의한다.
	Config probabilistic.Config
	// Hasher T 값을 deterministic byte input으로 바꾸는 compatibility anchor다.
	Hasher probabilistic.Hasher[T]
}

type normalizedOptions[T any] struct {
	client redis.Cmdable
	keys   redisKeys
	config probabilistic.Config
	hasher probabilistic.Hasher[T]
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func normalizeOptions[T any](options Options[T]) (normalizedOptions[T], error) {
	if isNilClient(options.Client) {
		return normalizedOptions[T]{}, fmt.Errorf("%w: nil redis client", ErrInvalidOptions)
	}
	if reflect.ValueOf(options.Config).IsZero() {
		return normalizedOptions[T]{}, fmt.Errorf("%w: invalid config", ErrInvalidOptions)
	}
	if options.Config.ExpectedInsertions() == 0 || options.Config.FalsePositiveProbability() <= 0 ||
		options.Config.BitSize() == 0 || options.Config.HashFunctionCount() == 0 {
		return normalizedOptions[T]{}, fmt.Errorf("%w: invalid config", ErrInvalidOptions)
	}
	if options.Config.BitSize() >= redisMaxBitOffset {
		return normalizedOptions[T]{}, fmt.Errorf("%w: bit size exceeds redis bitmap offset limit", ErrInvalidOptions)
	}
	if err := validateIdentifier("hasher key", options.Hasher.Key()); err != nil {
		return normalizedOptions[T]{}, fmt.Errorf("%w: %w", ErrInvalidOptions, err)
	}
	keys, err := buildKeys(options.Namespace)
	if err != nil {
		return normalizedOptions[T]{}, fmt.Errorf("%w: namespace: %w", ErrInvalidOptions, err)
	}
	return normalizedOptions[T]{client: options.Client, keys: keys, config: options.Config, hasher: options.Hasher}, nil
}

func isNilClient(client redis.Cmdable) bool {
	if client == nil {
		return true
	}
	value := reflect.ValueOf(client)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func validateIdentifier(kind, value string) error {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 128 || !utf8.ValidString(value) {
		return fmt.Errorf("%s: invalid length or whitespace", kind)
	}
	if containsSensitiveMarker(value) {
		return fmt.Errorf("%s: use a non-sensitive schema identifier", kind)
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' || r == ':' {
			continue
		}
		return fmt.Errorf("%s: invalid character %q", kind, r)
	}
	return nil
}

func validateNamespace(namespace string) error {
	if namespace == "" {
		return fmt.Errorf("empty")
	}
	if namespace != strings.TrimSpace(namespace) || len(namespace) > 128 || !utf8.ValidString(namespace) {
		return fmt.Errorf("invalid length or whitespace")
	}
	if strings.HasPrefix(namespace, ":") || strings.HasSuffix(namespace, ":") {
		return fmt.Errorf("invalid colon boundary")
	}
	if strings.HasSuffix(namespace, ":bits") || strings.HasSuffix(namespace, ":config") {
		return fmt.Errorf("reserved suffix")
	}
	if containsSensitiveMarker(namespace) {
		return fmt.Errorf("use a non-sensitive namespace")
	}
	for _, r := range namespace {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' || r == ':' {
			continue
		}
		return fmt.Errorf("invalid character %q", r)
	}
	return nil
}

func containsSensitiveMarker(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"@", "token", "secret", "password", "credential"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	normalized := strings.NewReplacer("-", "", "_", "", ".", "", ":", "").Replace(lower)
	return strings.Contains(normalized, "apikey")
}
