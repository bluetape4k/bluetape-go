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
	var client redis.Cmdable // redis.NewClient, redis.ClusterClient, or another go-redis client.

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
	var filter BloomFilter[string]

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
	var filter BloomFilter[string]

	if !operatorApproved(ctx, "clear shared Redis Bloom filter") {
		return
	}
	if err := filter.Clear(ctx); err != nil {
		panic(err)
	}
}

func ExampleBloomFilter_diagnostics() {
	ctx := context.Background()
	var filter BloomFilter[string]

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
	}

	err = fmt.Errorf("%w: redacted-key-id", ErrConfigCorrupt)
	if errors.Is(err, ErrConfigCorrupt) {
		// Escalate to the operator runbook before deleting or clearing shared state.
	}

	err = RedisError{Operation: "put", KeyID: "redacted-key-id", Err: context.DeadlineExceeded}
	var redisErr RedisError
	if errors.As(err, &redisErr) {
		// Log redisErr.Operation and the redacted redisErr.KeyID, not inserted values or raw keys.
	}
}

func operatorApproved(context.Context, string) bool {
	return true
}
