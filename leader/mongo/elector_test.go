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
	mongodbtestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/mongodb"
	bttesting "github.com/bluetape4k/bluetape-go/testing"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

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
