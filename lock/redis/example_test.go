package redislock_test

import (
	"context"
	"time"

	redislock "github.com/bluetape4k/bluetape-go/lock/redis"
	"github.com/redis/go-redis/v9"
)

func ExampleNew() {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer func() {
		_ = client.Close()
	}()

	mutex, err := redislock.New(client, redislock.Options{
		Key: "locks:billing-rollup",
		TTL: 30 * time.Second,
	})
	if err != nil {
		return
	}

	lease, err := mutex.TryLock(ctx)
	if err != nil {
		return
	}
	defer func() {
		_, _ = lease.Unlock(context.Background())
	}()

	// protected work runs here
}
