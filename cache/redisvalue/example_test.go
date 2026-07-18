package redisvalue_test

import (
	"context"
	"errors"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	"github.com/bluetape4k/bluetape-go/cache/redisvalue"
	"github.com/bluetape4k/bluetape-go/serialization"
	"github.com/redis/go-redis/v9"
)

type exampleValue struct {
	Name string `json:"name"`
}

func ExampleNewTieredCache() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", DialTimeout: 2 * time.Second,
		ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second,
	})
	defer func() { _ = client.Close() }()

	serializer, err := serialization.NewVersionedSerializer(
		serialization.NewJSONSerializer[*exampleValue](), 1,
	)
	if err != nil {
		return
	}
	config := redisvalue.DefaultConfig()
	config.Value.RemoteTTL = 10 * time.Minute
	config.Tiered.LocalTTL = time.Minute
	remote, err := redisvalue.NewValueCache(redisvalue.ValueOptions[*exampleValue]{
		Client: client, Namespace: "catalog", Serializer: serializer, Config: &config.Value,
	})
	if err != nil {
		return
	}
	tiered, err := redisvalue.NewTieredCache(redisvalue.TieredOptions[*exampleValue]{
		Local: cache.NewMemory[string, *exampleValue](), Remote: remote, Config: &config.Tiered,
	})
	if err != nil {
		return
	}

	// Pointer-valued entries are immutable snapshots while cached. L1 retains
	// the reference directly; only the Redis L2 serializer sees the value.
	value, err := tiered.GetOrLoadDefault(ctx, "sku:42", func(context.Context, string) (*exampleValue, error) {
		return &exampleValue{Name: "keyboard"}, nil
	})
	if err != nil {
		return
	}
	_ = value
	_ = tiered.Set(ctx, "sku:43", &exampleValue{Name: "mouse"}, 30*time.Second)

	// InvalidateLocal affects this decorator only. Clear scans and unlinks the
	// Redis namespace, then clears this decorator's L1.
	_ = tiered.InvalidateLocal(ctx, "sku:42")
	_ = tiered.ClearLocal(ctx)
	if err := tiered.Clear(ctx); err != nil {
		var cacheErr *redisvalue.CacheError
		if errors.As(err, &cacheErr) && cacheErr.Reason() == redisvalue.ReasonLocalBlocked {
			// Stop serving through this decorator until explicit local repair succeeds.
			_ = tiered.ClearLocal(ctx)
		}
	}
}
