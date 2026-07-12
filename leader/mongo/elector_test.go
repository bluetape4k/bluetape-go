package mongoleader

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	"github.com/bluetape4k/bluetape-go/leader/leadertest"
	mongodbtestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/mongodb"
	bttesting "github.com/bluetape4k/bluetape-go/testing"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestElectorIgnoresStaleRenewalWorkerState(t *testing.T) {
	oldDone := make(chan struct{})
	currentDone := make(chan struct{})
	elector := &Elector{owned: true, generation: 2, done: currentDone}

	elector.clearOwnershipAfterLoss(1, oldDone, true)

	if !elector.owned || elector.cleanup || elector.done != currentDone {
		t.Fatal("stale renewal worker changed current ownership state")
	}
}

func TestElectorInactiveCleanupDoesNotChangeState(t *testing.T) {
	elector := &Elector{}
	_, _, _, active := elector.clearOwnership()
	if active || elector.cleanup {
		t.Fatal("inactive cleanup changed elector state")
	}
}

func TestMongoElectorIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

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

	t.Run("conformance", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		control := newMongoConformanceControl(collection)
		leadertest.Run(t, leadertest.Harness{
			New: func(_ testing.TB, opts leader.Options) (leader.Elector, error) {
				elector, err := New(collection, opts, WithRetryDelay(10*time.Millisecond))
				if err != nil {
					return nil, err
				}
				elector.testHook = func(operation string) error {
					return control.after(opts, operation)
				}
				return elector, nil
			},
			Control: control,
		})
	})

	t.Run("campaign and resign", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		elector := newTestElector(t, collection, "campaign-and-resign", "member-1")

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
	})

	t.Run("repeated resign permits another campaign", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		elector := newTestElector(t, collection, "repeated-resign", "member-1")
		if err := elector.Campaign(ctx); err != nil {
			t.Fatalf("campaign: %v", err)
		}
		if err := elector.Resign(ctx); err != nil {
			t.Fatalf("first resign: %v", err)
		}
		if err := elector.Resign(ctx); err != nil {
			t.Fatalf("second resign: %v", err)
		}
		if err := elector.Campaign(ctx); err != nil {
			t.Fatalf("campaign after repeated resign: %v", err)
		}
		if err := elector.Resign(ctx); err != nil {
			t.Fatalf("final resign: %v", err)
		}
	})

	t.Run("duplicate local campaign", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		elector := newTestElector(t, collection, "duplicate-local-campaign", "member-1")

		if err := elector.Campaign(ctx); err != nil {
			t.Fatalf("campaign: %v", err)
		}
		t.Cleanup(func() {
			_ = elector.Resign(context.Background())
		})
		if err := elector.Campaign(ctx); !errors.Is(err, leader.ErrAlreadyLeader) {
			t.Fatalf("duplicate campaign error = %v, want ErrAlreadyLeader", err)
		}
	})

	t.Run("waits until current owner resigns", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		first := newTestElector(t, collection, "waits-until-resign", "member-1")
		second := newTestElector(t, collection, "waits-until-resign", "member-2")
		if err := first.Campaign(ctx); err != nil {
			t.Fatalf("first campaign: %v", err)
		}

		acquired := make(chan error, 1)
		go func() {
			waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer waitCancel()
			acquired <- second.Campaign(waitCtx)
		}()

		bttesting.Consistently(t, 100*time.Millisecond, func() bool {
			return !second.IsLeader()
		})
		if err := first.Resign(ctx); err != nil {
			t.Fatalf("first resign: %v", err)
		}
		if err := <-acquired; err != nil {
			t.Fatalf("second campaign after resign: %v", err)
		}
		t.Cleanup(func() {
			_ = second.Resign(context.Background())
		})
		if !second.IsLeader() {
			t.Fatal("second elector should own leadership after first resigns")
		}
	})

	t.Run("contention allows one active leader", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		var wins int32
		var deadlineLosses int32
		tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
			Workers:       8,
			RoundsPerTask: 1,
			Timeout:       2 * time.Second,
		})

		tasks := make([]concurrencytest.Task, 8)
		for i := range tasks {
			memberID := fmt.Sprintf("member-%d", i)
			tasks[i] = func(context.Context) error {
				elector := newTestElector(t, collection, "contention-one-leader", memberID)
				campaignCtx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
				defer cancel()
				err := elector.Campaign(campaignCtx)
				if err == nil {
					atomic.AddInt32(&wins, 1)
					t.Cleanup(func() {
						_ = elector.Resign(context.Background())
					})
					return nil
				}
				if errors.Is(err, context.DeadlineExceeded) {
					atomic.AddInt32(&deadlineLosses, 1)
					return nil
				}
				return err
			}
		}
		report := tester.RunT(t, tasks...)
		if report.Completed != 8 {
			t.Fatalf("contention report = %+v", report)
		}
		if wins != 1 {
			t.Fatalf("wins = %d, want 1", wins)
		}
		if deadlineLosses != 7 {
			t.Fatalf("deadline losses = %d, want 7", deadlineLosses)
		}
	})

	t.Run("expired document can be taken over before ttl cleanup", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		now := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)
		clockValue := atomic.Value{}
		clockValue.Store(now)
		clock := func() time.Time {
			return clockValue.Load().(time.Time)
		}
		first, err := New(collection, leader.Options{
			Group:         "expired-takeover",
			MemberID:      "member-1",
			Lease:         50 * time.Millisecond,
			RenewInterval: 10 * time.Second,
		}, WithClock(clock))
		if err != nil {
			t.Fatalf("new first elector: %v", err)
		}
		second, err := New(collection, leader.Options{
			Group:         "expired-takeover",
			MemberID:      "member-2",
			Lease:         time.Second,
			RenewInterval: 10 * time.Second,
		}, WithClock(clock))
		if err != nil {
			t.Fatalf("new second elector: %v", err)
		}

		if err := first.Campaign(ctx); err != nil {
			t.Fatalf("first campaign: %v", err)
		}
		clockValue.Store(now.Add(time.Second))
		if err := second.Campaign(ctx); err != nil {
			t.Fatalf("second campaign after expiry: %v", err)
		}
		t.Cleanup(func() {
			_ = first.Resign(context.Background())
			_ = second.Resign(context.Background())
		})
		current, err := second.Leader(ctx)
		if err != nil {
			t.Fatalf("leader after takeover: %v", err)
		}
		if !strings.HasPrefix(current, "member-2:") {
			t.Fatalf("leader token = %q, want member-2", current)
		}
	})

	t.Run("renewal loss flips local state", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		elector, err := New(collection, leader.Options{
			Group:         "renewal-loss-token-change",
			MemberID:      "member-1",
			Lease:         time.Second,
			RenewInterval: 25 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("new elector: %v", err)
		}
		if err := elector.Campaign(ctx); err != nil {
			t.Fatalf("campaign: %v", err)
		}
		_, err = collection.UpdateOne(
			ctx,
			bson.M{"_id": elector.key},
			bson.M{"$set": bson.M{"token": "member-2:stolen", "lease_until": time.Now().Add(time.Second).UTC()}},
		)
		if err != nil {
			t.Fatalf("steal ownership document: %v", err)
		}

		bttesting.Eventually(t, time.Second, func() bool {
			return !elector.IsLeader()
		})
		if err := elector.Resign(ctx); err != nil {
			t.Fatalf("resign after lost leadership: %v", err)
		}
		current, err := elector.Leader(ctx)
		if err != nil {
			t.Fatalf("leader after lost leadership: %v", err)
		}
		if current != "member-2:stolen" {
			t.Fatalf("leader token = %q, want stolen token", current)
		}
	})

	t.Run("canceled campaign preserves context error", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		first := newTestElector(t, collection, "canceled-campaign", "member-1")
		second := newTestElector(t, collection, "canceled-campaign", "member-2")
		if err := first.Campaign(ctx); err != nil {
			t.Fatalf("first campaign: %v", err)
		}
		t.Cleanup(func() {
			_ = first.Resign(context.Background())
		})

		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := second.Campaign(cancelCtx); !errors.Is(err, context.Canceled) {
			t.Fatalf("campaign error = %v, want context canceled", err)
		}
	})

	t.Run("group allows max leaders and rejects extra contender", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		first := newTestGroupElector(t, collection, "group-allows-max", "member-1", 2)
		second := newTestGroupElector(t, collection, "group-allows-max", "member-2", 2)
		third := newTestGroupElector(t, collection, "group-allows-max", "member-3", 2)

		if err := first.Campaign(ctx); err != nil {
			t.Fatalf("first group campaign: %v", err)
		}
		t.Cleanup(func() {
			_ = first.Resign(context.Background())
		})
		if err := second.Campaign(ctx); err != nil {
			t.Fatalf("second group campaign: %v", err)
		}
		t.Cleanup(func() {
			_ = second.Resign(context.Background())
		})

		active, err := first.ActiveCount(ctx)
		if err != nil {
			t.Fatalf("group active count: %v", err)
		}
		if active != 2 {
			t.Fatalf("active count = %d, want 2", active)
		}
		available, err := first.AvailableSlots(ctx)
		if err != nil {
			t.Fatalf("group available slots: %v", err)
		}
		if available != 0 {
			t.Fatalf("available slots = %d, want 0", available)
		}

		waitCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		if err := third.Campaign(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("third campaign error = %v, want deadline exceeded", err)
		}
		if third.IsLeader() {
			t.Fatal("third elector should not own a slot")
		}
	})

	t.Run("group resign frees one slot", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		first := newTestGroupElector(t, collection, "group-resign-frees-slot", "member-1", 2)
		second := newTestGroupElector(t, collection, "group-resign-frees-slot", "member-2", 2)
		third := newTestGroupElector(t, collection, "group-resign-frees-slot", "member-3", 2)

		if err := first.Campaign(ctx); err != nil {
			t.Fatalf("first group campaign: %v", err)
		}
		if err := second.Campaign(ctx); err != nil {
			t.Fatalf("second group campaign: %v", err)
		}
		t.Cleanup(func() {
			_ = second.Resign(context.Background())
			_ = third.Resign(context.Background())
		})

		if err := first.Resign(ctx); err != nil {
			t.Fatalf("first group resign: %v", err)
		}
		if err := third.Campaign(ctx); err != nil {
			t.Fatalf("third campaign after resign: %v", err)
		}
		active, err := third.ActiveCount(ctx)
		if err != nil {
			t.Fatalf("group active count after resign: %v", err)
		}
		if active != 2 {
			t.Fatalf("active count after reclaim = %d, want 2", active)
		}
	})

	t.Run("group expired slot can be taken over before ttl cleanup", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		now := time.Date(2026, 7, 9, 1, 0, 0, 0, time.UTC)
		clockValue := atomic.Value{}
		clockValue.Store(now)
		clock := func() time.Time {
			return clockValue.Load().(time.Time)
		}
		first, err := NewGroup(collection, leader.GroupOptions{
			Options: leader.Options{
				Group:         "group-expired-takeover",
				MemberID:      "member-1",
				Lease:         50 * time.Millisecond,
				RenewInterval: 10 * time.Second,
			},
			MaxLeaders: 1,
		}, WithClock(clock))
		if err != nil {
			t.Fatalf("new first group elector: %v", err)
		}
		second, err := NewGroup(collection, leader.GroupOptions{
			Options: leader.Options{
				Group:         "group-expired-takeover",
				MemberID:      "member-2",
				Lease:         time.Second,
				RenewInterval: 10 * time.Second,
			},
			MaxLeaders: 1,
		}, WithClock(clock))
		if err != nil {
			t.Fatalf("new second group elector: %v", err)
		}

		if err := first.Campaign(ctx); err != nil {
			t.Fatalf("first group campaign: %v", err)
		}
		clockValue.Store(now.Add(time.Second))
		if err := second.Campaign(ctx); err != nil {
			t.Fatalf("second group campaign after expiry: %v", err)
		}
		t.Cleanup(func() {
			_ = first.Resign(context.Background())
			_ = second.Resign(context.Background())
		})

		active, err := second.ActiveCount(ctx)
		if err != nil {
			t.Fatalf("group active count after expiry: %v", err)
		}
		if active != 1 {
			t.Fatalf("active count after expired takeover = %d, want 1", active)
		}
	})

	t.Run("group renewal loss flips local state", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		elector, err := NewGroup(collection, leader.GroupOptions{
			Options: leader.Options{
				Group:         "group-renewal-loss-token-change",
				MemberID:      "member-1",
				Lease:         time.Second,
				RenewInterval: 25 * time.Millisecond,
			},
			MaxLeaders: 2,
		})
		if err != nil {
			t.Fatalf("new group elector: %v", err)
		}
		if err := elector.Campaign(ctx); err != nil {
			t.Fatalf("group campaign: %v", err)
		}
		elector.mu.RLock()
		slotID := elector.slotID(elector.slot)
		elector.mu.RUnlock()
		_, err = collection.UpdateOne(
			ctx,
			bson.M{"_id": slotID},
			bson.M{"$set": bson.M{"token": "member-2:stolen", "lease_until": time.Now().Add(time.Second).UTC()}},
		)
		if err != nil {
			t.Fatalf("steal group slot: %v", err)
		}

		bttesting.Eventually(t, time.Second, func() bool {
			return !elector.IsLeader()
		})
		if err := elector.Resign(ctx); err != nil {
			t.Fatalf("group resign after lost leadership: %v", err)
		}
	})

	t.Run("group stress bounds concurrent leaders", func(t *testing.T) {
		collection := newTestCollection(ctx, t, client)
		const (
			group      = "group-stress-bounds-concurrent-leaders"
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
				elector, err := NewGroup(collection, leader.GroupOptions{
					Options: leader.Options{
						Group:         group,
						MemberID:      memberID,
						Lease:         2 * time.Second,
						RenewInterval: 100 * time.Millisecond,
					},
					MaxLeaders: maxLeaders,
				}, WithRetryDelay(10*time.Millisecond))
				if err != nil {
					return fmt.Errorf("new group elector %s: %w", memberID, err)
				}

				campaignCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
				if err := elector.Campaign(campaignCtx); err != nil {
					return fmt.Errorf("%s group campaign: %w", memberID, err)
				}
				defer func() {
					resignCtx, resignCancel := context.WithTimeout(context.Background(), time.Second)
					defer resignCancel()
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
					return fmt.Errorf("running group leaders exceeded max: %d > %d", current, maxLeaders)
				}

				active, err := elector.ActiveCount(ctx)
				if err != nil {
					return fmt.Errorf("%s group active count: %w", memberID, err)
				}
				if active > maxLeaders {
					return fmt.Errorf("mongo group active count exceeded max: %d > %d", active, maxLeaders)
				}

				time.Sleep(10 * time.Millisecond)
				return nil
			})
		}

		tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
			Workers:       members,
			RoundsPerTask: 3,
			Timeout:       10 * time.Second,
		})
		report := tester.RunT(t, tasks...)
		if report.Completed != members*3 {
			t.Fatalf("group stress should complete every campaign, got %+v", report)
		}
		if atomic.LoadInt32(&maxRunning) > maxLeaders {
			t.Fatalf("max running group leaders exceeded max: %d > %d", maxRunning, maxLeaders)
		}
	})
}

func TestNewRejectsInvalidInputs(t *testing.T) {
	if _, err := New(nil, leader.Options{Group: "g", MemberID: "m"}); err == nil {
		t.Fatal("expected nil collection error")
	}
	if _, err := New(&mongo.Collection{}, leader.Options{Group: "g", MemberID: "m"}, nil); err == nil {
		t.Fatal("expected nil option error")
	}
	if _, err := New(&mongo.Collection{}, leader.Options{Group: "g", MemberID: "m"}, WithRetryDelay(0)); err == nil {
		t.Fatal("expected invalid retry delay error")
	}
	if _, err := New(&mongo.Collection{}, leader.Options{Group: "g", MemberID: "m"}, WithClock(nil)); err == nil {
		t.Fatal("expected nil clock error")
	}
	if _, err := NewGroup(nil, leader.GroupOptions{Options: leader.Options{Group: "g", MemberID: "m"}, MaxLeaders: 1}); err == nil {
		t.Fatal("expected nil collection error for group")
	}
	if _, err := NewGroup(&mongo.Collection{}, leader.GroupOptions{Options: leader.Options{Group: "g", MemberID: "m"}}); err == nil {
		t.Fatal("expected invalid max leaders error")
	}
	if _, err := NewGroup(&mongo.Collection{}, leader.GroupOptions{Options: leader.Options{Group: "g", MemberID: "m"}, MaxLeaders: 1}, nil); err == nil {
		t.Fatal("expected nil group option error")
	}
}

func TestElectorResignHonorsCallerDeadlineWhileRenewalWorkerIsBlocked(t *testing.T) {
	blockedDone := make(chan struct{})
	elector := &Elector{
		owned: true,
		cancel: func() {
		},
		done: blockedDone,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	started := time.Now()
	err := elector.Resign(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Resign error = %v, want context deadline", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("Resign ignored caller deadline, elapsed=%s", elapsed)
	}
}

func newTestCollection(ctx context.Context, t *testing.T, client *mongo.Client) *mongo.Collection {
	t.Helper()

	name := "leases_" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	collection := client.Database("bluetape_leader_mongo_test").Collection(name)
	if err := EnsureIndexes(ctx, collection); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cleanupCancel()
		if err := collection.Drop(cleanupCtx); err != nil {
			t.Fatalf("drop collection: %v", err)
		}
	})
	return collection
}

func newTestElector(t *testing.T, collection *mongo.Collection, group string, memberID string) *Elector {
	t.Helper()

	elector, err := New(collection, leader.Options{
		Group:         group,
		MemberID:      memberID,
		Lease:         time.Second,
		RenewInterval: 50 * time.Millisecond,
	}, WithRetryDelay(10*time.Millisecond))
	if err != nil {
		t.Fatalf("new elector: %v", err)
	}
	return elector
}

func newTestGroupElector(t *testing.T, collection *mongo.Collection, group string, memberID string, maxLeaders int) *GroupElector {
	t.Helper()

	elector, err := NewGroup(collection, leader.GroupOptions{
		Options: leader.Options{
			Group:         group,
			MemberID:      memberID,
			Lease:         time.Second,
			RenewInterval: 50 * time.Millisecond,
		},
		MaxLeaders: maxLeaders,
	}, WithRetryDelay(10*time.Millisecond))
	if err != nil {
		t.Fatalf("new group elector: %v", err)
	}
	return elector
}
