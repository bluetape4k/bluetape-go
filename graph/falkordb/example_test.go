package falkordb_test

import (
	"context"

	"github.com/bluetape4k/bluetape-go/graph/falkordb"
	"github.com/redis/go-redis/v9"
)

func ExampleNewClient() {
	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	client, _ := falkordb.NewClient(redisClient, "example_graph")
	_, _ = client.Query(context.Background(), "RETURN 1", nil)
	// The Redis client remains caller-owned and must be closed by the caller.
	// Output:
}
