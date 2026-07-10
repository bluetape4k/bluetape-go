package redisfory_test

import (
	"context"
	"time"

	"github.com/apache/fory/go/fory"
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
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
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

	_ = valueCache.Set(ctx, "sku:42", exampleValue{Name: "keyboard", Count: 2}, time.Minute)
	_, _ = valueCache.Get(ctx, "sku:42")
	_ = valueCache.Delete(ctx, "sku:42")
}

func ExampleNewNativeCompatible() {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
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

	_ = valueCache.Set(ctx, "sku:42", exampleValue{Name: "keyboard", Count: 2}, time.Minute)
	_, _ = valueCache.Get(ctx, "sku:42")
	_ = valueCache.Delete(ctx, "sku:42")
}
