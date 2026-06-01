package redisleader_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	bttesting "github.com/bluetape4k/bluetape-go/testing"
	"github.com/redis/go-redis/v9"
)

func TestRedisElectorCampaignAndResign(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		_ = client.Close()
	})

	elector, err := redisleader.New(client, leader.Options{
		Group:         "campaign-and-resign",
		MemberID:      "member-1",
		Lease:         2 * time.Second,
		RenewInterval: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new elector: %v", err)
	}

	if err := elector.Campaign(ctx); err != nil {
		t.Fatalf("campaign: %v", err)
	}
	if !elector.IsLeader() {
		t.Fatal("elector should be leader after campaign")
	}

	current, err := elector.Leader(ctx)
	if err != nil {
		t.Fatalf("leader: %v", err)
	}
	if !strings.HasPrefix(current, "member-1:") {
		t.Fatalf("leader token should include member id, got %q", current)
	}

	if err := elector.Resign(ctx); err != nil {
		t.Fatalf("resign: %v", err)
	}

	bttesting.Eventually(t, time.Second, func() bool {
		current, err := elector.Leader(ctx)
		return err == nil && current == ""
	})
}

func TestRedisElectorRejectsSecondLeader(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		_ = client.Close()
	})

	first, err := redisleader.New(client, leader.Options{
		Group:    "rejects-second-leader",
		MemberID: "member-1",
	})
	if err != nil {
		t.Fatalf("new first elector: %v", err)
	}
	second, err := redisleader.New(client, leader.Options{
		Group:    "rejects-second-leader",
		MemberID: "member-2",
	})
	if err != nil {
		t.Fatalf("new second elector: %v", err)
	}

	if err := first.Campaign(ctx); err != nil {
		t.Fatalf("first campaign: %v", err)
	}
	t.Cleanup(func() {
		_ = first.Resign(context.Background())
	})

	if err := second.Campaign(ctx); !errors.Is(err, leader.ErrNotLeader) {
		t.Fatalf("second campaign should fail with ErrNotLeader, got %v", err)
	}
}
