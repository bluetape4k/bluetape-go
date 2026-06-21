package redisleader_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	bttesting "github.com/bluetape4k/bluetape-go/testing"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/redis/go-redis/v9"
)

func TestRedisGroupElectorAllowsMaxLeaders(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)

	first := newGroupElector(t, client, "allows-max-leaders", "member-1", 2)
	second := newGroupElector(t, client, "allows-max-leaders", "member-2", 2)

	if err := first.Campaign(ctx); err != nil {
		t.Fatalf("first campaign: %v", err)
	}
	t.Cleanup(func() {
		_ = first.Resign(context.Background())
	})
	if err := second.Campaign(ctx); err != nil {
		t.Fatalf("second campaign: %v", err)
	}
	t.Cleanup(func() {
		_ = second.Resign(context.Background())
	})

	active, err := first.ActiveCount(ctx)
	if err != nil {
		t.Fatalf("active count: %v", err)
	}
	if active != 2 {
		t.Fatalf("active count should be 2, got %d", active)
	}
	available, err := first.AvailableSlots(ctx)
	if err != nil {
		t.Fatalf("available slots: %v", err)
	}
	if available != 0 {
		t.Fatalf("available slots should be 0, got %d", available)
	}
}

func TestRedisGroupElectorWaitsUntilContextExpires(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)

	first := newGroupElector(t, client, "waits-until-context-expires", "member-1", 1)
	second := newGroupElector(t, client, "waits-until-context-expires", "member-2", 1)

	if err := first.Campaign(ctx); err != nil {
		t.Fatalf("first campaign: %v", err)
	}
	t.Cleanup(func() {
		_ = first.Resign(context.Background())
	})

	waitCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	if err := second.Campaign(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second campaign should preserve deadline error, got %v", err)
	}
	if second.IsLeader() {
		t.Fatal("second elector should not become leader")
	}
}

func TestRedisGroupElectorRejectsDuplicateCampaign(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)

	elector := newGroupElector(t, client, "rejects-duplicate-group-campaign", "member-1", 2)
	if err := elector.Campaign(ctx); err != nil {
		t.Fatalf("campaign: %v", err)
	}
	t.Cleanup(func() {
		_ = elector.Resign(context.Background())
	})

	if err := elector.Campaign(ctx); !errors.Is(err, leader.ErrAlreadyLeader) {
		t.Fatalf("duplicate campaign should fail with ErrAlreadyLeader, got %v", err)
	}
}

func TestRedisGroupElectorRepeatedResignIsIdempotent(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)

	elector := newGroupElector(t, client, "group-repeated-resign", "member-1", 1)
	if err := elector.Campaign(ctx); err != nil {
		t.Fatalf("campaign: %v", err)
	}
	if err := elector.Resign(ctx); err != nil {
		t.Fatalf("first resign: %v", err)
	}
	if err := elector.Resign(ctx); err != nil {
		t.Fatalf("second resign: %v", err)
	}

	active, err := elector.ActiveCount(ctx)
	if err != nil {
		t.Fatalf("active count: %v", err)
	}
	if active != 0 {
		t.Fatalf("active count should be 0 after resign, got %d", active)
	}
}

func TestRedisGroupElectorKeepsOtherSlotsOnResign(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)

	first := newGroupElector(t, client, "keeps-other-slots-on-resign", "member-1", 2)
	second := newGroupElector(t, client, "keeps-other-slots-on-resign", "member-2", 2)
	if err := first.Campaign(ctx); err != nil {
		t.Fatalf("first campaign: %v", err)
	}
	if err := second.Campaign(ctx); err != nil {
		t.Fatalf("second campaign: %v", err)
	}
	t.Cleanup(func() {
		_ = second.Resign(context.Background())
	})

	if err := first.Resign(ctx); err != nil {
		t.Fatalf("first resign: %v", err)
	}
	active, err := second.ActiveCount(ctx)
	if err != nil {
		t.Fatalf("active count: %v", err)
	}
	if active != 1 {
		t.Fatalf("resigning one elector should keep the other slot, active=%d", active)
	}
}

func TestRedisGroupElectorReclaimsExpiredSlots(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)

	const (
		group = "reclaims-expired-slots"
		key   = "bluetape:leader-group:" + group
	)
	if err := client.ZAdd(ctx, key, redis.Z{Score: 1, Member: "stale-member:token"}).Err(); err != nil {
		t.Fatalf("seed stale slot: %v", err)
	}

	elector := newGroupElector(t, client, group, "member-1", 1)
	if err := elector.Campaign(ctx); err != nil {
		t.Fatalf("campaign should reclaim stale slot: %v", err)
	}
	t.Cleanup(func() {
		_ = elector.Resign(context.Background())
	})

	active, err := elector.ActiveCount(ctx)
	if err != nil {
		t.Fatalf("active count: %v", err)
	}
	if active != 1 {
		t.Fatalf("active count should be 1 after reclaim, got %d", active)
	}
}

func TestRedisGroupElectorLosesLeadershipWhenTokenRemoved(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)

	const (
		group = "group-renewal-loss-token-removed"
		key   = "bluetape:leader-group:" + group
	)
	elector, err := redisleader.NewGroup(client, leader.GroupOptions{
		Options: leader.Options{
			Group:         group,
			MemberID:      "member-1",
			Lease:         time.Second,
			RenewInterval: 50 * time.Millisecond,
		},
		MaxLeaders: 1,
	})
	if err != nil {
		t.Fatalf("new group elector: %v", err)
	}
	if err := elector.Campaign(ctx); err != nil {
		t.Fatalf("campaign: %v", err)
	}

	tokens, err := client.ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		t.Fatalf("read group tokens: %v", err)
	}
	if len(tokens) != 1 || !strings.HasPrefix(tokens[0], "member-1:") {
		t.Fatalf("expected one member token, got %v", tokens)
	}
	if err := client.ZRem(ctx, key, tokens[0]).Err(); err != nil {
		t.Fatalf("remove token: %v", err)
	}

	bttesting.Eventually(t, time.Second, func() bool {
		return !elector.IsLeader()
	})
}

func TestRedisGroupElectorUsesGoOwnedKeyFormat(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)

	elector := newGroupElector(t, client, "compatibility-group", "go-node", 2)
	if err := elector.Campaign(ctx); err != nil {
		t.Fatalf("campaign: %v", err)
	}
	t.Cleanup(func() {
		_ = elector.Resign(context.Background())
	})

	tokens, err := client.ZRange(ctx, "bluetape:leader-group:compatibility-group", 0, -1).Result()
	if err != nil {
		t.Fatalf("read go leader group key: %v", err)
	}
	if len(tokens) != 1 || !strings.HasPrefix(tokens[0], "go-node:") {
		t.Fatalf("go group token should start with member id, got %v", tokens)
	}

	if exists, err := client.Exists(ctx, "lg:{compatibility-group}").Result(); err != nil {
		t.Fatalf("check kotlin group key: %v", err)
	} else if exists != 0 {
		t.Fatalf("go group elector should not write kotlin-style group key, exists=%d", exists)
	}
}

func TestRedisGroupElectorStressBoundsConcurrentLeaders(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)

	const (
		group      = "stress-bounds-concurrent-leaders"
		maxLeaders = 3
		members    = 10
	)
	var running int32
	var maxRunning int32
	var sequence int32
	tasks := make([]concurrencytest.Task, 0, members)
	for i := 0; i < members; i++ {
		taskIndex := i

		tasks = append(tasks, func(ctx context.Context) error {
			memberID := fmt.Sprintf("member-%02d-%04d", taskIndex, atomic.AddInt32(&sequence, 1))
			elector, err := redisleader.NewGroup(client, leader.GroupOptions{
				Options: leader.Options{
					Group:         group,
					MemberID:      memberID,
					Lease:         2 * time.Second,
					RenewInterval: 100 * time.Millisecond,
				},
				MaxLeaders: maxLeaders,
			})
			if err != nil {
				return fmt.Errorf("new group elector %s: %w", memberID, err)
			}

			campaignCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if err := elector.Campaign(campaignCtx); err != nil {
				return fmt.Errorf("%s campaign: %w", memberID, err)
			}
			defer func() {
				resignCtx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_ = elector.Resign(resignCtx)
			}()

			current := atomic.AddInt32(&running, 1)
			defer atomic.AddInt32(&running, -1)
			for {
				observed := atomic.LoadInt32(&maxRunning)
				if current <= observed || atomic.CompareAndSwapInt32(&maxRunning, observed, current) {
					break
				}
			}
			if current > maxLeaders {
				return fmt.Errorf("running leaders exceeded max: %d > %d", current, maxLeaders)
			}

			active, err := elector.ActiveCount(ctx)
			if err != nil {
				return fmt.Errorf("%s active count: %w", memberID, err)
			}
			if active > maxLeaders {
				return fmt.Errorf("redis active count exceeded max: %d > %d", active, maxLeaders)
			}

			time.Sleep(10 * time.Millisecond)
			return nil
		})
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       members,
		RoundsPerTask: 4,
		Timeout:       10 * time.Second,
	})
	report := tester.RunT(t, tasks...)
	if report.Completed != members*4 {
		t.Fatalf("stress test should complete every campaign, got %+v", report)
	}
	if atomic.LoadInt32(&maxRunning) > maxLeaders {
		t.Fatalf("max running leaders exceeded max: %d > %d", maxRunning, maxLeaders)
	}
}

func TestRedisGroupElectorConcurrentCampaignOnSameInstanceIsSingleOwner(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)

	elector := newGroupElector(t, client, "same-instance-concurrent-campaign", "member-1", 5)
	const goroutines = 16
	start := make(chan struct{})
	errs := make(chan error, goroutines)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- elector.Campaign(ctx)
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	var successes int
	var alreadyLeader int
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, leader.ErrAlreadyLeader):
			alreadyLeader++
		default:
			t.Fatalf("unexpected campaign error: %v", err)
		}
	}

	if successes != 1 {
		t.Fatalf("exactly one campaign should succeed, successes=%d alreadyLeader=%d", successes, alreadyLeader)
	}
	if err := elector.Resign(ctx); err != nil {
		t.Fatalf("resign: %v", err)
	}
	active, err := elector.ActiveCount(ctx)
	if err != nil {
		t.Fatalf("active count: %v", err)
	}
	if active != 0 {
		t.Fatalf("active count should be 0 after resign, got %d", active)
	}
}

func newGroupElector(
	t *testing.T,
	client redis.Cmdable,
	group string,
	memberID string,
	maxLeaders int,
) *redisleader.GroupElector {
	t.Helper()

	elector, err := redisleader.NewGroup(client, leader.GroupOptions{
		Options: leader.Options{
			Group:         group,
			MemberID:      memberID,
			Lease:         2 * time.Second,
			RenewInterval: 500 * time.Millisecond,
		},
		MaxLeaders: maxLeaders,
	})
	if err != nil {
		t.Fatalf("new group elector: %v", err)
	}
	return elector
}
