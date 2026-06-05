package redisleader_test

import (
	"context"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	"github.com/redis/go-redis/v9"
)

func ExampleNewStrategic() {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	defer func() {
		_ = client.Close()
	}()

	elector, err := redisleader.NewStrategic[string](client, leader.Options{
		Group:    "nightly-jobs",
		MemberID: "worker-1",
	})
	if err != nil {
		return
	}

	err = elector.RegisterCandidate(ctx, "nightly-jobs", leader.CandidateInfo{
		NodeID: "worker-1",
	}, 30*time.Second)
	if err != nil {
		return
	}

	strategy := leader.ScoredStrategy{
		Scorer: leader.IdleTimeScorer{},
	}
	_, ran, err := elector.RunIfLeader(ctx, "nightly-jobs", strategy, func(context.Context) (string, error) {
		return "report-created", nil
	})
	if err != nil {
		return
	}

	_ = ran
}
