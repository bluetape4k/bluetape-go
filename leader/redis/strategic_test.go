package redisleader_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	bttesting "github.com/bluetape4k/bluetape-go/testing"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/redis/go-redis/v9"
)

func TestRedisStrategicElectorRegistersListsAndUnregistersCandidates(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)
	elector := newStrategicElector[string](t, client, "strategic-register-list", "node-a")

	if err := elector.RegisterCandidate(ctx, "nightly", leader.CandidateInfo{
		NodeID:   "node-a",
		Weight:   3,
		Metadata: map[string]string{"zone": "a"},
	}, time.Second); err != nil {
		t.Fatalf("register candidate: %v", err)
	}

	candidates, err := elector.ListCandidates(ctx, "nightly")
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidate count = %d, want 1", len(candidates))
	}
	if candidates[0].NodeID != "node-a" || candidates[0].Weight != 3 {
		t.Fatalf("unexpected candidate: %+v", candidates[0])
	}
	if candidates[0].RegisteredAt.IsZero() {
		t.Fatal("registeredAt should be defaulted")
	}
	if candidates[0].Metadata["zone"] != "a" {
		t.Fatalf("metadata not preserved: %+v", candidates[0].Metadata)
	}

	if err := elector.UnregisterCandidate(ctx, "nightly", "node-a"); err != nil {
		t.Fatalf("unregister candidate: %v", err)
	}
	candidates, err = elector.ListCandidates(ctx, "nightly")
	if err != nil {
		t.Fatalf("list after unregister: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidate count after unregister = %d, want 0", len(candidates))
	}
}

func TestRedisStrategicElectorPrunesExpiredCandidates(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)
	elector := newStrategicElector[string](t, client, "strategic-expiry", "node-a")

	if err := elector.RegisterCandidate(ctx, "nightly", leader.CandidateInfo{NodeID: "node-a"}, 50*time.Millisecond); err != nil {
		t.Fatalf("register candidate: %v", err)
	}

	bttesting.Eventually(t, time.Second, func() bool {
		candidates, err := elector.ListCandidates(ctx, "nightly")
		return err == nil && len(candidates) == 0
	})
}

func TestRedisStrategicElectorUpdateResult(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)
	elector := newStrategicElector[string](t, client, "strategic-update-result", "node-a")

	if err := elector.RegisterCandidate(ctx, "nightly", leader.CandidateInfo{NodeID: "node-a"}, time.Second); err != nil {
		t.Fatalf("register candidate: %v", err)
	}
	if err := elector.UpdateResult(ctx, "nightly", "node-a", leader.CandidateSucceeded); err != nil {
		t.Fatalf("update success: %v", err)
	}
	if err := elector.UpdateResult(ctx, "nightly", "node-a", leader.CandidateFailed); err != nil {
		t.Fatalf("update failure: %v", err)
	}

	candidates, err := elector.ListCandidates(ctx, "nightly")
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidate count = %d, want 1", len(candidates))
	}
	candidate := candidates[0]
	if candidate.SuccessCount != 1 || candidate.FailureCount != 1 {
		t.Fatalf("result counts = success:%d failure:%d, want 1/1", candidate.SuccessCount, candidate.FailureCount)
	}
	if candidate.LastCompletedAt.IsZero() {
		t.Fatal("last completed time should be set")
	}
}

func TestRedisStrategicElectorUpdateResultIsAtomic(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)
	elector := newStrategicElector[string](t, client, "strategic-atomic-update", "node-a")

	if err := elector.RegisterCandidate(ctx, "nightly", leader.CandidateInfo{NodeID: "node-a"}, time.Second); err != nil {
		t.Fatalf("register candidate: %v", err)
	}

	const updates = 64
	var wg sync.WaitGroup
	errs := make(chan error, updates)
	for range updates {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- elector.UpdateResult(ctx, "nightly", "node-a", leader.CandidateSucceeded)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("update result: %v", err)
		}
	}

	candidates, err := elector.ListCandidates(ctx, "nightly")
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidate count = %d, want 1", len(candidates))
	}
	if candidates[0].SuccessCount != updates {
		t.Fatalf("success count = %d, want %d", candidates[0].SuccessCount, updates)
	}
}

func TestRedisStrategicElectorUpdateMissingCandidateReturnsNotLeader(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)
	elector := newStrategicElector[string](t, client, "strategic-missing-update", "node-a")

	err := elector.UpdateResult(ctx, "nightly", "node-a", leader.CandidateSucceeded)
	if !errors.Is(err, leader.ErrNotLeader) {
		t.Fatalf("missing update should return ErrNotLeader, got %v", err)
	}
}

func TestRedisStrategicElectorRunIfLeaderExecutesWinnerOnly(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)
	first := newStrategicElector[string](t, client, "strategic-run-winner", "node-a")
	second := newStrategicElector[string](t, client, "strategic-run-winner", "node-b")
	registered := time.Date(2026, 6, 5, 1, 0, 0, 0, time.UTC)

	if err := first.RegisterCandidate(ctx, "nightly", leader.CandidateInfo{
		NodeID: "node-a", RegisteredAt: registered,
	}, time.Second); err != nil {
		t.Fatalf("register first: %v", err)
	}
	if err := second.RegisterCandidate(ctx, "nightly", leader.CandidateInfo{
		NodeID: "node-b", RegisteredAt: registered.Add(time.Second),
	}, time.Second); err != nil {
		t.Fatalf("register second: %v", err)
	}

	result, ran, err := first.RunIfLeader(ctx, "nightly", leader.FifoStrategy{}, func(context.Context) (string, error) {
		return "ran", nil
	})
	if err != nil {
		t.Fatalf("run winner: %v", err)
	}
	if !ran || result != "ran" {
		t.Fatalf("winner run = (%q, %v), want ran/true", result, ran)
	}

	result, ran, err = second.RunIfLeader(ctx, "nightly", leader.FifoStrategy{}, func(context.Context) (string, error) {
		return "should-not-run", nil
	})
	if err != nil {
		t.Fatalf("run non-winner: %v", err)
	}
	if ran || result != "" {
		t.Fatalf("non-winner run = (%q, %v), want zero/false", result, ran)
	}

	candidates, err := first.ListCandidates(ctx, "nightly")
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	for _, candidate := range candidates {
		if candidate.NodeID == "node-a" && candidate.SuccessCount != 1 {
			t.Fatalf("winner success count = %d, want 1", candidate.SuccessCount)
		}
		if candidate.NodeID == "node-b" && candidate.TotalCount() != 0 {
			t.Fatalf("non-winner total count = %d, want 0", candidate.TotalCount())
		}
	}
}

func TestRedisStrategicElectorRunIfLeaderRecordsFailure(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)
	elector := newStrategicElector[string](t, client, "strategic-run-failure", "node-a")

	if err := elector.RegisterCandidate(ctx, "nightly", leader.CandidateInfo{NodeID: "node-a"}, time.Second); err != nil {
		t.Fatalf("register candidate: %v", err)
	}
	actionErr := errors.New("action failed")
	_, ran, err := elector.RunIfLeader(ctx, "nightly", leader.FifoStrategy{}, func(context.Context) (string, error) {
		return "", actionErr
	})
	if !ran {
		t.Fatal("winner action should run")
	}
	if !errors.Is(err, actionErr) {
		t.Fatalf("run should preserve action error, got %v", err)
	}

	candidates, err := elector.ListCandidates(ctx, "nightly")
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	if candidates[0].FailureCount != 1 {
		t.Fatalf("failure count = %d, want 1", candidates[0].FailureCount)
	}
}

func TestRedisStrategicElectorAsyncCancellation(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)
	elector := newStrategicElector[string](t, client, "strategic-async-cancel", "node-a")
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       2,
		RoundsPerTask: 8,
		Timeout:       2 * time.Second,
	})

	tester.RunT(t, func(ctx context.Context) error {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		err := elector.RegisterCandidate(cancelled, "nightly", leader.CandidateInfo{NodeID: "node-a"}, time.Second)
		if !errors.Is(err, context.Canceled) {
			return fmt.Errorf("register should preserve context cancellation, got %w", err)
		}
		return nil
	})
}

func TestRedisStrategicElectorStressRegistersAndElectsOneWinner(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(ctx, t)
	group := "nightly"
	electors := []*redisleader.StrategicElector[string]{
		newStrategicElector[string](t, client, "strategic-stress", "node-a"),
		newStrategicElector[string](t, client, "strategic-stress", "node-b"),
		newStrategicElector[string](t, client, "strategic-stress", "node-c"),
	}
	var actionRuns int64
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       6,
		RoundsPerTask: 12,
		Timeout:       5 * time.Second,
	})

	tasks := make([]concurrencytest.Task, 0, len(electors))
	for index, elector := range electors {
		nodeID := fmt.Sprintf("node-%c", 'a'+rune(index))
		registered := time.Date(2026, 6, 5, 1, 0, index, 0, time.UTC)
		current := elector
		tasks = append(tasks, func(ctx context.Context) error {
			if err := current.RegisterCandidate(ctx, group, leader.CandidateInfo{
				NodeID:       nodeID,
				RegisteredAt: registered,
			}, 2*time.Second); err != nil {
				return err
			}
			_, _, err := current.RunIfLeader(ctx, group, leader.FifoStrategy{}, func(context.Context) (string, error) {
				atomic.AddInt64(&actionRuns, 1)
				return "ok", nil
			})
			return err
		})
	}

	tester.RunT(t, tasks...)
	if actionRuns == 0 {
		t.Fatal("stress run should execute the elected action at least once")
	}

	candidates, err := electors[0].ListCandidates(ctx, group)
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("candidate count = %d, want 3", len(candidates))
	}
	if candidates[0].NodeID != "node-a" || candidates[0].SuccessCount == 0 {
		t.Fatalf("fifo winner should be node-a with successes, got %+v", candidates[0])
	}
}

func newRedisClient(ctx context.Context, t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: redistestcontainer.Start(ctx, t)})
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client
}

func newStrategicElector[T any](
	t *testing.T,
	client redis.Cmdable,
	group string,
	memberID string,
) *redisleader.StrategicElector[T] {
	t.Helper()
	elector, err := redisleader.NewStrategic[T](client, leader.Options{
		Group:    group,
		MemberID: memberID,
	})
	if err != nil {
		t.Fatalf("new strategic elector: %v", err)
	}
	return elector
}
