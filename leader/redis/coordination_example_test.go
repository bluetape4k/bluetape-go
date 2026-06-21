package redisleader_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/batch"
	"github.com/bluetape4k/bluetape-go/leader"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/redis/go-redis/v9"
)

// TestBatchSchedulerExample 는 여러 replica 중 하나만 예약 작업을 실행하는 문제를 다룬다.
func TestBatchSchedulerExample(t *testing.T) {
	ctx := context.Background()
	client := redisExampleClient(ctx, t)

	const (
		group     = "example-batch-scheduler"
		resultKey = "example:batch:nightly-settlement"
	)
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

	job, err := redisListBatchJob(
		"nightly-settlement",
		resultKey,
		client,
		[]string{"invoice-1", "invoice-2", "invoice-3"},
	)
	if err != nil {
		t.Fatalf("create scheduler batch job: %v", err)
	}
	report, ran, err := runBatchIfLeader(ctx, primary, job)
	if err != nil {
		t.Fatalf("run primary scheduled job: %v", err)
	}
	if !ran || !report.IsSuccess() || report.WriteCount != 3 {
		t.Fatalf("unexpected primary batch report ran=%v report=%+v", ran, report)
	}

	_, ran, err = runBatchIfLeader(ctx, secondary, job)
	if !errors.Is(err, leader.ErrNotLeader) {
		t.Fatalf("secondary should not run scheduled batch, ran=%v err=%v", ran, err)
	}
	if ran {
		t.Fatal("secondary should not run while it is not leader")
	}

	items, err := client.LRange(ctx, resultKey, 0, -1).Result()
	if err != nil {
		t.Fatalf("read scheduled job output: %v", err)
	}
	if got, want := len(items), 3; got != want {
		t.Fatalf("scheduled job should write once from the leader, got %d items want %d: %v", got, want, items)
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

	job, err := migrationBatchJob("schema-v20260603", migrationKey, client)
	if err != nil {
		t.Fatalf("create migration batch job: %v", err)
	}
	report, ran, err := runBatchIfLeader(ctx, first, job)
	if err != nil {
		t.Fatalf("first migration: %v", err)
	}
	if !ran || !report.IsSuccess() || report.WriteCount != 1 {
		t.Fatalf("first leader should apply migration once, ran=%v report=%+v", ran, report)
	}

	_, ran, err = runBatchIfLeader(ctx, second, job)
	if !errors.Is(err, leader.ErrNotLeader) {
		t.Fatalf("second elector should not run migration while first is leader, ran=%v err=%v", ran, err)
	}
	if ran {
		t.Fatal("second elector should not run before it owns leadership")
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

	job, err = migrationBatchJob("schema-v20260603", migrationKey, client)
	if err != nil {
		t.Fatalf("create second migration batch job: %v", err)
	}
	report, ran, err = runBatchIfLeader(ctx, second, job)
	if err != nil {
		t.Fatalf("second migration: %v", err)
	}
	if !ran || !report.IsSuccess() || report.SkipCount != 1 || report.WriteCount != 0 {
		t.Fatalf("second leader should skip already applied migration, ran=%v report=%+v", ran, report)
	}
	value, err := client.Get(ctx, migrationKey).Result()
	if err != nil {
		t.Fatalf("read migration marker: %v", err)
	}
	if value != "schema-v20260603" {
		t.Fatalf("unexpected migration marker %q", value)
	}
}

func TestLeaderGuardedBatchExecutionWithGoroutineStressTester(t *testing.T) {
	ctx := context.Background()
	client := redisExampleClient(ctx, t)

	const (
		group     = "example-batch-stress"
		resultKey = "example:batch:stress-output"
	)
	elector := redisExampleElector(t, client, group, "scheduler-a")
	if err := elector.Campaign(ctx); err != nil {
		t.Fatalf("campaign: %v", err)
	}
	t.Cleanup(func() {
		_ = elector.Resign(context.Background())
	})

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 12,
		Timeout:       2 * time.Second,
	})
	var sequence atomic.Int64
	stressReport, err := tester.Run(ctx, func(ctx context.Context) error {
		item := "job-" + strconv.FormatInt(sequence.Add(1), 10)
		job, err := redisListBatchJob("stress-batch", resultKey, client, []string{item})
		if err != nil {
			return err
		}
		report, ran, err := runBatchIfLeader(ctx, elector, job)
		if err != nil {
			return err
		}
		if !ran || !report.IsSuccess() || report.WriteCount != 1 {
			return fmt.Errorf("unexpected leader batch result ran=%v report=%+v", ran, report)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stress run failed: report=%+v err=%v", stressReport, err)
	}
	if stressReport.Completed != 12 {
		t.Fatalf("expected 12 completed stress jobs, got %+v", stressReport)
	}
	count, err := client.LLen(ctx, resultKey).Result()
	if err != nil {
		t.Fatalf("read stress output length: %v", err)
	}
	if count != 12 {
		t.Fatalf("expected 12 leader-guarded batch writes, got %d", count)
	}
}

func TestLeaderGuardedBatchExecutionWithAsyncJobTesterCancellation(t *testing.T) {
	ctx := context.Background()
	client := redisExampleClient(ctx, t)

	elector := redisExampleElector(t, client, "example-batch-async-cancel", "scheduler-a")
	if err := elector.Campaign(ctx); err != nil {
		t.Fatalf("campaign: %v", err)
	}
	t.Cleanup(func() {
		_ = elector.Resign(context.Background())
	})

	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers: 1,
		Timeout: 25 * time.Millisecond,
	})
	asyncReport, err := tester.Run(ctx, func(ctx context.Context) error {
		job, err := blockingBatchJob()
		if err != nil {
			return err
		}
		report, ran, err := runBatchIfLeader(ctx, elector, job)
		if !ran {
			return leader.ErrNotLeader
		}
		if report.Status != batch.StatusCancelled {
			return fmt.Errorf("expected cancelled batch report, got %+v", report)
		}
		return err
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got report=%+v err=%v", asyncReport, err)
	}
	if asyncReport.Failures != 1 {
		t.Fatalf("expected one async cancellation failure, got %+v", asyncReport)
	}
}

func redisExampleClient(ctx context.Context, t *testing.T) *redis.Client {
	t.Helper()

	client := newRedisClient(ctx, t)
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

func runBatchIfLeader(ctx context.Context, elector leader.Elector, runner batch.Runner) (batch.Report, bool, error) {
	if err := ctx.Err(); err != nil {
		return batch.Report{}, false, err
	}
	if !elector.IsLeader() {
		return batch.Report{}, false, leader.ErrNotLeader
	}
	report := runner.Run(ctx)
	return report, true, report.Err
}

func redisListBatchJob(name string, key string, client redis.Cmdable, items []string) (*batch.Job, error) {
	step, err := batch.NewStep(batch.StepOptions[string, string]{
		Name:      name + "-step",
		ChunkSize: 2,
		Reader:    newExampleSliceReader(items),
		Processor: batch.IdentityProcessor[string](),
		Writer:    redisListWriter{client: client, key: key},
	})
	if err != nil {
		return nil, err
	}
	return batch.NewJob(name, step)
}

func migrationBatchJob(name string, markerKey string, client redis.Cmdable) (*batch.Job, error) {
	skip, err := batch.SkipErrors(1, func(err error) bool {
		return errors.Is(err, errMigrationAlreadyApplied)
	})
	if err != nil {
		return nil, err
	}
	step, err := batch.NewStep(batch.StepOptions[string, string]{
		Name:       name + "-step",
		ChunkSize:  1,
		Reader:     newExampleSliceReader([]string{name}),
		Processor:  batch.IdentityProcessor[string](),
		Writer:     redisMigrationWriter{client: client, key: markerKey},
		SkipPolicy: skip,
	})
	if err != nil {
		return nil, err
	}
	return batch.NewJob(name, step)
}

func blockingBatchJob() (*batch.Job, error) {
	step, err := batch.NewStep(batch.StepOptions[int, int]{
		Name:      "blocking-step",
		ChunkSize: 1,
		Reader:    blockingBatchReader{},
		Processor: batch.IdentityProcessor[int](),
		Writer:    discardBatchWriter[int]{},
	})
	if err != nil {
		return nil, err
	}
	return batch.NewJob("blocking-batch", step)
}

type exampleSliceReader[T any] struct {
	values []T
	index  int
	mu     sync.Mutex
}

func newExampleSliceReader[T any](values []T) *exampleSliceReader[T] {
	return &exampleSliceReader[T]{values: append([]T(nil), values...)}
}

func (r *exampleSliceReader[T]) Open(context.Context) error { return nil }

func (r *exampleSliceReader[T]) Read(ctx context.Context) (T, bool, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.index >= len(r.values) {
		return zero, false, nil
	}
	value := r.values[r.index]
	r.index++
	return value, true, nil
}

func (r *exampleSliceReader[T]) Close(context.Context) error { return nil }

type redisListWriter struct {
	client redis.Cmdable
	key    string
}

func (w redisListWriter) Open(context.Context) error { return nil }

func (w redisListWriter) Write(ctx context.Context, items []string) error {
	for _, item := range items {
		if err := w.client.RPush(ctx, w.key, item).Err(); err != nil {
			return fmt.Errorf("write batch item: %w", err)
		}
	}
	return nil
}

func (w redisListWriter) Close(context.Context) error { return nil }

var errMigrationAlreadyApplied = errors.New("migration already applied")

type redisMigrationWriter struct {
	client redis.Cmdable
	key    string
}

func (w redisMigrationWriter) Open(context.Context) error { return nil }

func (w redisMigrationWriter) Write(ctx context.Context, items []string) error {
	for _, item := range items {
		ok, err := w.client.SetNX(ctx, w.key, item, 0).Result()
		if err != nil {
			return fmt.Errorf("write migration marker: %w", err)
		}
		if !ok {
			return errMigrationAlreadyApplied
		}
	}
	return nil
}

func (w redisMigrationWriter) Close(context.Context) error { return nil }

type blockingBatchReader struct{}

func (blockingBatchReader) Open(context.Context) error { return nil }

func (blockingBatchReader) Read(ctx context.Context) (int, bool, error) {
	<-ctx.Done()
	return 0, false, ctx.Err()
}

func (blockingBatchReader) Close(context.Context) error { return nil }

type discardBatchWriter[T any] struct{}

func (discardBatchWriter[T]) Open(context.Context) error { return nil }

func (discardBatchWriter[T]) Write(ctx context.Context, _ []T) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (discardBatchWriter[T]) Close(context.Context) error { return nil }
