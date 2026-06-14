package redisleader_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
)

func TestRedisElectorUsesGoOwnedKeyFormat(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)

	elector, err := redisleader.New(client, leader.Options{
		Group:         "compatibility-lock",
		MemberID:      "go-node",
		Lease:         2 * time.Second,
		RenewInterval: 500 * time.Millisecond,
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

	value, err := client.Get(ctx, "bluetape:leader:compatibility-lock").Result()
	if err != nil {
		t.Fatalf("read go leader key: %v", err)
	}
	if !strings.HasPrefix(value, "go-node:") {
		t.Fatalf("go leader token should start with member id, got %q", value)
	}

	ttl, err := client.PTTL(ctx, "bluetape:leader:compatibility-lock").Result()
	if err != nil {
		t.Fatalf("read go leader key ttl: %v", err)
	}
	if ttl <= 0 {
		t.Fatalf("go leader key should have a positive ttl, got %s", ttl)
	}

	if exists, err := client.Exists(ctx, "compatibility-lock").Result(); err != nil {
		t.Fatalf("check kotlin lettuce-style key: %v", err)
	} else if exists != 0 {
		t.Fatalf("go elector should not write kotlin lettuce-style key, exists=%d", exists)
	}
}
