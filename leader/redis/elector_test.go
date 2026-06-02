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

	current, err := elector.Leader(ctx)
	if err != nil {
		t.Fatalf("empty leader lookup: %v", err)
	}
	if current != "" {
		t.Fatalf("leader should be empty before campaign, got %q", current)
	}

	if err := elector.Campaign(ctx); err != nil {
		t.Fatalf("campaign: %v", err)
	}
	if !elector.IsLeader() {
		t.Fatal("elector should be leader after campaign")
	}

	current, err = elector.Leader(ctx)
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

func TestRedisElectorRejectsDuplicateCampaign(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		_ = client.Close()
	})

	elector, err := redisleader.New(client, leader.Options{
		Group:    "rejects-duplicate-campaign",
		MemberID: "member-1",
	})
	if err != nil {
		t.Fatalf("new elector: %v", err)
	}

	if err := elector.Campaign(ctx); err != nil {
		t.Fatalf("campaign: %v", err)
	}
	t.Cleanup(func() {
		_ = elector.Resign(context.Background())
	})

	if err := elector.Campaign(ctx); !errors.Is(err, leader.ErrAlreadyLeader) {
		t.Fatalf("duplicate campaign should fail with ErrAlreadyLeader, got %v", err)
	}
	if !elector.IsLeader() {
		t.Fatal("duplicate campaign should not drop ownership")
	}
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

func TestRedisElectorRepeatedResignIsIdempotent(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		_ = client.Close()
	})

	elector, err := redisleader.New(client, leader.Options{
		Group:    "repeated-resign",
		MemberID: "member-1",
	})
	if err != nil {
		t.Fatalf("new elector: %v", err)
	}

	if err := elector.Campaign(ctx); err != nil {
		t.Fatalf("campaign: %v", err)
	}
	if err := elector.Resign(ctx); err != nil {
		t.Fatalf("first resign: %v", err)
	}
	if err := elector.Resign(ctx); err != nil {
		t.Fatalf("second resign: %v", err)
	}

	bttesting.Eventually(t, time.Second, func() bool {
		current, err := elector.Leader(ctx)
		return err == nil && current == ""
	})
}

func TestRedisElectorLosesLeadershipWhenTokenChanges(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		_ = client.Close()
	})

	const (
		group    = "renewal-loss-token-change"
		prefix   = "test:leader"
		newToken = "member-2:stolen"
	)
	elector, err := redisleader.New(client, leader.Options{
		Group:         group,
		MemberID:      "member-1",
		Lease:         time.Second,
		RenewInterval: 50 * time.Millisecond,
		KeyPrefix:     prefix,
	})
	if err != nil {
		t.Fatalf("new elector: %v", err)
	}
	if err := elector.Campaign(ctx); err != nil {
		t.Fatalf("campaign: %v", err)
	}

	if err := client.Set(ctx, prefix+":"+group, newToken, time.Second).Err(); err != nil {
		t.Fatalf("steal leadership key: %v", err)
	}

	bttesting.Eventually(t, time.Second, func() bool {
		return !elector.IsLeader()
	})

	if err := elector.Resign(ctx); err != nil {
		t.Fatalf("resign after lost leadership: %v", err)
	}
	current, err := elector.Leader(ctx)
	if err != nil {
		t.Fatalf("leader after lost leadership resign: %v", err)
	}
	if current != newToken {
		t.Fatalf("resign after lost leadership should not delete new leader, got %q", current)
	}
}

func TestRedisElectorLosesLeadershipWhenRenewalFails(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		_ = client.Close()
	})

	elector, err := redisleader.New(client, leader.Options{
		Group:         "renewal-fails",
		MemberID:      "member-1",
		Lease:         time.Second,
		RenewInterval: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new elector: %v", err)
	}
	if err := elector.Campaign(ctx); err != nil {
		t.Fatalf("campaign: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("close redis client: %v", err)
	}

	bttesting.Eventually(t, time.Second, func() bool {
		return !elector.IsLeader()
	})
}

func TestRedisElectorPreservesWrappedContextError(t *testing.T) {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		_ = client.Close()
	})

	elector, err := redisleader.New(client, leader.Options{
		Group:    "wrapped-context-error",
		MemberID: "member-1",
	})
	if err != nil {
		t.Fatalf("new elector: %v", err)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()

	if err := elector.Campaign(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("campaign should preserve context.Canceled, got %v", err)
	}
	if _, err := elector.Leader(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("leader lookup should preserve context.Canceled, got %v", err)
	}
}
