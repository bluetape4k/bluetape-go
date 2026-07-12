package redislock_test

import (
	"context"
	"errors"
	"time"

	redislock "github.com/bluetape4k/bluetape-go/lock/redis"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

func ExampleNew() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
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
	if errors.Is(err, btredis.ErrCommitUnknown) && lease != nil {
		_ = reconcileExampleLease(lease)
		return
	}
	if err != nil {
		return
	}
	defer func() {
		_ = reconcileExampleLease(lease)
	}()

	// protected work runs here
}

func reconcileExampleLease(lease *redislock.Lease) error {
	var firstErr error
	for range 2 {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := lease.Unlock(cleanupCtx)
		cancel()
		if err == nil {
			return nil // released or already absent after a lost response
		}
		if !errors.Is(err, btredis.ErrCommitUnknown) {
			return errors.Join(firstErr, err)
		}
		firstErr = errors.Join(firstErr, err)
	}
	return firstErr // lease TTL is the final fallback
}
