package redisbloom

import (
	"context"
	"math"

	"github.com/bluetape4k/bluetape-go/probabilistic"
	"github.com/bluetape4k/bluetape-go/probabilistic/internal/bloomhash"
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
	BitCount(ctx context.Context) (uint64, error)
	IsEmpty(ctx context.Context) (bool, error)
	MightContain(ctx context.Context, value T) (bool, error)
	Put(ctx context.Context, value T) (bool, error)
	ApproximateElementCount(ctx context.Context) (uint64, error)
	ExpectedFPP(ctx context.Context) (float64, error)
	Clear(ctx context.Context) error
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

func (f *bloomFilter[T]) offsets(value T) ([]any, error) {
	bytes, err := f.hasher.Bytes(value)
	if err != nil {
		return nil, err
	}
	args := make([]any, 0, f.config.HashFunctionCount()+1)
	args = append(args, f.meta.fingerprint)
	return bloomhash.AppendIndexes(args, bytes, f.config.HashFunctionCount(), f.config.BitSize()), nil
}

func (f *bloomFilter[T]) Put(ctx context.Context, value T) (bool, error) {
	args, err := f.offsets(value)
	if err != nil {
		return false, err
	}
	result, err := putScript.Run(normalizeContext(ctx), f.client, []string{f.keys.bits, f.keys.config}, args...).Int()
	if err != nil {
		return false, mapScriptError("put", f.keys.redactedID, err)
	}
	return result == 1, nil
}

func (f *bloomFilter[T]) MightContain(ctx context.Context, value T) (bool, error) {
	args, err := f.offsets(value)
	if err != nil {
		return false, err
	}
	result, err := mightContainScript.Run(normalizeContext(ctx), f.client, []string{f.keys.bits, f.keys.config}, args...).Int()
	if err != nil {
		return false, mapScriptError("might contain", f.keys.redactedID, err)
	}
	return result == 1, nil
}

func (f *bloomFilter[T]) Clear(ctx context.Context) error {
	if err := clearScript.Run(normalizeContext(ctx), f.client, []string{f.keys.bits, f.keys.config}, f.meta.fingerprint).Err(); err != nil {
		return mapScriptError("clear", f.keys.redactedID, err)
	}
	return nil
}

func (f *bloomFilter[T]) BitCount(ctx context.Context) (uint64, error) {
	lastByte := (f.config.BitSize() - 1) / 8
	result, err := bitCountScript.Run(normalizeContext(ctx), f.client, []string{f.keys.bits, f.keys.config}, f.meta.fingerprint, lastByte).Int64()
	if err != nil {
		return 0, mapScriptError("bit count", f.keys.redactedID, err)
	}
	return uint64(result), nil
}

func (f *bloomFilter[T]) IsEmpty(ctx context.Context) (bool, error) {
	result, err := isEmptyScript.Run(normalizeContext(ctx), f.client, []string{f.keys.bits, f.keys.config}, f.meta.fingerprint).Int64()
	if err != nil {
		return false, mapScriptError("is empty", f.keys.redactedID, err)
	}
	return result == 0, nil
}

func (f *bloomFilter[T]) ApproximateElementCount(ctx context.Context) (uint64, error) {
	bitCount, err := f.BitCount(ctx)
	if err != nil {
		return 0, err
	}
	return approximateElementCount(bitCount, f.config.BitSize(), f.config.HashFunctionCount()), nil
}

func (f *bloomFilter[T]) ExpectedFPP(ctx context.Context) (float64, error) {
	bitCount, err := f.BitCount(ctx)
	if err != nil {
		return 0, err
	}
	return expectedFPP(bitCount, f.config.BitSize(), f.config.HashFunctionCount()), nil
}

func expectedFPP(bitCount uint64, bitSize uint64, hashFunctionCount uint64) float64 {
	if bitCount == 0 || bitSize == 0 || hashFunctionCount == 0 {
		return 0
	}
	fillRatio := float64(bitCount) / float64(bitSize)
	return math.Pow(fillRatio, float64(hashFunctionCount))
}

func approximateElementCount(bitCount uint64, bitSize uint64, hashFunctionCount uint64) uint64 {
	if bitCount == 0 || bitSize == 0 || hashFunctionCount == 0 {
		return 0
	}
	if bitCount >= bitSize {
		return math.MaxUint64
	}
	fraction := 1 - float64(bitCount)/float64(bitSize)
	estimate := math.Ceil(-(float64(bitSize) / float64(hashFunctionCount)) * math.Log(fraction))
	if math.IsInf(estimate, 0) || math.IsNaN(estimate) || estimate >= float64(math.MaxUint64) {
		return math.MaxUint64
	}
	return uint64(estimate)
}
