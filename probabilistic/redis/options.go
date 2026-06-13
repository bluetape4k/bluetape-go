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

// Options configures a Redis-backed Bloom filter.
type Options[T any] struct {
	Client    redis.Cmdable
	Namespace string
	Config    probabilistic.Config
	Hasher    probabilistic.Hasher[T]
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
	if strings.Contains(value, "@") || strings.Contains(strings.ToLower(value), "token") {
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
	if strings.Contains(namespace, "@") || strings.Contains(strings.ToLower(namespace), "token") {
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
