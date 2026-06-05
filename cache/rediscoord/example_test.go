package rediscoord_test

import (
	"github.com/bluetape4k/bluetape-go/cache"
	"github.com/bluetape4k/bluetape-go/cache/rediscoord"
	"github.com/redis/go-redis/v9"
)

func ExampleNewStampedeCache() {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer func() {
		_ = client.Close()
	}()

	coordinated, err := rediscoord.NewStampedeCache[string](rediscoord.Options[string]{
		Client:    client,
		Cache:     cache.NewMemory[string, string](),
		Namespace: "example",
		Codec:     rediscoord.JSONCodec[string]{},
	})
	if err != nil {
		panic(err)
	}
	_ = coordinated
}
