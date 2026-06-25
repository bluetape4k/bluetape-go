package redisleader_test

import (
	"context"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	"github.com/redis/go-redis/v9"
)

func ExampleNew() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer func() {
		_ = client.Close()
	}()

	elector, err := redisleader.New(client, leader.Options{
		Group:         "billing-workers",
		MemberID:      "worker-1",
		Lease:         30 * time.Second,
		RenewInterval: 10 * time.Second,
	})
	if err != nil {
		return
	}

	if err := elector.Campaign(ctx); err != nil {
		return
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = elector.Resign(cleanupCtx)
	}()

	_ = elector.IsLeader()
}
