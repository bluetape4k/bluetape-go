package redisleader_test

import (
	"context"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	"github.com/redis/go-redis/v9"
)

func ExampleNewGroup() {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer func() {
		_ = client.Close()
	}()

	elector, err := redisleader.NewGroup(client, leader.GroupOptions{
		Options: leader.Options{
			Group:         "batch-workers",
			MemberID:      "worker-1",
			Lease:         30 * time.Second,
			RenewInterval: 10 * time.Second,
		},
		MaxLeaders: 3,
	})
	if err != nil {
		return
	}

	if err := elector.Campaign(ctx); err != nil {
		return
	}
	defer func() {
		_ = elector.Resign(context.Background())
	}()

	_ = elector.IsLeader()
}
