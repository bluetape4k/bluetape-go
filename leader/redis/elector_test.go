package redisleader_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	bttesting "github.com/bluetape4k/bluetape-go/testing"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisElectorCampaignAndResign(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)

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
	client := newRedisClient(ctx, t)

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
	client := newRedisClient(ctx, t)

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
	client := newRedisClient(ctx, t)

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
	client := newRedisClient(ctx, t)

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
	client := newRedisClient(ctx, t)

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
	client := newRedisClient(ctx, t)

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

func TestRedisElectorCampaignProviderErrorIsRedactedAndWrapped(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)

	const (
		group  = "redacted-campaign-error"
		prefix = "tenant:leader"
	)
	elector, err := redisleader.New(client, leader.Options{
		Group:     group,
		MemberID:  "member-1",
		KeyPrefix: prefix,
	})
	if err != nil {
		t.Fatalf("new elector: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close redis client: %v", err)
	}

	err = elector.Campaign(ctx)
	if !errors.Is(err, goredis.ErrClosed) {
		t.Fatalf("campaign error should preserve redis.ErrClosed, got %v", err)
	}
	var opErr *btredis.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("campaign error = %T, want *redis.OpError", err)
	}
	if strings.Contains(err.Error(), prefix+":"+group) {
		t.Fatalf("campaign error leaked raw Redis key: %v", err)
	}
}

func TestRedisElectorStoresCanonicalOwnerTokenSuffix(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)

	const memberID = "member-1"
	elector, err := redisleader.New(client, leader.Options{
		Group:    "canonical-owner-token",
		MemberID: memberID,
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

	value, err := elector.Leader(ctx)
	if err != nil {
		t.Fatalf("leader: %v", err)
	}
	suffix, ok := strings.CutPrefix(value, memberID+":")
	if !ok {
		t.Fatalf("leader value = %q, want %q prefix", value, memberID+":")
	}
	if _, err := btredis.ParseOwnerToken(suffix); err != nil {
		t.Fatalf("leader token suffix should be canonical shared owner token: %v", err)
	}
}
