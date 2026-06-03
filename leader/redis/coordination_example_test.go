package redisleader_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	"github.com/redis/go-redis/v9"
)

// TestBatchSchedulerExample 는 여러 replica 중 하나만 예약 작업을 실행하는 문제를 다룬다.
func TestBatchSchedulerExample(t *testing.T) {
	ctx := context.Background()
	client := redisExampleClient(ctx, t)

	const group = "example-batch-scheduler"
	primary := redisExampleElector(t, client, group, "scheduler-a")
	secondary := redisExampleElector(t, client, group, "scheduler-b")

	if err := primary.Campaign(ctx); err != nil {
		t.Fatalf("primary campaign: %v", err)
	}
	t.Cleanup(func() {
		_ = primary.Resign(context.Background())
	})

	if err := secondary.Campaign(ctx); !errors.Is(err, leader.ErrNotLeader) {
		t.Fatalf("secondary campaign should fail while primary is leader, got %v", err)
	}

	if primary.IsLeader() {
		if err := client.Incr(ctx, "example:batch:nightly-settlement").Err(); err != nil {
			t.Fatalf("run primary scheduled job: %v", err)
		}
	}
	if secondary.IsLeader() {
		if err := client.Incr(ctx, "example:batch:nightly-settlement").Err(); err != nil {
			t.Fatalf("run secondary scheduled job: %v", err)
		}
	}

	count, err := client.Get(ctx, "example:batch:nightly-settlement").Int()
	if err != nil {
		t.Fatalf("read scheduled job count: %v", err)
	}
	if count != 1 {
		t.Fatalf("scheduled job should run once while one leader exists, got %d", count)
	}

	if err := primary.Resign(ctx); err != nil {
		t.Fatalf("primary resign: %v", err)
	}
	if err := secondary.Campaign(ctx); err != nil {
		t.Fatalf("secondary campaign after resign: %v", err)
	}
	t.Cleanup(func() {
		_ = secondary.Resign(context.Background())
	})
}

// TestGroupBatchWorkersExample 는 동시에 실행할 수 있는 batch worker 수를 제한한다.
func TestGroupBatchWorkersExample(t *testing.T) {
	ctx := context.Background()
	client := redisExampleClient(ctx, t)

	const group = "example-group-batch-workers"
	workers := []*redisleader.GroupElector{
		redisExampleGroupElector(t, client, group, "worker-a", 2),
		redisExampleGroupElector(t, client, group, "worker-b", 2),
		redisExampleGroupElector(t, client, group, "worker-c", 2),
	}

	for i := 0; i < 2; i++ {
		worker := workers[i]
		if err := worker.Campaign(ctx); err != nil {
			t.Fatalf("worker %d campaign: %v", i, err)
		}
		t.Cleanup(func() {
			_ = worker.Resign(context.Background())
		})
		if err := client.Incr(ctx, "example:group-batch:running").Err(); err != nil {
			t.Fatalf("count running worker: %v", err)
		}
	}

	waitCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	if err := workers[2].Campaign(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("third worker should wait until context deadline, got %v", err)
	}

	count, err := client.Get(ctx, "example:group-batch:running").Int()
	if err != nil {
		t.Fatalf("read running worker count: %v", err)
	}
	if count != 2 {
		t.Fatalf("only two workers should run concurrently, got %d", count)
	}
}

// TestMigrationGateExample 는 배포 중 여러 instance가 같은 migration을 중복 적용하지 않게 한다.
func TestMigrationGateExample(t *testing.T) {
	ctx := context.Background()
	client := redisExampleClient(ctx, t)

	const (
		group        = "example-migration-gate"
		migrationKey = "example:migration:v20260603"
	)
	first := redisExampleElector(t, client, group, "api-a")
	second := redisExampleElector(t, client, group, "api-b")

	if err := first.Campaign(ctx); err != nil {
		t.Fatalf("first campaign: %v", err)
	}
	t.Cleanup(func() {
		_ = first.Resign(context.Background())
	})

	applied, err := applyMigrationOnce(ctx, client, migrationKey, first)
	if err != nil {
		t.Fatalf("first migration: %v", err)
	}
	if !applied {
		t.Fatal("first leader should apply migration")
	}

	if err := first.Resign(ctx); err != nil {
		t.Fatalf("first resign: %v", err)
	}
	if err := second.Campaign(ctx); err != nil {
		t.Fatalf("second campaign: %v", err)
	}
	t.Cleanup(func() {
		_ = second.Resign(context.Background())
	})

	applied, err = applyMigrationOnce(ctx, client, migrationKey, second)
	if err != nil {
		t.Fatalf("second migration: %v", err)
	}
	if applied {
		t.Fatal("second leader should skip already applied migration")
	}
}

func redisExampleClient(ctx context.Context, t *testing.T) *redis.Client {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client
}

func redisExampleElector(t *testing.T, client redis.Cmdable, group string, memberID string) *redisleader.Elector {
	t.Helper()

	elector, err := redisleader.New(client, leader.Options{
		Group:         group,
		MemberID:      memberID,
		Lease:         2 * time.Second,
		RenewInterval: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new elector: %v", err)
	}
	return elector
}

func redisExampleGroupElector(
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

func applyMigrationOnce(ctx context.Context, client redis.Cmdable, key string, elector leader.Elector) (bool, error) {
	if !elector.IsLeader() {
		return false, leader.ErrNotLeader
	}
	return client.SetNX(ctx, key, "applied", 0).Result()
}
