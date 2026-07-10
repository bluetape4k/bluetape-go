package redisstream_test

import (
	"context"
	"time"

	redisstream "github.com/bluetape4k/bluetape-go/redis/stream"
	redis "github.com/redis/go-redis/v9"
)

func ExampleAppend() {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer func() {
		_ = client.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := redisstream.Append(ctx, client, redis.XAddArgs{
		Stream: "orders",
		Values: map[string]any{
			"event_id": "evt-42",
			"kind":     "created",
		},
	})
	if err != nil {
		return
	}
}
