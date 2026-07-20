package leader_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	"github.com/bluetape4k/bluetape-go/leader"
	etcdleader "github.com/bluetape4k/bluetape-go/leader/etcd"
	mongoleader "github.com/bluetape4k/bluetape-go/leader/mongo"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	sqlleader "github.com/bluetape4k/bluetape-go/leader/sql"
	mongodbtestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/mongodb"
	postgrestestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/postgres"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	mobyclient "github.com/moby/moby/client"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcetcd "github.com/testcontainers/testcontainers-go/modules/etcd"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	providerBenchmarkEnv    = "BLUETAPE_LEADER_PROVIDER_BENCH"
	providerStartupTimeout  = 90 * time.Second
	providerOperationLimit  = 10 * time.Second
	providerAttemptLimit    = 5 * time.Second
	providerRoundLimit      = 10 * time.Second
	providerLease           = 30 * time.Second
	providerExpiryLease     = time.Second
	providerExpiryPoll      = 25 * time.Millisecond
	providerCleanupTimeout  = 10 * time.Second
	providerEtcdVersion     = "v3.6.13"
	providerEtcdImagePrefix = "gcr.io/etcd-development/etcd@"
)

var providerBenchmarkSink []leaderRoundResult

var providerEtcdDigests = map[string]string{
	"linux/amd64": "sha256:946dfbae58b1dec56af786a23e7322484b58281547bef1e848321f6beeb388d5",
	"linux/arm64": "sha256:23c14fbdf70105a54146cf5ed3a81613b99a973c60d5907851a251ca15664e96",
}

type leaderRoundOptions struct {
	workers      int
	attemptLimit time.Duration
	roundLimit   time.Duration
}

type leaderRoundResult struct {
	worker int
	member string
	leader string
	won    bool
}

func runLeaderRound(
	ctx context.Context,
	opts leaderRoundOptions,
	worker func(context.Context, int) (leaderRoundResult, error),
) ([]leaderRoundResult, error) {
	if ctx == nil {
		return nil, errors.New("leader round context must not be nil")
	}
	if opts.workers <= 0 || opts.attemptLimit <= 0 || opts.roundLimit <= 0 || worker == nil {
		return nil, errors.New("leader round requires positive limits and a worker")
	}

	deadlineCtx, cancelDeadline := context.WithTimeout(ctx, opts.roundLimit)
	defer cancelDeadline()
	winnerCancellation := errors.New("leader round stopped after winner")
	roundCtx, cancelRound := context.WithCancelCause(deadlineCtx)
	defer cancelRound(nil)

	start := make(chan struct{})
	ready := make(chan struct{}, opts.workers)
	results := make(chan leaderRoundResult, opts.workers)
	var workers sync.WaitGroup
	var firstErr error
	var firstErrOnce sync.Once
	var stoppedOnWinner atomic.Bool

	for workerID := range opts.workers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			ready <- struct{}{}
			select {
			case <-start:
			case <-roundCtx.Done():
				// Winner cancellation can happen only after start closes. If both
				// are ready, consume the opened barrier before observing cancellation.
				select {
				case <-start:
				default:
					firstErrOnce.Do(func() { firstErr = roundCtx.Err() })
					results <- leaderRoundResult{worker: workerID}
					return
				}
			}

			attemptCtx, cancelAttempt := context.WithTimeout(roundCtx, opts.attemptLimit)
			result, err := worker(attemptCtx, workerID)
			cancelAttempt()
			result.worker = workerID
			if result.won && stoppedOnWinner.CompareAndSwap(false, true) {
				cancelRound(winnerCancellation)
			}
			winnerStopped := stoppedOnWinner.Load() && errors.Is(context.Cause(roundCtx), winnerCancellation)
			if err != nil && !(winnerStopped && errors.Is(err, context.Canceled)) {
				firstErrOnce.Do(func() {
					firstErr = fmt.Errorf("leader round worker %d: %w", workerID, err)
					cancelRound(err)
				})
			}
			results <- result
		}()
	}

	for range opts.workers {
		select {
		case <-ready:
		case <-roundCtx.Done():
			firstErrOnce.Do(func() { firstErr = roundCtx.Err() })
		}
	}
	close(start)

	joined := make(chan struct{})
	go func() {
		workers.Wait()
		close(results)
		close(joined)
	}()

	collected := make([]leaderRoundResult, 0, opts.workers)
	for result := range results {
		collected = append(collected, result)
	}
	<-joined
	return collected, firstErr
}

type leaderProviderFixture struct {
	name               string
	providerVersion    string
	imageReference     string
	newElector         func(member, group string) (leader.Elector, error)
	observe            func(context.Context, string) (string, error)
	replace            func(context.Context, string, string, string) error
	rejectStaleRelease func(context.Context, string, string) (bool, error)
	cleanup            func(context.Context, string) error
	close              func(context.Context) error
	abort              func(context.Context, leader.Elector) error
}

type providerEtcdStaleRelease struct {
	key            string
	createRevision int64
}

type providerCampaignEntryContext struct {
	context.Context
	once    sync.Once
	entered chan struct{}
}

func (ctx *providerCampaignEntryContext) Err() error {
	ctx.once.Do(func() { close(ctx.entered) })
	return ctx.Context.Err()
}

type providerElectorResource struct {
	group   string
	elector leader.Elector
	close   func(context.Context) error
	closed  bool
}

type providerResourceRegistry struct {
	mu        sync.Mutex
	resources map[leader.Elector]*providerElectorResource
}

func newProviderResourceRegistry() *providerResourceRegistry {
	return &providerResourceRegistry{resources: make(map[leader.Elector]*providerElectorResource)}
}

func (registry *providerResourceRegistry) add(group string, elector leader.Elector, closeFn func(context.Context) error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.resources[elector] = &providerElectorResource{group: group, elector: elector, close: closeFn}
}

func (registry *providerResourceRegistry) abort(ctx context.Context, elector leader.Elector) error {
	registry.mu.Lock()
	resource := registry.resources[elector]
	if resource == nil {
		registry.mu.Unlock()
		return errors.New("provider elector resource is not registered")
	}
	if resource.closed {
		registry.mu.Unlock()
		return nil
	}
	resource.closed = true
	closeFn := resource.close
	registry.mu.Unlock()
	return runBoundedCleanup(ctx, closeFn)
}

func (registry *providerResourceRegistry) take(group string) []*providerElectorResource {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	resources := make([]*providerElectorResource, 0, len(registry.resources))
	for elector, resource := range registry.resources {
		if group != "" && resource.group != group {
			continue
		}
		resources = append(resources, resource)
		delete(registry.resources, elector)
	}
	return resources
}

func cleanupProviderResources(
	ctx context.Context,
	registry *providerResourceRegistry,
	group string,
	backendCleanup func(context.Context, string) error,
) error {
	resources := registry.take(group)
	var resignErr error
	for _, resource := range resources {
		if resource.closed {
			continue
		}
		resignErr = errors.Join(resignErr, runBoundedCleanup(ctx, resource.elector.Resign))
	}
	backendErr := runBoundedCleanup(ctx, func(cleanupCtx context.Context) error {
		return backendCleanup(cleanupCtx, group)
	})
	var closeErr error
	for _, resource := range resources {
		if resource.closed {
			continue
		}
		resource.closed = true
		closeErr = errors.Join(closeErr, runBoundedCleanup(ctx, resource.close))
	}
	return errors.Join(resignErr, backendErr, closeErr)
}

func runBoundedCleanup(ctx context.Context, cleanup func(context.Context) error) error {
	if cleanup == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), providerCleanupTimeout)
	defer cancel()
	return cleanup(cleanupCtx)
}

func registerProviderFixtureCleanup(tb testing.TB, fixture leaderProviderFixture) {
	tb.Helper()
	tb.Cleanup(func() {
		ctx := context.Background()
		err := errors.Join(
			runBoundedCleanup(ctx, func(cleanupCtx context.Context) error { return fixture.cleanup(cleanupCtx, "") }),
			runBoundedCleanup(ctx, fixture.close),
		)
		if err != nil {
			tb.Errorf("cleanup %s provider fixture: %v", fixture.name, err)
		}
	})
}

func providerOptions(prefix, group, member string, lease time.Duration) leader.Options {
	renewInterval := lease / 3
	if lease == providerExpiryLease {
		renewInterval = 250 * time.Millisecond
	}
	return leader.Options{
		Group:         group,
		MemberID:      member,
		Lease:         lease,
		RenewInterval: renewInterval,
		KeyPrefix:     prefix,
	}
}

func randomProviderID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate provider benchmark id: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func mustProviderID(tb testing.TB) string {
	tb.Helper()
	value, err := randomProviderID()
	if err != nil {
		tb.Fatal(err)
	}
	return value
}

func providerSetupContext(tb testing.TB) (context.Context, context.CancelFunc) {
	tb.Helper()
	return context.WithTimeout(context.Background(), providerStartupTimeout)
}

func operationContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), providerOperationLimit)
}

func newRedisProviderFixture(tb testing.TB, lease time.Duration) leaderProviderFixture {
	tb.Helper()
	ctx, cancel := providerSetupContext(tb)
	defer cancel()
	server := redistestcontainer.StartServer(ctx, tb)
	details, err := server.ConnectionDetails(ctx)
	if err != nil {
		tb.Fatalf("redis connection details: %v", err)
	}
	address, err := details.Require(redistestcontainer.AddressKey)
	if err != nil {
		tb.Fatalf("redis address: %v", err)
	}
	control := redis.NewClient(&redis.Options{Addr: address})
	if err := control.Ping(ctx).Err(); err != nil {
		_ = control.Close()
		tb.Fatalf("ping redis provider: %v", err)
	}
	prefix := mustProviderID(tb)
	registry := newProviderResourceRegistry()
	backendCleanup := func(ctx context.Context, group string) error {
		if group != "" {
			return control.Del(ctx, prefix+":"+group).Err()
		}
		var cursor uint64
		var joined error
		for {
			keys, next, scanErr := control.Scan(ctx, cursor, prefix+":*", 128).Result()
			joined = errors.Join(joined, scanErr)
			if scanErr != nil {
				return joined
			}
			if len(keys) > 0 {
				joined = errors.Join(joined, control.Del(ctx, keys...).Err())
			}
			cursor = next
			if cursor == 0 {
				return joined
			}
		}
	}
	fixture := leaderProviderFixture{
		name:            "Redis",
		providerVersion: "7.4",
		imageReference:  "redis:7.4-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99",
		newElector: func(member, group string) (leader.Elector, error) {
			client := redis.NewClient(&redis.Options{Addr: address})
			readyCtx, readyCancel := operationContext()
			err := client.Ping(readyCtx).Err()
			readyCancel()
			if err != nil {
				_ = client.Close()
				return nil, fmt.Errorf("ping redis elector client: %w", err)
			}
			elector, err := redisleader.New(client, providerOptions(prefix, group, member, lease))
			if err != nil {
				_ = client.Close()
				return nil, err
			}
			registry.add(group, elector, func(context.Context) error { return client.Close() })
			return elector, nil
		},
		observe: func(ctx context.Context, group string) (string, error) {
			value, err := control.Get(ctx, prefix+":"+group).Result()
			if errors.Is(err, redis.Nil) {
				return "", nil
			}
			return value, err
		},
		replace: func(ctx context.Context, group, staleOwner, replacementOwner string) error {
			const replaceScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
  redis.call("psetex", KEYS[1], ARGV[3], ARGV[2])
  return 1
end
return 0`
			replaced, err := control.Eval(ctx, replaceScript, []string{prefix + ":" + group}, staleOwner, replacementOwner, lease.Milliseconds()).Int64()
			if err != nil {
				return err
			}
			if replaced != 1 {
				return errors.New("redis replacement control did not match the active owner")
			}
			return nil
		},
		rejectStaleRelease: func(ctx context.Context, group, staleOwner string) (bool, error) {
			const releaseScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0`
			deleted, err := control.Eval(ctx, releaseScript, []string{prefix + ":" + group}, staleOwner).Int64()
			return deleted == 0, err
		},
		cleanup: func(ctx context.Context, group string) error {
			return cleanupProviderResources(ctx, registry, group, backendCleanup)
		},
		close: func(context.Context) error { return control.Close() },
		abort: registry.abort,
	}
	registerProviderFixtureCleanup(tb, fixture)
	return fixture
}

func newMongoProviderFixture(tb testing.TB, lease time.Duration) leaderProviderFixture {
	tb.Helper()
	ctx, cancel := providerSetupContext(tb)
	defer cancel()
	server := mongodbtestcontainer.StartServer(ctx, tb)
	details, err := server.ConnectionDetails(ctx)
	if err != nil {
		tb.Fatalf("mongodb connection details: %v", err)
	}
	uri, err := details.Require(mongodbtestcontainer.URIKey)
	if err != nil {
		tb.Fatalf("mongodb URI: %v", err)
	}
	control, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		tb.Fatalf("connect mongodb provider: %v", err)
	}
	if err := control.Ping(ctx, readpref.Primary()); err != nil {
		_ = control.Disconnect(context.Background())
		tb.Fatalf("ping mongodb provider: %v", err)
	}
	prefix := mustProviderID(tb)
	database := control.Database("leader_benchmark_" + prefix)
	collection := database.Collection("leases")
	registry := newProviderResourceRegistry()
	backendCleanup := func(ctx context.Context, group string) error {
		if group == "" {
			return database.Drop(ctx)
		}
		_, err := collection.DeleteOne(ctx, bson.M{"_id": prefix + ":" + group})
		return err
	}
	fixture := leaderProviderFixture{
		name:            "MongoDB",
		providerVersion: "7.0",
		imageReference:  "mongo:7.0@sha256:340c1c56fb10e95cf79ff547f8664b96bc6ead9909bc355238cbf865a9695a6f",
		newElector: func(member, group string) (leader.Elector, error) {
			client, err := mongo.Connect(options.Client().ApplyURI(uri))
			if err != nil {
				return nil, err
			}
			readyCtx, readyCancel := operationContext()
			err = client.Ping(readyCtx, readpref.Primary())
			readyCancel()
			if err != nil {
				_ = client.Disconnect(context.Background())
				return nil, fmt.Errorf("ping mongodb elector client: %w", err)
			}
			elector, err := mongoleader.New(client.Database(database.Name()).Collection("leases"), providerOptions(prefix, group, member, lease), mongoleader.WithRetryDelay(providerExpiryPoll))
			if err != nil {
				_ = client.Disconnect(context.Background())
				return nil, err
			}
			registry.add(group, elector, client.Disconnect)
			return elector, nil
		},
		observe: func(ctx context.Context, group string) (string, error) {
			var document struct {
				Token      string    `bson:"token"`
				LeaseUntil time.Time `bson:"lease_until"`
			}
			err := collection.FindOne(ctx, bson.M{"_id": prefix + ":" + group, "lease_until": bson.M{"$gt": time.Now().UTC()}}).Decode(&document)
			if errors.Is(err, mongo.ErrNoDocuments) {
				return "", nil
			}
			return document.Token, err
		},
		replace: func(ctx context.Context, group, staleOwner, replacementOwner string) error {
			result, err := collection.UpdateOne(ctx,
				bson.M{"_id": prefix + ":" + group, "token": staleOwner},
				bson.M{"$set": bson.M{
					"member_id":   "replacement-control",
					"token":       replacementOwner,
					"lease_until": time.Now().UTC().Add(lease),
					"updated_at":  time.Now().UTC(),
				}},
			)
			if err != nil {
				return err
			}
			if result.MatchedCount != 1 {
				return errors.New("mongodb replacement control did not match the active owner")
			}
			return nil
		},
		rejectStaleRelease: func(ctx context.Context, group, staleOwner string) (bool, error) {
			result, err := collection.DeleteOne(ctx, bson.M{"_id": prefix + ":" + group, "token": staleOwner})
			if err != nil {
				return false, err
			}
			return result.DeletedCount == 0, nil
		},
		cleanup: func(ctx context.Context, group string) error {
			return cleanupProviderResources(ctx, registry, group, backendCleanup)
		},
		close: control.Disconnect,
		abort: registry.abort,
	}
	registerProviderFixtureCleanup(tb, fixture)
	return fixture
}

func newPostgresProviderFixture(tb testing.TB, lease time.Duration) leaderProviderFixture {
	tb.Helper()
	ctx, cancel := providerSetupContext(tb)
	defer cancel()
	server := postgrestestcontainer.StartServer(ctx, tb)
	details, err := server.ConnectionDetails(ctx)
	if err != nil {
		tb.Fatalf("postgres connection details: %v", err)
	}
	dsn, err := details.Require(postgrestestcontainer.ConnectionStringKey)
	if err != nil {
		tb.Fatalf("postgres connection string: %v", err)
	}
	control, err := sql.Open("pgx", dsn)
	if err != nil {
		tb.Fatalf("open postgres provider: %v", err)
	}
	if err := control.PingContext(ctx); err != nil {
		_ = control.Close()
		tb.Fatalf("ping postgres provider: %v", err)
	}
	if _, err := control.ExecContext(ctx, sqlleader.SchemaSQL); err != nil {
		_ = control.Close()
		tb.Fatalf("create postgres leader schema: %v", err)
	}
	prefix := mustProviderID(tb)
	registry := newProviderResourceRegistry()
	backendCleanup := func(ctx context.Context, group string) error {
		if group == "" {
			_, err := control.ExecContext(ctx, `delete from public.bluetape_leader_leases where leader_key like $1`, prefix+":%")
			return err
		}
		_, err := control.ExecContext(ctx, `delete from public.bluetape_leader_leases where leader_key=$1`, prefix+":"+group)
		return err
	}
	fixture := leaderProviderFixture{
		name:            "PostgreSQL",
		providerVersion: "16",
		imageReference:  "postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777",
		newElector: func(member, group string) (leader.Elector, error) {
			db, err := sql.Open("pgx", dsn)
			if err != nil {
				return nil, err
			}
			readyCtx, readyCancel := operationContext()
			err = db.PingContext(readyCtx)
			readyCancel()
			if err != nil {
				_ = db.Close()
				return nil, fmt.Errorf("ping postgres elector pool: %w", err)
			}
			elector, err := sqlleader.New(db, providerOptions(prefix, group, member, lease))
			if err != nil {
				_ = db.Close()
				return nil, err
			}
			registry.add(group, elector, func(context.Context) error { return db.Close() })
			return elector, nil
		},
		observe: func(ctx context.Context, group string) (string, error) {
			var owner string
			err := control.QueryRowContext(ctx, `select owner_token from public.bluetape_leader_leases where leader_key=$1 and lease_until>pg_catalog.clock_timestamp()`, prefix+":"+group).Scan(&owner)
			if errors.Is(err, sql.ErrNoRows) {
				return "", nil
			}
			return owner, err
		},
		replace: func(ctx context.Context, group, staleOwner, replacementOwner string) error {
			result, err := control.ExecContext(ctx, `update public.bluetape_leader_leases
set member_id='replacement-control', owner_token=$3,
lease_until=pg_catalog.clock_timestamp()+$4::bigint*interval '1 microsecond',
updated_at=pg_catalog.clock_timestamp() where leader_key=$1 and owner_token=$2`,
				prefix+":"+group, staleOwner, replacementOwner, lease.Microseconds())
			if err != nil {
				return err
			}
			rows, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if rows != 1 {
				return fmt.Errorf("postgres replacement control updated %d rows, want 1", rows)
			}
			return nil
		},
		rejectStaleRelease: func(ctx context.Context, group, staleOwner string) (bool, error) {
			result, err := control.ExecContext(ctx, `delete from public.bluetape_leader_leases where leader_key=$1 and owner_token=$2`, prefix+":"+group, staleOwner)
			if err != nil {
				return false, err
			}
			rows, err := result.RowsAffected()
			if err != nil {
				return false, err
			}
			return rows == 0, nil
		},
		cleanup: func(ctx context.Context, group string) error {
			return cleanupProviderResources(ctx, registry, group, backendCleanup)
		},
		close: func(context.Context) error { return control.Close() },
		abort: registry.abort,
	}
	registerProviderFixtureCleanup(tb, fixture)
	return fixture
}

func newEtcdProviderFixture(tb testing.TB, lease time.Duration) leaderProviderFixture {
	tb.Helper()
	ctx, cancel := providerSetupContext(tb)
	defer cancel()
	platform, err := providerContainerPlatform(ctx)
	if err != nil {
		tb.Fatalf("resolve etcd container platform: %v", err)
	}
	digest, ok := providerEtcdDigests[platform]
	if !ok {
		tb.Fatalf("no approved etcd digest for platform %q", platform)
	}
	image := providerEtcdImagePrefix + digest
	container, err := tcetcd.Run(ctx, image, testcontainers.WithImagePlatform(platform))
	if err != nil {
		if container != nil {
			err = errors.Join(err, testcleanup.Terminate(ctx, providerCleanupTimeout, container))
		}
		tb.Fatal(testcleanup.FormatStartError("etcd", image, err))
	}
	testcleanup.Register(ctx, tb, "etcd", container)
	endpoints, err := container.ClientEndpoints(ctx)
	if err != nil {
		tb.Fatalf("resolve etcd client endpoints: %v", err)
	}
	control, err := clientv3.New(clientv3.Config{Endpoints: endpoints, DialTimeout: 5 * time.Second})
	if err != nil {
		tb.Fatalf("create etcd provider client: %v", err)
	}
	if err := waitForProviderEtcdReady(ctx, control, endpoints); err != nil {
		_ = control.Close()
		tb.Fatalf("wait for etcd provider readiness: %v", err)
	}
	prefix := mustProviderID(tb)
	registry := newProviderResourceRegistry()
	var staleReleaseMu sync.Mutex
	staleReleases := make(map[string]providerEtcdStaleRelease)
	backendCleanup := func(ctx context.Context, group string) error {
		_, err := control.Delete(ctx, providerEtcdCleanupPrefix(prefix, group), clientv3.WithPrefix())
		return err
	}
	fixture := leaderProviderFixture{
		name:            "etcd",
		providerVersion: providerEtcdVersion,
		imageReference:  image,
		newElector: func(member, group string) (leader.Elector, error) {
			client, err := clientv3.New(clientv3.Config{Endpoints: endpoints, DialTimeout: 5 * time.Second})
			if err != nil {
				return nil, err
			}
			readyCtx, readyCancel := operationContext()
			err = waitForProviderEtcdReady(readyCtx, client, endpoints)
			readyCancel()
			if err != nil {
				_ = client.Close()
				return nil, err
			}
			elector, err := etcdleader.New(client, providerOptions(prefix, group, member, lease))
			if err != nil {
				_ = client.Close()
				return nil, err
			}
			registry.add(group, elector, func(context.Context) error { return client.Close() })
			return elector, nil
		},
		observe: func(ctx context.Context, group string) (string, error) {
			observer, err := etcdleader.New(control, providerOptions(prefix, group, "observer", lease))
			if err != nil {
				return "", err
			}
			return observer.Leader(ctx)
		},
		replace: func(ctx context.Context, group, staleOwner, replacementOwner string) error {
			groupPrefix := providerEtcdCleanupPrefix(prefix, group)
			getOptions := append([]clientv3.OpOption{clientv3.WithPrefix()}, clientv3.WithFirstCreate()...)
			current, err := control.Get(ctx, groupPrefix, getOptions...)
			if err != nil {
				return err
			}
			if len(current.Kvs) != 1 || current.Kvs[0] == nil {
				return errors.New("etcd replacement control found no active owner")
			}
			ttl := int64((lease + time.Second - 1) / time.Second)
			grant, err := control.Grant(ctx, ttl)
			if err != nil {
				return err
			}
			old := current.Kvs[0]
			newKey := groupPrefix + fmt.Sprintf("%x", grant.ID)
			transaction, err := control.Txn(ctx).
				If(
					clientv3.Compare(clientv3.CreateRevision(string(old.Key)), "=", old.CreateRevision),
					clientv3.Compare(clientv3.Value(string(old.Key)), "=", staleOwner),
					clientv3.Compare(clientv3.CreateRevision(newKey), "=", 0),
				).
				Then(
					clientv3.OpDelete(string(old.Key)),
					clientv3.OpPut(newKey, replacementOwner, clientv3.WithLease(grant.ID)),
				).
				Commit()
			if err != nil || !transaction.Succeeded {
				revokeErr := runBoundedCleanup(context.WithoutCancel(ctx), func(cleanupCtx context.Context) error {
					_, revokeErr := control.Revoke(cleanupCtx, grant.ID)
					return revokeErr
				})
				if err == nil {
					err = errors.New("etcd replacement control transaction lost its compare")
				}
				return errors.Join(err, revokeErr)
			}
			staleReleaseMu.Lock()
			staleReleases[group] = providerEtcdStaleRelease{
				key:            string(old.Key),
				createRevision: old.CreateRevision,
			}
			staleReleaseMu.Unlock()
			return nil
		},
		rejectStaleRelease: func(ctx context.Context, group, staleOwner string) (bool, error) {
			staleReleaseMu.Lock()
			staleRelease, ok := staleReleases[group]
			staleReleaseMu.Unlock()
			if !ok {
				return false, errors.New("etcd stale release control has no replacement receipt")
			}
			transaction, err := control.Txn(ctx).
				If(
					clientv3.Compare(clientv3.CreateRevision(staleRelease.key), "=", staleRelease.createRevision),
					clientv3.Compare(clientv3.Value(staleRelease.key), "=", staleOwner),
				).
				Then(clientv3.OpDelete(staleRelease.key)).
				Commit()
			if err != nil {
				return false, err
			}
			return !transaction.Succeeded, nil
		},
		cleanup: func(ctx context.Context, group string) error {
			return cleanupProviderResources(ctx, registry, group, backendCleanup)
		},
		close: func(context.Context) error { return control.Close() },
		abort: registry.abort,
	}
	registerProviderFixtureCleanup(tb, fixture)
	return fixture
}

func providerContainerPlatform(ctx context.Context) (string, error) {
	platform := strings.TrimSpace(os.Getenv("DOCKER_DEFAULT_PLATFORM"))
	if platform == "" {
		client, err := testcontainers.NewDockerClientWithOpts(ctx)
		if err != nil {
			return "", err
		}
		defer func() { _ = client.Close() }()
		info, err := client.Info(ctx, mobyclient.InfoOptions{})
		if err != nil {
			return "", fmt.Errorf("read docker daemon platform: %w", err)
		}
		platform = info.Info.OSType + "/" + info.Info.Architecture
	}
	parts := strings.Split(platform, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) != "linux" {
		return "", fmt.Errorf("unsupported docker platform %q", platform)
	}
	architecture := strings.TrimSpace(parts[1])
	switch architecture {
	case "x86_64":
		architecture = "amd64"
	case "aarch64":
		architecture = "arm64"
	}
	normalized := "linux/" + architecture
	if _, ok := providerEtcdDigests[normalized]; !ok {
		return "", fmt.Errorf("unsupported docker platform %q", normalized)
	}
	return normalized, nil
}

func waitForProviderEtcdReady(ctx context.Context, client *clientv3.Client, endpoints []string) error {
	if len(endpoints) == 0 {
		return errors.New("etcd provider has no client endpoints")
	}
	var lastErr error
	for ctx.Err() == nil {
		ready := true
		for _, endpoint := range endpoints {
			status, err := client.Status(ctx, endpoint)
			if err != nil || status == nil || status.Header == nil || status.Header.MemberId == 0 || status.Leader == 0 {
				ready = false
				lastErr = err
				break
			}
		}
		if ready {
			id, err := randomProviderID()
			if err != nil {
				return err
			}
			key := "/bluetape4k/test/readiness/" + id
			if _, err := client.Put(ctx, key, "ready"); err != nil {
				return fmt.Errorf("etcd readiness put: %w", err)
			}
			response, err := client.Get(ctx, key)
			if err != nil {
				return fmt.Errorf("etcd readiness get: %w", err)
			}
			_, deleteErr := client.Delete(ctx, key)
			if len(response.Kvs) != 1 || string(response.Kvs[0].Value) != "ready" {
				return errors.Join(errors.New("etcd readiness returned an unexpected value"), deleteErr)
			}
			return deleteErr
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	return errors.Join(ctx.Err(), lastErr)
}

func providerEtcdCleanupPrefix(prefix, group string) string {
	encode := base64.RawURLEncoding.EncodeToString
	path := "/bluetape4k/leader/" + encode([]byte(prefix)) + "/"
	if group != "" {
		path += encode([]byte(group)) + "/"
	}
	return path
}

type leaderProviderFactory struct {
	name string
	open func(testing.TB, time.Duration) leaderProviderFixture
}

var leaderProviderFactories = []leaderProviderFactory{
	{name: "Redis", open: newRedisProviderFixture},
	{name: "MongoDB", open: newMongoProviderFixture},
	{name: "PostgreSQL", open: newPostgresProviderFixture},
	{name: "etcd", open: newEtcdProviderFixture},
}

func BenchmarkProviderLeaderLocal(b *testing.B) {
	b.Run("LocalHarness/CampaignContention/N=8", func(b *testing.B) {
		const workers = 8
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			var winner atomic.Int32
			winner.Store(-1)
			results, err := runLeaderRound(context.Background(), leaderRoundOptions{
				workers:      workers,
				attemptLimit: providerAttemptLimit,
				roundLimit:   providerRoundLimit,
			}, func(_ context.Context, worker int) (leaderRoundResult, error) {
				won := winner.CompareAndSwap(-1, int32(worker))
				return leaderRoundResult{member: fmt.Sprintf("member-%d", worker), won: won}, nil
			})
			if err != nil {
				b.Fatalf("run local leader round: %v", err)
			}
			if got := countRoundWinners(results); got != 1 {
				b.Fatalf("local winners = %d, want 1", got)
			}
			providerBenchmarkSink = results
		}
	})
}

func BenchmarkProviderLeaderContainers(b *testing.B) {
	if os.Getenv(providerBenchmarkEnv) != "1" {
		b.Skipf("set %s=1 to run provider container benchmarks", providerBenchmarkEnv)
	}
	for _, factory := range leaderProviderFactories {
		b.Run(factory.name, func(b *testing.B) {
			b.Run("CampaignUncontended", func(b *testing.B) {
				runProviderLeaderBenchmark(b, factory, "CampaignUncontended", providerLease, 1)
			})
			b.Run("ResignOwned", func(b *testing.B) {
				runProviderLeaderBenchmark(b, factory, "ResignOwned", providerLease, 1)
			})
			b.Run("CampaignContention/N=8", func(b *testing.B) {
				runProviderLeaderBenchmark(b, factory, "CampaignContention", providerLease, 8)
			})
			b.Run("LeaderLookup", func(b *testing.B) {
				runProviderLeaderBenchmark(b, factory, "LeaderLookup", providerLease, 1)
			})
			b.Run("ExpiryTakeover", func(b *testing.B) {
				runProviderLeaderBenchmark(b, factory, "ExpiryTakeover", providerExpiryLease, 1)
			})
		})
	}
}

func runProviderLeaderBenchmark(b *testing.B, factory leaderProviderFactory, scenario string, lease time.Duration, workers int) {
	b.Helper()
	b.StopTimer()
	fixture := factory.open(b, lease)
	b.Logf("provider_version=%q image_reference=%q", sanitizeProviderMetadata(fixture.providerVersion), sanitizeProviderMetadata(fixture.imageReference))
	b.ReportAllocs()
	b.ResetTimer()
	b.StopTimer()

	for range b.N {
		group := mustProviderID(b)
		var results []leaderRoundResult
		var runErr error
		switch scenario {
		case "CampaignUncontended", "CampaignContention":
			electors := make([]leader.Elector, workers)
			for worker := range workers {
				electors[worker], runErr = fixture.newElector(fmt.Sprintf("member-%d", worker), group)
				if runErr != nil {
					break
				}
			}
			if runErr == nil {
				b.StartTimer()
				results, runErr = runLeaderRound(context.Background(), leaderRoundOptions{
					workers:      workers,
					attemptLimit: providerAttemptLimit,
					roundLimit:   providerRoundLimit,
				}, func(ctx context.Context, worker int) (leaderRoundResult, error) {
					err := electors[worker].Campaign(ctx)
					return leaderRoundResult{member: fmt.Sprintf("member-%d", worker), won: err == nil}, err
				})
				b.StopTimer()
			}
			if runErr == nil {
				runErr = verifyCampaignRound(fixture, group, results, electors)
			}
		case "ResignOwned":
			var elector leader.Elector
			elector, runErr = fixture.newElector("member-0", group)
			var seededOwner string
			if runErr == nil {
				seededOwner, runErr = seedProviderOwner(fixture, group, elector)
			}
			if runErr == nil {
				b.StartTimer()
				results, runErr = runLeaderRound(context.Background(), leaderRoundOptions{workers: 1, attemptLimit: providerAttemptLimit, roundLimit: providerRoundLimit}, func(ctx context.Context, _ int) (leaderRoundResult, error) {
					return leaderRoundResult{member: "member-0"}, elector.Resign(ctx)
				})
				b.StopTimer()
			}
			if runErr == nil {
				runErr = verifyOwnerAbsent(fixture, group, seededOwner)
			}
		case "LeaderLookup":
			var elector leader.Elector
			elector, runErr = fixture.newElector("member-0", group)
			var seededOwner string
			if runErr == nil {
				seededOwner, runErr = seedProviderOwner(fixture, group, elector)
			}
			if runErr == nil {
				b.StartTimer()
				results, runErr = runLeaderRound(context.Background(), leaderRoundOptions{workers: 1, attemptLimit: providerAttemptLimit, roundLimit: providerRoundLimit}, func(ctx context.Context, _ int) (leaderRoundResult, error) {
					owner, err := elector.Leader(ctx)
					return leaderRoundResult{member: "member-0", leader: owner}, err
				})
				b.StopTimer()
			}
			if runErr == nil && (len(results) != 1 || results[0].leader != seededOwner) {
				runErr = fmt.Errorf("leader lookup = %q, want %q", firstRoundLeader(results), seededOwner)
			}
		case "ExpiryTakeover":
			var incumbent leader.Elector
			incumbent, runErr = fixture.newElector("member-old", group)
			var oldOwner string
			if runErr == nil {
				oldOwner, runErr = seedProviderOwner(fixture, group, incumbent)
			}
			if runErr == nil {
				runErr = fixture.abort(context.Background(), incumbent)
			}
			var contender leader.Elector
			if runErr == nil {
				contender, runErr = fixture.newElector("member-new", group)
			}
			if runErr == nil {
				b.StartTimer()
				results, runErr = runLeaderRound(context.Background(), leaderRoundOptions{workers: 1, attemptLimit: providerOperationLimit, roundLimit: providerRoundLimit}, func(ctx context.Context, _ int) (leaderRoundResult, error) {
					if err := waitForProviderOwnerChange(ctx, fixture, group, oldOwner, providerExpiryLease+5*time.Second); err != nil {
						return leaderRoundResult{}, err
					}
					err := contender.Campaign(ctx)
					return leaderRoundResult{member: "member-new", won: err == nil}, err
				})
				b.StopTimer()
			}
			if runErr == nil {
				runErr = verifyReplacementOwner(fixture, group, oldOwner, "member-new")
			}
		default:
			runErr = fmt.Errorf("unknown leader benchmark scenario %q", scenario)
		}

		cleanupErr := runBoundedCleanup(context.Background(), func(ctx context.Context) error { return fixture.cleanup(ctx, group) })
		providerBenchmarkSink = results
		if err := errors.Join(runErr, cleanupErr); err != nil {
			b.Fatalf("%s/%s: %v", factory.name, scenario, err)
		}
	}
}

func sanitizeProviderMetadata(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
}

func countRoundWinners(results []leaderRoundResult) int {
	winners := 0
	for _, result := range results {
		if result.won {
			winners++
		}
	}
	return winners
}

func firstRoundLeader(results []leaderRoundResult) string {
	if len(results) == 0 {
		return ""
	}
	return results[0].leader
}

func verifyCampaignRound(fixture leaderProviderFixture, group string, results []leaderRoundResult, electors []leader.Elector) error {
	if winners := countRoundWinners(results); winners != 1 {
		return fmt.Errorf("campaign winners = %d, want 1", winners)
	}
	winner := ""
	winnerWorker := -1
	for _, result := range results {
		if result.won {
			winner = result.member
			winnerWorker = result.worker
			break
		}
	}
	if winnerWorker < 0 || winnerWorker >= len(electors) || electors[winnerWorker] == nil {
		return fmt.Errorf("campaign winner worker %d has no elector", winnerWorker)
	}
	if !electors[winnerWorker].IsLeader() {
		return fmt.Errorf("campaign winner Elector.IsLeader() = false")
	}
	ctx, cancel := operationContext()
	defer cancel()
	owner, err := fixture.observe(ctx, group)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(owner, winner+":") {
		return fmt.Errorf("campaign owner = %q, want member prefix %q", owner, winner+":")
	}
	return nil
}

func seedProviderOwner(fixture leaderProviderFixture, group string, elector leader.Elector) (string, error) {
	ctx, cancel := operationContext()
	defer cancel()
	if err := elector.Campaign(ctx); err != nil {
		return "", err
	}
	owner, err := fixture.observe(ctx, group)
	if err != nil {
		return "", err
	}
	if owner == "" {
		return "", errors.New("seeded provider owner is empty")
	}
	return owner, nil
}

func verifyOwnerAbsent(fixture leaderProviderFixture, group, previous string) error {
	ctx, cancel := operationContext()
	defer cancel()
	owner, err := fixture.observe(ctx, group)
	if err != nil {
		return err
	}
	if owner != "" {
		return fmt.Errorf("owner after resign = %q, want absence (previous %q)", owner, previous)
	}
	return nil
}

func verifyReplacementOwner(fixture leaderProviderFixture, group, previous, member string) error {
	ctx, cancel := operationContext()
	defer cancel()
	owner, err := fixture.observe(ctx, group)
	if err != nil {
		return err
	}
	if owner == previous || !strings.HasPrefix(owner, member+":") {
		return fmt.Errorf("replacement owner = %q, previous %q, want member prefix %q", owner, previous, member+":")
	}
	return nil
}

func waitForProviderOwnerChange(ctx context.Context, fixture leaderProviderFixture, group, previous string, limit time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	ticker := time.NewTicker(providerExpiryPoll)
	defer ticker.Stop()
	for {
		owner, err := fixture.observe(waitCtx, group)
		if err != nil {
			return err
		}
		if owner != previous {
			return nil
		}
		select {
		case <-waitCtx.Done():
			return waitCtx.Err()
		case <-ticker.C:
		}
	}
}

func TestProviderLeaderBenchmarkProbes(t *testing.T) {
	if os.Getenv(providerBenchmarkEnv) != "1" {
		t.Skipf("set %s=1 to run provider benchmark probes", providerBenchmarkEnv)
	}
	for _, factory := range leaderProviderFactories {
		t.Run(factory.name, func(t *testing.T) {
			fixture := factory.open(t, providerExpiryLease)
			t.Run("ActiveHolderCancellation", func(t *testing.T) {
				testActiveHolderCancellation(t, fixture)
			})
			t.Run("RenewalPersistence", func(t *testing.T) {
				testRenewalPersistence(t, fixture)
			})
			t.Run("CancellationCleanup", func(t *testing.T) {
				testCancellationCleanup(t, fixture)
			})
			t.Run("StaleOwnerRejected", func(t *testing.T) {
				testStaleOwnerRejected(t, fixture)
			})
		})
	}
}

func testActiveHolderCancellation(t *testing.T, fixture leaderProviderFixture) {
	t.Helper()
	group := mustProviderID(t)
	holder, err := fixture.newElector("member-active", group)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := seedProviderOwner(fixture, group, holder)
	if err != nil {
		t.Fatalf("seed active holder: %v", err)
	}
	contender, err := fixture.newElector("member-blocked", group)
	if err != nil {
		t.Fatal(err)
	}
	campaignCtx, cancelCampaign := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancelCampaign()
	done := make(chan error, 1)
	ready := make(chan struct{})
	var active atomic.Int32
	active.Add(1)
	go func() {
		defer active.Add(-1)
		close(ready)
		done <- contender.Campaign(campaignCtx)
	}()
	<-ready
	time.Sleep(2 * providerExpiryPoll)
	select {
	case campaignErr := <-done:
		t.Fatalf("contender completed before deadline expiry: %v", campaignErr)
	default:
	}
	select {
	case campaignErr := <-done:
		if !errors.Is(campaignErr, context.DeadlineExceeded) {
			t.Fatalf("blocked contender error = %v, want deadline exceeded", campaignErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked contender did not drain within 2s")
	}
	if got := active.Load(); got != 0 {
		t.Fatalf("active contenders after cancellation = %d, want 0", got)
	}
	if !holder.IsLeader() {
		t.Fatal("original holder lost local leadership after contender cancellation")
	}
	if current := observeProviderOwner(t, fixture, group); current != owner {
		t.Fatalf("owner after blocked contender cancellation = %q, want preserved %q", current, owner)
	}
	cleanupProviderProbe(t, fixture, group)
}

func testRenewalPersistence(t *testing.T, fixture leaderProviderFixture) {
	t.Helper()
	group := mustProviderID(t)
	elector, err := fixture.newElector("member-renew", group)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := seedProviderOwner(fixture, group, elector)
	if err != nil {
		t.Fatalf("seed renewal holder: %v", err)
	}
	time.Sleep(providerExpiryLease + 500*time.Millisecond)
	if current := observeProviderOwner(t, fixture, group); current != owner {
		t.Fatalf("owner after renewal window = %q, want %q", current, owner)
	}
	cleanupProviderProbe(t, fixture, group)
}

func testCancellationCleanup(t *testing.T, fixture leaderProviderFixture) {
	t.Helper()
	group := mustProviderID(t)
	holder, err := fixture.newElector("member-holder", group)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := seedProviderOwner(fixture, group, holder)
	if err != nil {
		t.Fatalf("seed cancellation holder: %v", err)
	}
	contender, err := fixture.newElector("member-blocked", group)
	if err != nil {
		t.Fatal(err)
	}
	var active atomic.Int32
	done := make(chan error, 1)
	entered := make(chan struct{})
	baseCtx, cancelCampaign := context.WithCancel(context.Background())
	defer cancelCampaign()
	campaignCtx := &providerCampaignEntryContext{Context: baseCtx, entered: entered}
	active.Add(1)
	go func() {
		defer active.Add(-1)
		done <- contender.Campaign(campaignCtx)
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked campaign did not enter Campaign within 2s")
	}
	cancelCampaign()
	select {
	case campaignErr := <-done:
		if !errors.Is(campaignErr, context.Canceled) {
			t.Fatalf("blocked campaign error = %v, want context cancellation", campaignErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked campaign did not drain within 2s")
	}
	if got := active.Load(); got != 0 {
		t.Fatalf("active blocked campaigns after cancellation = %d, want 0", got)
	}
	if current := observeProviderOwner(t, fixture, group); current != owner {
		t.Fatalf("owner after contender cancellation = %q, want preserved %q", current, owner)
	}
	cleanupProviderProbe(t, fixture, group)
}

func testStaleOwnerRejected(t *testing.T, fixture leaderProviderFixture) {
	t.Helper()
	group := mustProviderID(t)
	stale, err := fixture.newElector("member-stale", group)
	if err != nil {
		t.Fatal(err)
	}
	staleOwner, err := seedProviderOwner(fixture, group, stale)
	if err != nil {
		t.Fatalf("seed stale owner: %v", err)
	}
	replacementOwner := "member-replacement:" + mustProviderID(t)
	if replacementOwner == staleOwner {
		t.Fatalf("replacement owner reused stale token %q", staleOwner)
	}
	replaceCtx, replaceCancel := operationContext()
	if err := fixture.replace(replaceCtx, group, staleOwner, replacementOwner); err != nil {
		replaceCancel()
		t.Fatalf("force replacement owner: %v", err)
	}
	replaceCancel()
	releaseCtx, releaseCancel := operationContext()
	rejected, releaseErr := fixture.rejectStaleRelease(releaseCtx, group, staleOwner)
	releaseCancel()
	if releaseErr != nil {
		t.Fatalf("execute stale owner release against %s backend: %v", fixture.name, releaseErr)
	}
	if !rejected {
		t.Fatalf("%s backend accepted stale owner release", fixture.name)
	}
	if current := observeProviderOwner(t, fixture, group); current != replacementOwner {
		t.Fatalf("owner after direct stale release = %q, want replacement %q", current, replacementOwner)
	}
	resignCtx, resignCancel := context.WithTimeout(context.Background(), 2*time.Second)
	resignErr := stale.Resign(resignCtx)
	resignCancel()
	if resignErr != nil {
		t.Fatalf("stale owner resign: %v", resignErr)
	}
	if current := observeProviderOwner(t, fixture, group); current != replacementOwner {
		t.Fatalf("owner after stale resign = %q, want replacement %q", current, replacementOwner)
	}
	cleanupProviderProbe(t, fixture, group)
}

func observeProviderOwner(t *testing.T, fixture leaderProviderFixture, group string) string {
	t.Helper()
	ctx, cancel := operationContext()
	defer cancel()
	owner, err := fixture.observe(ctx, group)
	if err != nil {
		t.Fatalf("observe %s owner: %v", fixture.name, err)
	}
	return owner
}

func cleanupProviderProbe(t *testing.T, fixture leaderProviderFixture, group string) {
	t.Helper()
	if err := runBoundedCleanup(context.Background(), func(ctx context.Context) error { return fixture.cleanup(ctx, group) }); err != nil {
		t.Fatalf("cleanup %s probe group: %v", fixture.name, err)
	}
	if owner := observeProviderOwner(t, fixture, group); owner != "" {
		t.Fatalf("backend owner after cleanup = %q, want absence", owner)
	}
}

type localLeadershipElector struct {
	leader bool
}

func (elector localLeadershipElector) Campaign(context.Context) error { return nil }
func (elector localLeadershipElector) Resign(context.Context) error   { return nil }
func (elector localLeadershipElector) IsLeader() bool                 { return elector.leader }
func (elector localLeadershipElector) Leader(context.Context) (string, error) {
	return "member-0:token", nil
}

func TestProviderEtcdCleanupPrefixScopesGeneratedNamespace(t *testing.T) {
	if got, want := providerEtcdCleanupPrefix("0123", ""), "/bluetape4k/leader/MDEyMw/"; got != want {
		t.Fatalf("namespace cleanup prefix = %q, want %q", got, want)
	}
	if got, want := providerEtcdCleanupPrefix("0123", "group-a"), "/bluetape4k/leader/MDEyMw/Z3JvdXAtYQ/"; got != want {
		t.Fatalf("group cleanup prefix = %q, want %q", got, want)
	}
}

func TestVerifyCampaignRoundRequiresLocalLeadership(t *testing.T) {
	fixture := leaderProviderFixture{
		observe: func(context.Context, string) (string, error) { return "member-0:token", nil },
	}
	results := []leaderRoundResult{{worker: 0, member: "member-0", won: true}}
	err := verifyCampaignRound(fixture, "group", results, []leader.Elector{localLeadershipElector{leader: false}})
	if err == nil || !strings.Contains(err.Error(), "IsLeader") {
		t.Fatalf("verify campaign round error = %v, want IsLeader failure", err)
	}
}

func TestRunLeaderRoundJoinsAllWorkers(t *testing.T) {
	const workers = 8
	var started atomic.Int32
	var active atomic.Int32

	results, err := runLeaderRound(context.Background(), leaderRoundOptions{
		workers:      workers,
		attemptLimit: time.Second,
		roundLimit:   2 * time.Second,
	}, func(context.Context, int) (leaderRoundResult, error) {
		started.Add(1)
		active.Add(1)
		defer active.Add(-1)
		return leaderRoundResult{}, nil
	})
	if err != nil {
		t.Fatalf("run leader round: %v", err)
	}
	if got := len(results); got != workers {
		t.Fatalf("results = %d, want %d", got, workers)
	}
	if got := started.Load(); got != workers {
		t.Fatalf("started workers = %d, want %d", got, workers)
	}
	if got := active.Load(); got != 0 {
		t.Fatalf("active workers after return = %d, want 0", got)
	}
}

func TestRunLeaderRoundStartsAllWorkersBeforeWinnerCancellation(t *testing.T) {
	const (
		workers    = 8
		iterations = 100
	)
	for iteration := range iterations {
		var winner atomic.Bool
		var started atomic.Int32
		results, err := runLeaderRound(context.Background(), leaderRoundOptions{
			workers:      workers,
			attemptLimit: time.Second,
			roundLimit:   2 * time.Second,
		}, func(ctx context.Context, _ int) (leaderRoundResult, error) {
			started.Add(1)
			if winner.CompareAndSwap(false, true) {
				return leaderRoundResult{won: true}, nil
			}
			<-ctx.Done()
			return leaderRoundResult{}, ctx.Err()
		})
		if err != nil {
			t.Fatalf("iteration %d: run leader round: %v", iteration, err)
		}
		if got := len(results); got != workers {
			t.Fatalf("iteration %d: results = %d, want %d", iteration, got, workers)
		}
		if got := started.Load(); got != workers {
			t.Fatalf("iteration %d: started workers = %d, want %d", iteration, got, workers)
		}
	}
}

func TestRunLeaderRoundPreservesDeadline(t *testing.T) {
	const workers = 4
	results, err := runLeaderRound(context.Background(), leaderRoundOptions{
		workers:      workers,
		attemptLimit: time.Second,
		roundLimit:   20 * time.Millisecond,
	}, func(ctx context.Context, _ int) (leaderRoundResult, error) {
		<-ctx.Done()
		return leaderRoundResult{}, ctx.Err()
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("run leader round error = %v, want deadline exceeded", err)
	}
	if got := len(results); got != workers {
		t.Fatalf("results = %d, want %d", got, workers)
	}
}

func TestRunLeaderRoundCancelsPeersAndJoinsOnFirstError(t *testing.T) {
	const workers = 8
	injected := errors.New("injected member-0 failure")
	var active atomic.Int32

	started := time.Now()
	results, err := runLeaderRound(context.Background(), leaderRoundOptions{
		workers:      workers,
		attemptLimit: time.Second,
		roundLimit:   2 * time.Second,
	}, func(ctx context.Context, worker int) (leaderRoundResult, error) {
		active.Add(1)
		defer active.Add(-1)
		if worker == 0 {
			return leaderRoundResult{}, injected
		}
		<-ctx.Done()
		return leaderRoundResult{}, ctx.Err()
	})
	elapsed := time.Since(started)
	if !errors.Is(err, injected) {
		t.Fatalf("run leader round error = %v, want injected error", err)
	}
	if elapsed >= time.Second {
		t.Fatalf("run leader round returned after %s, want under 1s", elapsed)
	}
	if got := len(results); got != workers {
		t.Fatalf("results = %d, want %d", got, workers)
	}
	if got := active.Load(); got != 0 {
		t.Fatalf("active workers after return = %d, want 0", got)
	}
}
