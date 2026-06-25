package jwt

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	tcmongodb "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestMongoRepositoryDistributedProvidersShareRotatedKeys(t *testing.T) {
	ctx := context.Background()
	client := jwtMongoClient(ctx, t)
	repoA := newTestMongoRepository(t, client, "provider-shared", MongoRepositoryOptions{})
	repoB := newTestMongoRepository(t, client, "provider-shared", MongoRepositoryOptions{})

	providerA, err := NewDistributedHMACProvider(ctx, repoA, HS256)
	if err != nil {
		t.Fatalf("NewDistributedHMACProvider(A) error = %v", err)
	}
	providerB, err := NewDistributedHMACProvider(ctx, repoB, HS256)
	if err != nil {
		t.Fatalf("NewDistributedHMACProvider(B) error = %v", err)
	}

	token, err := providerA.ComposeContext(ctx, WithSubject("account-42"), WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("ComposeContext() error = %v", err)
	}
	reader, err := providerB.ParseContext(ctx, token, WithExpectedSubject("account-42"))
	if err != nil {
		t.Fatalf("ParseContext() error = %v", err)
	}
	if reader.Subject() != "account-42" {
		t.Fatalf("Subject() = %q, want account-42", reader.Subject())
	}

	if _, err := providerA.ForcedRotateContext(ctx); err != nil {
		t.Fatalf("ForcedRotateContext() error = %v", err)
	}
	rotatedToken, err := providerA.ComposeContext(ctx, WithSubject("account-43"), WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("ComposeContext(rotated) error = %v", err)
	}
	if _, err := providerB.ParseContext(ctx, rotatedToken, WithExpectedSubject("account-43")); err != nil {
		t.Fatalf("ParseContext(rotated) error = %v", err)
	}
}

func TestMongoRepositoryFindRejectsMissingUnknownAndExpiredKID(t *testing.T) {
	ctx := context.Background()
	client := jwtMongoClient(ctx, t)
	repo := newTestMongoRepository(t, client, "find-reject", MongoRepositoryOptions{})
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	expired := newTestHMACKey(t, "expired", now.Add(-2*time.Hour), time.Hour)
	seedMongoKeyChain(ctx, t, repo, expired, true)

	if _, err := repo.Find(ctx, "", now); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("Find(empty) error = %v, want ErrKeyNotFound", err)
	}
	if _, err := repo.Find(ctx, strings.Repeat("a", maxKIDBytes+1), now); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("Find(long kid) error = %v, want ErrKeyNotFound", err)
	}
	if _, err := repo.Find(ctx, "missing", now); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("Find(missing) error = %v, want ErrKeyNotFound", err)
	}
	if _, err := repo.Find(ctx, "expired", now); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("Find(expired) error = %v, want ErrInvalidKey", err)
	}
}

func TestMongoRepositoryRotateCASReturnsConcurrentWinner(t *testing.T) {
	ctx := context.Background()
	client := jwtMongoClient(ctx, t)
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       1,
		RoundsPerTask: 3,
		Timeout:       3 * time.Second,
	})
	round := 0
	var mu sync.Mutex

	tester.RunT(t, func(context.Context) error {
		mu.Lock()
		round++
		namespace := fmt.Sprintf("mongo-rotate-cas-%d", round)
		mu.Unlock()

		repo := newTestMongoRepository(t, client, namespace, MongoRepositoryOptions{})
		ready := make(chan struct{}, 2)
		release := make(chan struct{})
		results := make(chan string, 2)
		errs := make(chan error, 2)
		startRotate := func(kid string) {
			key, err := repo.Rotate(ctx, func() (*KeyChain, error) {
				ready <- struct{}{}
				<-release
				return newTestHMACKey(t, kid, now, time.Hour), nil
			}, now)
			if err != nil {
				errs <- err
				return
			}
			results <- key.KID()
		}

		go startRotate(namespace + "-a")
		go startRotate(namespace + "-b")
		for i := 0; i < 2; i++ {
			select {
			case <-ready:
			case <-time.After(200 * time.Millisecond):
			}
		}
		close(release)

		got := make([]string, 0, 2)
		for i := 0; i < 2; i++ {
			select {
			case err := <-errs:
				return err
			case kid := <-results:
				got = append(got, kid)
			case <-time.After(2 * time.Second):
				return errorsNew("timed out waiting for rotate results")
			}
		}
		if got[0] != got[1] {
			return fmt.Errorf("concurrent rotate returned different winners: %v", got)
		}
		count, err := repo.keysCollection().CountDocuments(ctx, repo.namespaceFilter())
		if err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("stored key count = %d, want 1", count)
		}
		return nil
	})
}

func TestMongoRepositoryRotateLoserCandidateDoesNotOverwriteCurrentOrLeak(t *testing.T) {
	ctx := context.Background()
	client := jwtMongoClient(ctx, t)
	repoA := newTestMongoRepository(t, client, "rotate-loser", MongoRepositoryOptions{})
	repoB := newTestMongoRepository(t, client, "rotate-loser", MongoRepositoryOptions{})
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)

	winner, err := repoA.Rotate(ctx, func() (*KeyChain, error) {
		return newTestHMACKey(t, "winner", now, time.Hour), nil
	}, now)
	if err != nil {
		t.Fatalf("repoA Rotate() error = %v", err)
	}
	loserResult, err := repoB.storeCAS(ctx, "", newTestHMACKey(t, "loser", now, time.Hour), now)
	if err != nil {
		t.Fatalf("repoB storeCAS() error = %v", err)
	}
	if loserResult.KID() != winner.KID() {
		t.Fatalf("loser storeCAS() kid = %q, want %q", loserResult.KID(), winner.KID())
	}
	current, err := repoB.Current(ctx, now)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if current.KID() != "winner" {
		t.Fatalf("Current() kid = %q, want winner", current.KID())
	}
	if _, err := repoA.Find(ctx, "loser", now); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("Find(loser) error = %v, want ErrKeyNotFound", err)
	}
	count, err := repoA.keysCollection().CountDocuments(ctx, repoA.namespaceFilter())
	if err != nil {
		t.Fatalf("CountDocuments() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("stored key count = %d, want 1", count)
	}
}

func TestMongoRepositoryCapacityTrimPreservesNewestKeys(t *testing.T) {
	ctx := context.Background()
	client := jwtMongoClient(ctx, t)
	repo := newTestMongoRepository(t, client, "capacity", MongoRepositoryOptions{Capacity: 2})
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)

	for i, kid := range []string{"old", "middle", "new"} {
		createdAt := now.Add(time.Duration(i) * time.Minute)
		if _, err := repo.ForcedRotate(ctx, func() (*KeyChain, error) {
			return newTestHMACKey(t, kid, createdAt, time.Hour), nil
		}, createdAt); err != nil {
			t.Fatalf("ForcedRotate(%s) error = %v", kid, err)
		}
	}
	if _, err := repo.Find(ctx, "old", now.Add(3*time.Minute)); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("Find(old) error = %v, want ErrKeyNotFound", err)
	}
	for _, kid := range []string{"middle", "new"} {
		if _, err := repo.Find(ctx, kid, now.Add(3*time.Minute)); err != nil {
			t.Fatalf("Find(%s) error = %v", kid, err)
		}
	}
}

func TestMongoRepositoryCapacityTrimKeepsCandidateAndNewestRetainedKey(t *testing.T) {
	ctx := context.Background()
	client := jwtMongoClient(ctx, t)
	repo := newTestMongoRepository(t, client, "capacity-skew", MongoRepositoryOptions{Capacity: 2})
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)

	for i, kid := range []string{"middle", "new"} {
		createdAt := now.Add(time.Duration(i+1) * time.Minute)
		if _, err := repo.ForcedRotate(ctx, func() (*KeyChain, error) {
			return newTestHMACKey(t, kid, createdAt, time.Hour), nil
		}, createdAt); err != nil {
			t.Fatalf("ForcedRotate(%s) error = %v", kid, err)
		}
	}
	if _, err := repo.ForcedRotate(ctx, func() (*KeyChain, error) {
		return newTestHMACKey(t, "skewed-candidate", now.Add(-time.Minute), time.Hour), nil
	}, now); err != nil {
		t.Fatalf("ForcedRotate(skewed-candidate) error = %v", err)
	}

	count, err := repo.keysCollection().CountDocuments(ctx, repo.namespaceFilter())
	if err != nil {
		t.Fatalf("CountDocuments() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("stored key count = %d, want 2", count)
	}
	if _, err := repo.Find(ctx, "skewed-candidate", now); err != nil {
		t.Fatalf("Find(skewed-candidate) error = %v", err)
	}
	if _, err := repo.Find(ctx, "new", now.Add(3*time.Minute)); err != nil {
		t.Fatalf("Find(new) error = %v", err)
	}
	if _, err := repo.Find(ctx, "middle", now.Add(3*time.Minute)); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("Find(middle) error = %v, want ErrKeyNotFound", err)
	}
}

func TestMongoRepositoryContextCancellationPreserved(t *testing.T) {
	ctx := context.Background()
	client := jwtMongoClient(ctx, t)
	repo := newTestMongoRepository(t, client, "cancel", MongoRepositoryOptions{})
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       2,
		RoundsPerTask: 5,
		Timeout:       2 * time.Second,
	})
	tester.RunT(t, func(ctx context.Context) error {
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		_, err := repo.Find(canceled, "kid", time.Now())
		if !errors.Is(err, context.Canceled) {
			return fmt.Errorf("expected context.Canceled, got %w", err)
		}
		return nil
	})
}

func jwtMongoClient(ctx context.Context, t *testing.T) *mongo.Client {
	t.Helper()
	uri, err := jwtMongoURI(ctx)
	if err != nil {
		t.Fatalf("start mongodb container: %v", err)
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect mongodb: %v", err)
	}
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	return client
}

var jwtMongoFixture struct {
	once      sync.Once
	container *tcmongodb.MongoDBContainer
	uri       string
	err       error
}

func jwtMongoURI(ctx context.Context) (string, error) {
	jwtMongoFixture.once.Do(func() {
		container, err := tcmongodb.Run(ctx, "mongo:7.0")
		if err != nil {
			jwtMongoFixture.err = err
			return
		}
		uri, err := container.ConnectionString(ctx)
		if err != nil {
			jwtMongoFixture.err = err
			_ = testcleanup.Terminate(ctx, 0, container)
			return
		}
		jwtMongoFixture.container = container
		jwtMongoFixture.uri = uri
	})
	return jwtMongoFixture.uri, jwtMongoFixture.err
}

func newTestMongoRepository(t *testing.T, client *mongo.Client, namespace string, options MongoRepositoryOptions) *MongoRepository {
	t.Helper()
	if options.Client == nil {
		options.Client = client
	}
	if options.Database == "" {
		options.Database = "bluetape_jwt_test"
	}
	if options.Collection == "" {
		options.Collection = "keychains"
	}
	if options.Namespace == "" {
		options.Namespace = testRedisRepositoryNamespace(t, namespace)
	}
	repo, err := NewMongoRepository(options)
	if err != nil {
		t.Fatalf("NewMongoRepository() error = %v", err)
	}
	t.Cleanup(func() { _ = repo.DeleteAll(context.Background()) })
	return repo
}

func seedMongoKeyChain(ctx context.Context, t *testing.T, repo *MongoRepository, key *KeyChain, current bool) {
	t.Helper()
	if _, err := repo.ForcedRotate(ctx, func() (*KeyChain, error) { return key, nil }, key.CreatedAt()); err != nil {
		t.Fatalf("seed mongo keychain: %v", err)
	}
	if !current {
		return
	}
	if err := repo.setCurrent(ctx, key.KID()); err != nil {
		t.Fatalf("seed mongo current: %v", err)
	}
}
