package redisbloom

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/bluetape4k/bluetape-go/probabilistic"
	"github.com/redis/go-redis/v9"
)

// HyperLogLog estimates the cardinality of caller values in Redis.
//
// Values are transformed through the configured probabilistic.Hasher and then
// stored as SHA-256 hex digests, so Redis receives stable identifiers rather
// than raw caller values.
type HyperLogLog[T any] interface {
	HasherKey() string
	Add(ctx context.Context, values ...T) (bool, error)
	Count(ctx context.Context) (uint64, error)
	Merge(ctx context.Context, sourceNamespaces ...string) error
}

// HyperLogLogOptions configures a Redis-backed HyperLogLog.
type HyperLogLogOptions[T any] struct {
	Client    redis.Cmdable
	Namespace string
	Hasher    probabilistic.Hasher[T]
}

type hyperLogLog[T any] struct {
	client redis.Cmdable
	key    hyperLogLogKey
	hasher probabilistic.Hasher[T]
}

type normalizedHyperLogLogOptions[T any] struct {
	client redis.Cmdable
	key    hyperLogLogKey
	hasher probabilistic.Hasher[T]
}

// NewHyperLogLog creates a Redis-backed HyperLogLog from explicit options.
func NewHyperLogLog[T any](options HyperLogLogOptions[T]) (HyperLogLog[T], error) {
	normalized, err := normalizeHyperLogLogOptions(options)
	if err != nil {
		return nil, err
	}
	return &hyperLogLog[T]{
		client: normalized.client,
		key:    normalized.key,
		hasher: normalized.hasher,
	}, nil
}

// NewStringHyperLogLog creates a Redis-backed HyperLogLog for string values.
func NewStringHyperLogLog(client redis.Cmdable, namespace string) (HyperLogLog[string], error) {
	hasher, err := probabilistic.NewHasher(stringHasherKey, func(value string) []byte {
		return []byte(value)
	})
	if err != nil {
		return nil, err
	}
	return NewHyperLogLog(HyperLogLogOptions[string]{
		Client:    client,
		Namespace: namespace,
		Hasher:    hasher,
	})
}

// NewBytesHyperLogLog creates a Redis-backed HyperLogLog for byte slices.
func NewBytesHyperLogLog(client redis.Cmdable, namespace string) (HyperLogLog[[]byte], error) {
	hasher, err := probabilistic.NewHasher(bytesHasherKey, func(value []byte) []byte {
		copied := make([]byte, len(value))
		copy(copied, value)
		return copied
	})
	if err != nil {
		return nil, err
	}
	return NewHyperLogLog(HyperLogLogOptions[[]byte]{
		Client:    client,
		Namespace: namespace,
		Hasher:    hasher,
	})
}

func normalizeHyperLogLogOptions[T any](options HyperLogLogOptions[T]) (normalizedHyperLogLogOptions[T], error) {
	if isNilClient(options.Client) {
		return normalizedHyperLogLogOptions[T]{}, fmt.Errorf("%w: nil redis client", ErrInvalidOptions)
	}
	if err := validateIdentifier("hasher key", options.Hasher.Key()); err != nil {
		return normalizedHyperLogLogOptions[T]{}, fmt.Errorf("%w: %w", ErrInvalidOptions, err)
	}
	key, err := buildHyperLogLogKey(options.Namespace)
	if err != nil {
		return normalizedHyperLogLogOptions[T]{}, fmt.Errorf("%w: namespace: %w", ErrInvalidOptions, err)
	}
	return normalizedHyperLogLogOptions[T]{
		client: options.Client,
		key:    key,
		hasher: options.Hasher,
	}, nil
}

func (h *hyperLogLog[T]) HasherKey() string {
	return h.hasher.Key()
}

func (h *hyperLogLog[T]) Add(ctx context.Context, values ...T) (bool, error) {
	if len(values) == 0 {
		return false, nil
	}
	elements, err := h.digestValues(values)
	if err != nil {
		return false, err
	}
	changed, err := h.client.PFAdd(normalizeContext(ctx), h.key.key, elements...).Result()
	if err != nil {
		return false, mapRedisError("redis hll", "add", h.key.redactedID, err)
	}
	return changed == 1, nil
}

func (h *hyperLogLog[T]) Count(ctx context.Context) (uint64, error) {
	count, err := h.client.PFCount(normalizeContext(ctx), h.key.key).Result()
	if err != nil {
		return 0, mapRedisError("redis hll", "count", h.key.redactedID, err)
	}
	return uint64(count), nil
}

func (h *hyperLogLog[T]) Merge(ctx context.Context, sourceNamespaces ...string) error {
	if len(sourceNamespaces) == 0 {
		return fmt.Errorf("%w: hll merge sources empty", ErrInvalidOptions)
	}
	sourceKeys := make([]string, 0, len(sourceNamespaces)+1)
	sourceKeys = append(sourceKeys, h.key.key)
	for _, namespace := range sourceNamespaces {
		key, err := buildHyperLogLogKey(namespace)
		if err != nil {
			return fmt.Errorf("%w: source namespace: %w", ErrInvalidOptions, err)
		}
		sourceKeys = append(sourceKeys, key.key)
	}
	if err := h.client.PFMerge(normalizeContext(ctx), h.key.key, sourceKeys...).Err(); err != nil {
		return mapRedisError("redis hll", "merge", h.key.redactedID, err)
	}
	return nil
}

func (h *hyperLogLog[T]) digestValues(values []T) ([]any, error) {
	elements := make([]any, 0, len(values))
	for _, value := range values {
		bytes, err := h.hasher.Bytes(value)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(bytes)
		elements = append(elements, hex.EncodeToString(sum[:]))
	}
	return elements, nil
}
