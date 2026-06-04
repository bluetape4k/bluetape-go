package redisnear_test

import (
	"context"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/cache/redisnear"
	"github.com/redis/go-redis/v9"
)

func ExampleNewPubSub() {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer func() {
		_ = client.Close()
	}()

	near, err := redisnear.NewPubSub[string](ctx, redisnear.Options[string]{
		Client:    client,
		Namespace: "catalog",
	})
	if err != nil {
		return
	}
	defer func() {
		_ = near.Close()
	}()

	value, err := near.GetOrLoad(ctx, "item:1", time.Minute,
		func(context.Context, string) (string, error) {
			return "catalog-value", nil
		},
	)
	if err != nil {
		return
	}
	fmt.Println(value)
}
