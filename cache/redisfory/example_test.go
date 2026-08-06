package redisfory_test

import (
	"context"
	"errors"
	"time"

	"github.com/apache/fory/go/fory"
	"github.com/bluetape4k/bluetape-go/cache"
	"github.com/bluetape4k/bluetape-go/cache/redisfory"
	"github.com/redis/go-redis/v9"
)

type exampleValue struct {
	Name  string
	Count int
}

func registerExampleValue(runtime *fory.Fory) error {
	return runtime.RegisterStructByName(exampleValue{}, "redisfory.example.Value")
}

func ExampleNewNativeFast() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client := redis.NewClient(&redis.Options{
		Addr:         "localhost:6379",
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	defer func() { _ = client.Close() }()

	valueCache, err := redisfory.NewNativeFast[exampleValue](redisfory.Options{
		Client:           client,
		Namespace:        "catalog.products",
		SchemaGeneration: 1,
		Register:         registerExampleValue,
	})
	if err != nil {
		return
	}

	if err := valueCache.Set(ctx, "sku:42", exampleValue{Name: "keyboard", Count: 2}, time.Minute); err != nil {
		return
	}
	loaded, err := valueCache.Get(ctx, "sku:42")
	if errors.Is(err, cache.ErrCacheMiss) {
		return
	}
	if err != nil {
		return
	}
	_ = loaded
	if err := valueCache.Delete(ctx, "sku:42"); err != nil {
		return
	}
}

func ExampleNewNativeCompatible() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client := redis.NewClient(&redis.Options{
		Addr:         "localhost:6379",
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	defer func() { _ = client.Close() }()

	valueCache, err := redisfory.NewNativeCompatible[exampleValue](redisfory.Options{
		Client:           client,
		Namespace:        "catalog.compatible",
		SchemaGeneration: 1,
		Register:         registerExampleValue,
	})
	if err != nil {
		return
	}

	if err := valueCache.Set(ctx, "sku:42", exampleValue{Name: "keyboard", Count: 2}, time.Minute); err != nil {
		return
	}
	loaded, err := valueCache.Get(ctx, "sku:42")
	if errors.Is(err, cache.ErrCacheMiss) {
		return
	}
	if err != nil {
		return
	}
	_ = loaded
	if err := valueCache.Delete(ctx, "sku:42"); err != nil {
		return
	}
}
