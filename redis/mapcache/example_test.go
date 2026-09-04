package redismap_test

import (
	"fmt"
	"io"
	"log/slog"

	redismap "github.com/bluetape4k/bluetape-go/redis/mapcache"
	"github.com/bluetape4k/bluetape-go/serialization"
	"github.com/redis/go-redis/v9"
)

func ExampleNew() {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer func() { _ = client.Close() }()
	cache, err := redismap.New(client, redismap.Options[string]{
		Namespace:  "example",
		HashTag:    "tenant-a",
		Serializer: serialization.StringSerializer{},
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		panic(err)
	}
	_ = cache
	fmt.Println("durable Redis map entries; near-cache invalidation stays caller-owned")
	// Output:
	// durable Redis map entries; near-cache invalidation stays caller-owned
}
