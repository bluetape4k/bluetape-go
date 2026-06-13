package redisbloom

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluetape4k/bluetape-go/probabilistic"
	"github.com/redis/go-redis/v9"
)

func ExampleNewStringBloomFilter() {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer func() {
		_ = client.Close()
	}()

	cfg, err := probabilistic.NewConfig(1_000_000, 0.01)
	if err != nil {
		panic(err)
	}

	filter, err := NewStringBloomFilter(ctx, client, "auth:tenant-a:login-attempts", cfg)
	if err != nil {
		panic(err)
	}
	_ = filter
}

func ExampleBloomFilter_Put_falseNotDuplicate() {
	ctx := context.Background()
	filter := exampleFilter()

	changed, err := filter.Put(ctx, "candidate-key")
	if err != nil {
		panic(err)
	}
	if !changed {
		// All hashed bits were already set. This is not duplicate certainty.
		maybeSeen, err := filter.MightContain(ctx, "candidate-key")
		if err != nil {
			panic(err)
		}
		_ = maybeSeen
	}
}

func ExampleBloomFilter_Clear_adminOnly() {
	ctx := context.Background()
	filter := exampleFilter()

	if !operatorApproved(ctx, "clear shared Redis Bloom filter") {
		return
	}
	if err := filter.Clear(ctx); err != nil {
		panic(err)
	}
}

func ExampleBloomFilter_diagnostics() {
	ctx := context.Background()
	filter := exampleFilter()

	bitsSet, err := filter.BitCount(ctx)
	if err != nil {
		panic(err)
	}
	estimate, err := filter.ApproximateElementCount(ctx)
	if err != nil {
		panic(err)
	}
	fpp, err := filter.ExpectedFPP(ctx)
	if err != nil {
		panic(err)
	}
	_, _, _ = bitsSet, estimate, fpp
}

func Example_errors() {
	err := fmt.Errorf("%w: redacted-key-id", ErrConfigMismatch)
	if errors.Is(err, ErrConfigMismatch) {
		// Inspect stored metadata, then rebuild or switch readers to a new namespace.
		_ = "switch readers after verification"
	}

	err = fmt.Errorf("%w: redacted-key-id", ErrConfigCorrupt)
	if errors.Is(err, ErrConfigCorrupt) {
		// Escalate to the operator runbook before deleting or clearing shared state.
		_ = "operator runbook required"
	}

	err = RedisError{Operation: "put", KeyID: "redacted-key-id", Err: context.DeadlineExceeded}
	var redisErr RedisError
	if errors.As(err, &redisErr) {
		// Log redisErr.Operation and the redacted redisErr.KeyID, not inserted values or raw keys.
		_, _ = redisErr.Operation, redisErr.KeyID
	}
}

func operatorApproved(context.Context, string) bool {
	return true
}

type exampleBloomFilter struct{}

func exampleFilter() BloomFilter[string] {
	return exampleBloomFilter{}
}

func (exampleBloomFilter) ExpectedInsertions() uint64 {
	return 1_000_000
}

func (exampleBloomFilter) FalsePositiveProbability() float64 {
	return 0.01
}

func (exampleBloomFilter) BitSize() uint64 {
	return 9_585_059
}

func (exampleBloomFilter) HashFunctionCount() uint64 {
	return 7
}

func (exampleBloomFilter) HasherKey() string {
	return "probabilistic:string:v1"
}

func (exampleBloomFilter) BitCount(context.Context) (uint64, error) {
	return 42, nil
}

func (exampleBloomFilter) IsEmpty(context.Context) (bool, error) {
	return false, nil
}

func (exampleBloomFilter) MightContain(context.Context, string) (bool, error) {
	return true, nil
}

func (exampleBloomFilter) Put(context.Context, string) (bool, error) {
	return false, nil
}

func (exampleBloomFilter) ApproximateElementCount(context.Context) (uint64, error) {
	return 6, nil
}

func (exampleBloomFilter) ExpectedFPP(context.Context) (float64, error) {
	return 0.000000000001, nil
}

func (exampleBloomFilter) Clear(context.Context) error {
	return nil
}
