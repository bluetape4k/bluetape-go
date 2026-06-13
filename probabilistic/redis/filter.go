package redisbloom

import (
	"context"

	"github.com/bluetape4k/bluetape-go/probabilistic"
	"github.com/redis/go-redis/v9"
)

const (
	stringHasherKey = "probabilistic:string:v1"
	bytesHasherKey  = "probabilistic:bytes:v1"
)

// BloomFilter exposes metadata for a Redis-backed Bloom filter.
type BloomFilter[T any] interface {
	ExpectedInsertions() uint64
	FalsePositiveProbability() float64
	BitSize() uint64
	HashFunctionCount() uint64
	HasherKey() string
}

type bloomFilter[T any] struct {
	client redis.Cmdable
	keys   redisKeys
	config probabilistic.Config
	hasher probabilistic.Hasher[T]
	meta   metadata
}

// NewBloomFilter creates a Redis-backed Bloom filter from explicit options.
func NewBloomFilter[T any](ctx context.Context, options Options[T]) (BloomFilter[T], error) {
	normalized, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}
	meta := newMetadata(normalized.config, normalized.hasher.Key())
	if err := initializeConfig(ctx, normalized.client, normalized.keys, meta); err != nil {
		return nil, err
	}
	return &bloomFilter[T]{
		client: normalized.client,
		keys:   normalized.keys,
		config: normalized.config,
		hasher: normalized.hasher,
		meta:   meta,
	}, nil
}

// NewStringBloomFilter creates a Redis-backed Bloom filter for string values.
func NewStringBloomFilter(ctx context.Context, client redis.Cmdable, namespace string, cfg probabilistic.Config) (BloomFilter[string], error) {
	hasher, err := probabilistic.NewHasher(stringHasherKey, func(value string) []byte {
		return []byte(value)
	})
	if err != nil {
		return nil, err
	}
	return NewBloomFilter(ctx, Options[string]{
		Client:    client,
		Namespace: namespace,
		Config:    cfg,
		Hasher:    hasher,
	})
}

// NewBytesBloomFilter creates a Redis-backed Bloom filter for byte slices.
func NewBytesBloomFilter(ctx context.Context, client redis.Cmdable, namespace string, cfg probabilistic.Config) (BloomFilter[[]byte], error) {
	hasher, err := probabilistic.NewHasher(bytesHasherKey, func(value []byte) []byte {
		copied := make([]byte, len(value))
		copy(copied, value)
		return copied
	})
	if err != nil {
		return nil, err
	}
	return NewBloomFilter(ctx, Options[[]byte]{
		Client:    client,
		Namespace: namespace,
		Config:    cfg,
		Hasher:    hasher,
	})
}

func (f *bloomFilter[T]) ExpectedInsertions() uint64 {
	return f.config.ExpectedInsertions()
}

func (f *bloomFilter[T]) FalsePositiveProbability() float64 {
	return f.config.FalsePositiveProbability()
}

func (f *bloomFilter[T]) BitSize() uint64 {
	return f.config.BitSize()
}

func (f *bloomFilter[T]) HashFunctionCount() uint64 {
	return f.config.HashFunctionCount()
}

func (f *bloomFilter[T]) HasherKey() string {
	return f.hasher.Key()
}
