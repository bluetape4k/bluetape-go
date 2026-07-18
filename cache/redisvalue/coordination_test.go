package redisvalue

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestCoordinatorTokenSerializesSameKey(t *testing.T) {
	registry := newCoordinatorRegistry[string]()
	coordinator := registry.acquire("shared")
	if err := coordinator.acquireToken(context.Background()); err != nil {
		t.Fatal(err)
	}

	entered := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(entered)
		err := coordinator.acquireToken(context.Background())
		if err == nil {
			coordinator.releaseToken()
		}
		done <- err
	}()
	<-entered
	select {
	case err := <-done:
		t.Fatalf("second token acquired early: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	coordinator.releaseToken()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	registry.release("shared", coordinator)
	if registry.active() != 0 {
		t.Fatalf("active coordinators = %d", registry.active())
	}
}

func TestCoordinatorTokenCancellationDoesNotConsumeToken(t *testing.T) {
	coordinator := newKeyCoordinator[string]()
	if err := coordinator.acquireToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := coordinator.acquireToken(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("acquireToken() = %v", err)
	}
	coordinator.releaseToken()
	if err := coordinator.acquireToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	coordinator.releaseToken()
}

func TestCoordinatorFollowersSharePublishedSuccessAndError(t *testing.T) {
	for _, tt := range []struct {
		name    string
		value   string
		err     error
		wantErr error
	}{
		{name: "success", value: "loaded"},
		{name: "error", err: errors.New("loader failed")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			coordinator := newKeyCoordinator[string]()
			flight, leader := coordinator.joinFlight()
			if !leader {
				t.Fatal("first participant is not leader")
			}
			firstFollower, leader := coordinator.joinFlight()
			if leader || firstFollower != flight {
				t.Fatal("first follower did not join active flight")
			}
			secondFollower, leader := coordinator.joinFlight()
			if leader || secondFollower != flight {
				t.Fatal("second follower did not join active flight")
			}
			if flight.participants.Load() != 3 {
				t.Fatalf("participants = %d", flight.participants.Load())
			}

			coordinator.publishFlight(flight, tt.value, tt.err)
			for i := 0; i < 2; i++ {
				got, err := coordinator.waitFlight(context.Background(), flight)
				if got != tt.value || !errors.Is(err, tt.err) {
					t.Fatalf("waitFlight() = %q/%v", got, err)
				}
			}
			if flight.participants.Load() != 0 {
				t.Fatalf("participants after publication = %d", flight.participants.Load())
			}
			if coordinator.flight != nil {
				t.Fatal("published flight remained active")
			}
		})
	}
}

func TestCoordinatorFollowerCancellationDetachesOnlyFollower(t *testing.T) {
	coordinator := newKeyCoordinator[string]()
	flight, _ := coordinator.joinFlight()
	follower, leader := coordinator.joinFlight()
	if leader || follower != flight {
		t.Fatal("follower did not join")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := coordinator.waitFlight(ctx, follower); !errors.Is(err, context.Canceled) {
		t.Fatalf("waitFlight() = %v", err)
	}
	if flight.participants.Load() != 1 {
		t.Fatalf("leader participant was detached: %d", flight.participants.Load())
	}
	coordinator.publishFlight(flight, "loaded", nil)
	if flight.participants.Load() != 0 {
		t.Fatalf("participants = %d", flight.participants.Load())
	}
}

func TestCoordinatorLeaderCancellationBeforeTokenPublishesAndRetires(t *testing.T) {
	registry := newCoordinatorRegistry[string]()
	blocker := registry.acquire("shared")
	if err := blocker.acquireToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	leaderCoordinator := registry.acquire("shared")
	flight, leader := leaderCoordinator.joinFlight()
	if !leader {
		t.Fatal("flight creator is not leader")
	}
	followerCoordinator := registry.acquire("shared")
	follower, leader := followerCoordinator.joinFlight()
	if leader || follower != flight {
		t.Fatal("follower did not join leader flight")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := leaderCoordinator.acquireToken(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("acquireToken() = %v", err)
	}
	leaderCoordinator.publishFlight(flight, "", err)
	if _, followerErr := followerCoordinator.waitFlight(context.Background(), follower); !errors.Is(followerErr, context.Canceled) {
		t.Fatalf("follower error = %v", followerErr)
	}

	blocker.releaseToken()
	registry.release("shared", followerCoordinator)
	registry.release("shared", leaderCoordinator)
	registry.release("shared", blocker)
	if registry.active() != 0 {
		t.Fatalf("registry retained %d coordinators", registry.active())
	}
}

func TestCoordinatorPublicationCancellationArbitration(t *testing.T) {
	t.Run("cancellation first", func(t *testing.T) {
		coordinator := newKeyCoordinator[string]()
		flight, _ := coordinator.joinFlight()
		follower, _ := coordinator.joinFlight()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := coordinator.waitFlight(ctx, follower); !errors.Is(err, context.Canceled) {
			t.Fatalf("waitFlight() = %v", err)
		}
		coordinator.publishFlight(flight, "late", nil)
		if flight.participants.Load() != 0 {
			t.Fatalf("participants = %d", flight.participants.Load())
		}
	})

	t.Run("publication first", func(t *testing.T) {
		coordinator := newKeyCoordinator[string]()
		flight, _ := coordinator.joinFlight()
		follower, _ := coordinator.joinFlight()
		coordinator.publishFlight(flight, "published", nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		value, err := coordinator.waitFlight(ctx, follower)
		if err != nil || value != "published" {
			t.Fatalf("waitFlight() = %q/%v", value, err)
		}
		if flight.participants.Load() != 0 {
			t.Fatalf("participants = %d", flight.participants.Load())
		}
	})
}

func TestCoordinatorNewArrivalAfterPublicationStartsNewGeneration(t *testing.T) {
	coordinator := newKeyCoordinator[string]()
	first, leader := coordinator.joinFlight()
	if !leader {
		t.Fatal("first flight has no leader")
	}
	coordinator.publishFlight(first, "first", nil)
	second, leader := coordinator.joinFlight()
	if !leader || second == first || second.generation <= first.generation {
		t.Fatalf("second flight = %+v, first = %+v, leader=%v", second, first, leader)
	}
	coordinator.publishFlight(second, "second", nil)
}

func TestCoordinatorFlightStateHasNoWaiterCollection(t *testing.T) {
	typeOfFlight := reflect.TypeOf(loadFlight[string]{})
	for i := 0; i < typeOfFlight.NumField(); i++ {
		field := typeOfFlight.Field(i)
		if field.Type.Kind() == reflect.Map || field.Type.Kind() == reflect.Slice {
			t.Fatalf("flight field %s retains a waiter collection: %s", field.Name, field.Type)
		}
	}
}

func TestCoordinatorRegistryReturnsToZeroAfterConcurrentKeys(t *testing.T) {
	registry := newCoordinatorRegistry[string]()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			coordinator := registry.acquire("shared")
			if err := coordinator.acquireToken(context.Background()); err != nil {
				t.Errorf("acquireToken() = %v", err)
				registry.release("shared", coordinator)
				return
			}
			coordinator.releaseToken()
			registry.release("shared", coordinator)
		}()
	}
	wg.Wait()
	if registry.active() != 0 {
		t.Fatalf("registry retained %d coordinators", registry.active())
	}
}

func TestCoordinatorRetirementABADoesNotCreateTwoTokenDomains(t *testing.T) {
	registry := newCoordinatorRegistry[string]()
	first := registry.acquire("shared")
	retireEntered := make(chan struct{})
	retireRelease := make(chan struct{})
	registry.beforeRetire = func() {
		close(retireEntered)
		<-retireRelease
	}
	done := make(chan struct{})
	go func() {
		registry.release("shared", first)
		close(done)
	}()
	<-retireEntered
	nextReady := make(chan *keyCoordinator[string], 1)
	go func() { nextReady <- registry.acquire("shared") }()
	close(retireRelease)
	<-done
	next := <-nextReady
	if first == next {
		t.Fatal("retired coordinator was reused")
	}
	if registry.active() != 1 {
		t.Fatalf("active coordinators = %d", registry.active())
	}
	registry.release("shared", next)
	if registry.active() != 0 {
		t.Fatalf("registry retained %d coordinators", registry.active())
	}
}
