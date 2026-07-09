package mongoleader

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	mongodbtestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/mongodb"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestMongoStrategicElectorIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	client := newMongoClient(ctx, t)

	t.Run("register list and unregister candidates", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		elector := newTestStrategicElector[string](t, collection, "strategic-register-list", "node-a")

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
	})

	t.Run("prunes expired candidates before ttl cleanup", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		now := time.Date(2026, 7, 10, 1, 0, 0, 0, time.UTC)
		clockValue := atomic.Value{}
		clockValue.Store(now)
		clock := func() time.Time {
			return clockValue.Load().(time.Time)
		}
		elector, err := NewStrategic[string](collection, leader.Options{
			Group:    "strategic-expiry",
			MemberID: "node-a",
		}, WithClock(clock))
		if err != nil {
			t.Fatalf("new strategic elector: %v", err)
		}

		if err := elector.RegisterCandidate(ctx, "nightly", leader.CandidateInfo{NodeID: "node-a"}, 50*time.Millisecond); err != nil {
			t.Fatalf("register candidate: %v", err)
		}
		clockValue.Store(now.Add(time.Second))

		candidates, err := elector.ListCandidates(ctx, "nightly")
		if err != nil {
			t.Fatalf("list after expiry: %v", err)
		}
		if len(candidates) != 0 {
			t.Fatalf("candidate count after expiry = %d, want 0", len(candidates))
		}
		remaining, err := collection.CountDocuments(ctx, map[string]any{"group_key": elector.groupKey("nightly")})
		if err != nil {
			t.Fatalf("count pruned candidates: %v", err)
		}
		if remaining != 0 {
			t.Fatalf("expired candidate document should be pruned, remaining=%d", remaining)
		}
	})

	t.Run("update result is atomic", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		elector := newTestStrategicElector[string](t, collection, "strategic-atomic-update", "node-a")
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
	})

	t.Run("update missing candidate returns not leader", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		elector := newTestStrategicElector[string](t, collection, "strategic-missing-update", "node-a")

		err := elector.UpdateResult(ctx, "nightly", "node-a", leader.CandidateSucceeded)
		if !errors.Is(err, leader.ErrNotLeader) {
			t.Fatalf("missing update should return ErrNotLeader, got %v", err)
		}
	})

	t.Run("run if leader executes winner only", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		first := newTestStrategicElector[string](t, collection, "strategic-run-winner", "node-a")
		second := newTestStrategicElector[string](t, collection, "strategic-run-winner", "node-b")
		registered := time.Date(2026, 7, 10, 1, 0, 0, 0, time.UTC)

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
	})

	t.Run("supports random and scored strategies", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		first := newTestStrategicElector[string](t, collection, "strategic-strategies", "node-a")
		second := newTestStrategicElector[string](t, collection, "strategic-strategies", "node-b")
		registered := time.Date(2026, 7, 10, 1, 0, 0, 0, time.UTC)

		if err := first.RegisterCandidate(ctx, "nightly", leader.CandidateInfo{
			NodeID: "node-a", RegisteredAt: registered, Weight: 1,
		}, time.Second); err != nil {
			t.Fatalf("register first: %v", err)
		}
		if err := second.RegisterCandidate(ctx, "nightly", leader.CandidateInfo{
			NodeID: "node-b", RegisteredAt: registered.Add(time.Second), Weight: 10,
		}, time.Second); err != nil {
			t.Fatalf("register second: %v", err)
		}

		_, ran, err := second.RunIfLeader(ctx, "nightly", leader.ScoredStrategy{Scorer: leader.WeightScorer{}}, func(context.Context) (string, error) {
			return "scored", nil
		})
		if err != nil {
			t.Fatalf("run scored winner: %v", err)
		}
		if !ran {
			t.Fatal("weighted candidate should run")
		}
		candidates, err := first.ListCandidates(ctx, "nightly")
		if err != nil {
			t.Fatalf("list for random: %v", err)
		}
		if _, ok := (leader.RandomStrategy{Seed: 42}).Elect(candidates); !ok {
			t.Fatal("random strategy should elect from MongoDB candidates")
		}
	})

	t.Run("run if leader records failure", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		elector := newTestStrategicElector[string](t, collection, "strategic-run-failure", "node-a")
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
	})

	t.Run("async cancellation", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		elector := newTestStrategicElector[string](t, collection, "strategic-async-cancel", "node-a")
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
	})

	t.Run("stress registers and elects one winner", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		group := "nightly"
		electors := []*StrategicElector[string]{
			newTestStrategicElector[string](t, collection, "strategic-stress", "node-a"),
			newTestStrategicElector[string](t, collection, "strategic-stress", "node-b"),
			newTestStrategicElector[string](t, collection, "strategic-stress", "node-c"),
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
			registered := time.Date(2026, 7, 10, 1, 0, index, 0, time.UTC)
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
	})
}

func TestNewStrategicRejectsInvalidInputs(t *testing.T) {
	if _, err := NewStrategic[string](nil, leader.Options{Group: "g", MemberID: "m"}); err == nil {
		t.Fatal("expected nil collection error")
	}
	if _, err := NewStrategic[string](&mongo.Collection{}, leader.Options{Group: "g", MemberID: "m"}, nil); err == nil {
		t.Fatal("expected nil option error")
	}
	if _, err := NewStrategic[string](&mongo.Collection{}, leader.Options{Group: "g", MemberID: "m"}, WithRetryDelay(0)); err == nil {
		t.Fatal("expected invalid retry delay error")
	}
}

func newTestStrategicElector[T any](
	t *testing.T,
	collection *mongo.Collection,
	group string,
	memberID string,
) *StrategicElector[T] {
	t.Helper()
	elector, err := NewStrategic[T](collection, leader.Options{
		Group:    group,
		MemberID: memberID,
	})
	if err != nil {
		t.Fatalf("new strategic elector: %v", err)
	}
	return elector
}

func newMongoClient(ctx context.Context, t *testing.T) *mongo.Client {
	t.Helper()

	uri := mongodbtestcontainer.Start(ctx, t)
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect mongodb: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cleanupCancel()
		if err := client.Disconnect(cleanupCtx); err != nil {
			t.Fatalf("disconnect mongodb client: %v", err)
		}
	})
	return client
}
