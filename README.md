# bluetape-go

[English](README.md) | [한국어](README.ko.md)

![bluetape-go hero](docs/assets/bluetape-go-hero.png)

Go backend utilities and distributed infrastructure packages for the bluetape
ecosystem.

`bluetape-go` complements the Kotlin/JVM bluetape4k libraries. It is not a
rewrite of bluetape4k. It provides idiomatic Go packages for teams that prefer
Go for backend infrastructure, service coordination, test fixtures, resilience,
caching, workflow, batch, graph, text, audit, and AWS-adjacent service code.

## Architecture

![bluetape-go Architecture Overview](docs/assets/bluetape-go-architecture-overview.png)

## Current Status

`bluetape-go` is on the `0.3.0` development line. The repository already
contains foundation utilities, codecs, compression, concurrency helpers,
serialization contracts, Redis-backed leader election, resilience policies, and
the cache and Redis coordination packages. Remaining `0.3.0` work focuses on
token-bucket rate limiting and pluggable leader election strategies.

## Packages

| Package | Status | Purpose |
|---|---:|---|
| `core` | active | Small shared validation, zero/default, pointer, string, and number helpers. |
| `collections` | active | Focused generic slice/map helpers for chunking, grouping, distinct, and error-aware transforms. |
| `concurrency` | active | Context-aware goroutine groups, worker pools, and bounded parallel helpers. |
| `codec` | active | Base58, Base62, Base64, hex, and URL-safe encoding helpers. |
| `compression` | active | gzip, deflate, zstd, lz4, snappy, and registry-backed compression helpers. |
| `serialization` | active | JSON and binary serializer interfaces with safe defaults. |
| `testing` | active | Common test helpers for eventual consistency checks. |
| `testcontainers/redis` | active | Redis fixture helpers based on Testcontainers for Go. |
| `testcontainers/postgres` | active | PostgreSQL fixture helpers based on Testcontainers for Go. |
| `testcontainers/mysql` | active | MySQL 8.4 fixture helpers based on Testcontainers for Go. |
| `testcontainers/nats` | active | NATS fixture helpers based on Testcontainers for Go. |
| `testcontainers/kafka` | active | Kafka fixture helpers based on Testcontainers for Go. |
| `leader` | active | Leader election API. |
| `leader/redis` | active | Redis-backed single and group leader election using TTL renewal and ZSET slot tokens. |
| `resilience` | active | First-party composable retry, timeout, circuit breaker, and bulkhead policies with synchronous observability hooks and `net/http` adapters for service calls. |
| `cache` | active | Generic in-process TTL cache interfaces with context-aware loaders and same-key stampede protection. |
| `cache/redisnear` | active | Redis Pub/Sub near-cache invalidation for process-local loading caches. |
| `cache/rediscoord` | active | Opt-in Redis coordination wrapper that shares one loader result across process-local caches during a cold burst. |
| `lock/redis` | active | Redis single-instance owner-token lock with TTL acquisition and owner-safe Lua unlock. |

Next planned package families include `workflow`, `batch`, `id`, `jwt`,
`graph`, `text`, `audit`, and AWS helper/example packages.

## Install

```bash
go get github.com/bluetape4k/bluetape-go
```

## Leader Election

```go
client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

elector, err := redisleader.New(client, leader.Options{
    Group:    "billing-workers",
    MemberID: "worker-1",
})
if err != nil {
    return err
}

if err := elector.Campaign(ctx); err != nil {
    return err
}
defer elector.Resign(context.Background())
```

The Kotlin/JVM `bluetape4k-leader` repository remains supported separately.
Mixed Kotlin/Go Redis leader participants are not supported in `0.1.0`. The Go
Redis backend owns its own key format: `bluetape:leader:<group>` stores a
`memberID:random` token with a Redis TTL. Kotlin/JVM `bluetape4k-leader`
Lettuce uses the lock name directly with a generated token value, and Redisson
uses Redisson `RLock` internals. Keep Kotlin and Go leader groups separate
unless a future explicit interoperability adapter is added.

Redis leader examples cover backend coordination problems that must run on only
one replica at a time:

| Example | Problem | Smoke test |
|---|---|---|
| Batch scheduler | Prevents every scheduler replica from running the same nightly job. | `go test -count=1 ./leader/redis -run TestBatchSchedulerExample` |
| Migration gate | Allows only one service instance to apply a rollout migration. | `go test -count=1 ./leader/redis -run TestMigrationGateExample` |

Use `NewGroup` when a bounded number of replicas may run the same worker lane at
the same time:

```go
group, err := redisleader.NewGroup(client, leader.GroupOptions{
    Options: leader.Options{
        Group:    "batch-workers",
        MemberID: "worker-1",
    },
    MaxLeaders: 3,
})
if err != nil {
    return err
}

if err := group.Campaign(ctx); err != nil {
    return err
}
defer group.Resign(context.Background())
```

The Redis group backend stores live slots in
`bluetape:leader-group:<group>` as ZSET members. Each member is a
`memberID:random` token with an expiry score based on Redis server time. Expired
slots are pruned during acquire and status checks, so leaked slots from crashed
processes are reclaimed without a separate reaper.

## Resilience Policies

Resilience policies provide first-party retry, timeout, circuit breaker, and
bulkhead primitives that can be composed around a service call. Each policy
accepts an `OnEvent` hook. The hook is called synchronously on the protected
call path with a structured `resilience.Event` payload containing stable policy
type, event kind, event category, attempt/state data, and low-cardinality error
category labels.

Use `OnEvent` to bridge policy decisions to the logging, metrics, or tracing
tooling already used by the service:

```go
retry, err := resilience.NewRetry[string](resilience.RetryOptions{
    Name:        "catalog",
    MaxAttempts: 3,
    Backoff:     resilience.ConstantBackoff(50 * time.Millisecond),
    OnEvent: func(ctx context.Context, event resilience.Event) {
        logger.InfoContext(ctx, "resilience event",
            "policy", event.PolicyName,
            "type", event.PolicyType,
            "kind", event.Kind,
            "category", event.Category,
            "error_category", event.ErrorCategory,
            "attempt", event.Attempt,
        )
    },
})
```

The package intentionally does not include a built-in OpenTelemetry exporter.
Keep event handlers fast and non-blocking because a slow handler delays the
protected call.

HTTP clients can use the same policies through a `net/http` transport adapter.
The adapter can turn retryable response statuses into `StatusError`, closes the
retryable response body before the next attempt, and keeps observability on the
same `OnEvent` hook contract:

```go
retry, err := resilience.NewRetry[*http.Response](resilience.RetryOptions{
    Name:        "catalog-http",
    MaxAttempts: 3,
    Backoff:     resilience.ConstantBackoff(50 * time.Millisecond),
    OnEvent:     onResilienceEvent,
})
if err != nil {
    return err
}
timeout, err := resilience.NewTimeout[*http.Response](resilience.TimeoutOptions{
    Name:    "catalog-http",
    Timeout: 500 * time.Millisecond,
    OnEvent: onResilienceEvent,
})
if err != nil {
    return err
}
breaker, err := resilience.NewCircuitBreaker[*http.Response](resilience.CircuitBreakerOptions{
    Name:             "catalog-http",
    FailureThreshold: 5,
    OpenTimeout:      30 * time.Second,
    OnEvent:          onResilienceEvent,
})
if err != nil {
    return err
}

client := http.Client{
    Transport: resilience.NewRoundTripper(resilience.RoundTripperOptions{
        Transport:       http.DefaultTransport,
        Policies:        []resilience.Policy[*http.Response]{retry, timeout, breaker},
        RetryableStatus: resilience.RetryableServerError,
    }),
}
```

Server handlers can be protected with admission or timeout policies through
`NewHandler`. Avoid retrying a server handler after it has written a response;
prefer retry on outbound client calls where the request body is replayable.

## Cache

The `cache` package provides a small, framework-neutral cache contract and a
process-local memory implementation. `ErrCacheMiss` identifies absent or expired
entries, TTL `0` stores values without expiration, and `GetOrLoad` collapses
same-key in-flight loader calls inside one cache instance.

```go
localCache := cache.NewMemory[string, string]()

value, err := localCache.GetOrLoad(ctx, "catalog", time.Minute,
    func(ctx context.Context, key string) (string, error) {
        return loadCatalogValue(ctx, key)
    },
)
if err != nil {
    return err
}
fmt.Println(value)
```

`cache/redisnear` adds Redis Pub/Sub invalidation around a process-local
`cache.LoadingCache[string,V]`. Redis is used only as the invalidation bus; each
process still owns its local values.

```go
client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

near, err := redisnear.NewPubSub[string](ctx, redisnear.Options[string]{
    Client:    client,
    Namespace: "catalog",
})
if err != nil {
    return err
}
defer func() {
    _ = near.Close()
}()
```

`Set`, `Delete`, and `Clear` mutate the local cache and publish peer
invalidation. Peer caches delete affected entries, then refill through their
own loader on the next miss. `GetOrLoad` fills only the local cache and does not
publish invalidation. If the subscriber sees a receive error before close, it
clears the local cache and reports the error through `Options.OnError`.
`OnError` is a best-effort diagnostic hook: it is delivered asynchronously
through a bounded internal buffer, and handler panics are recovered so the
subscriber keeps processing invalidations.

A receive failure caused by a Redis outage clears local entries, but automatic
resubscribe is still a best-effort behavior of the active Redis connection. For
terminal subscriber failures or Redis restarts, close the current `NearCache`
and create a new one with a fresh Redis client before relying on peer
invalidation again.

Local mutation is not rolled back when the Redis publish fails. Treat the
returned publish error as an operational signal that peers may keep stale local
entries until their TTL expires or another invalidation arrives. Redis channel
isolation is also part of the deployment contract: use Redis ACL/TLS and
namespace/channel separation because Pub/Sub messages are invalidation commands,
not an authentication boundary.

`Delete` and `Clear` are safe for concurrent callers, but they do not cancel an
already running loader. A successful in-flight loader may repopulate the cache
after a delete or clear according to normal cache-aside ordering.

`cache/rediscoord` adds opt-in cross-process stampede protection for cold miss
bursts. It wraps any `cache.LoadingCache[string,V]`, including
`redisnear.NearCache`, uses a Redis owner-token load lease, and stores a
short-lived encoded result envelope so waiters can populate their local cache
without running their own loader.

```go
coordinated, err := rediscoord.NewStampedeCache[string](rediscoord.Options[string]{
    Client:    client,
    Cache:     near,
    Namespace: "catalog",
    Codec:     rediscoord.JSONCodec[string]{},
})
if err != nil {
    return err
}

value, err := coordinated.GetOrLoad(ctx, "sku:42", time.Minute,
    func(ctx context.Context, key string) (string, error) {
        return loadCatalogValue(ctx, key)
    },
)
```

The result envelope is a transient coordination artifact, not a durable Redis
cache value. Redis can see the encoded payload, so deploy it with ACL/TLS and
namespace isolation when payloads are sensitive. If the winning loader exceeds
the configured lock TTL, another process may acquire the load lease and run a
loader; set `LockTTL` for the expected loader duration. Coordinator benchmarks
remain opt-in and tracked with the cache benchmark suite rather than `make ci`.

## Redis Distributed Lock

`lock/redis` provides a small single-Redis-instance lock for coordination tasks
that need owner tokens and automatic TTL cleanup. `TryLock` performs one
non-blocking acquire attempt with `SET NX` plus TTL. `Lease.Unlock` removes the
key only when the stored token still matches the lease token.

```go
client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

mutex, err := redislock.New(client, redislock.Options{
    Key: "locks:billing-rollup",
    TTL: 30 * time.Second,
})
if err != nil {
    return err
}

lease, err := mutex.TryLock(ctx)
if errors.Is(err, redislock.ErrNotAcquired) {
    return nil
}
if err != nil {
    return err
}
defer lease.Unlock(context.Background())
```

This package intentionally does not implement Redlock quorum, fencing tokens,
TTL renewal, or blocking retry loops. Compose retries at the call site when the
caller can define backoff, cancellation, and observability policy.

## Roadmap

| Milestone | Theme |
|---|---|
| `0.1.0` | Core support, collections, goroutine helpers, codecs, compression, Redis leader election, Testcontainers. |
| `0.2.0` | Resilience primitives: retry, timeout, circuit breaker, bulkhead, HTTP middleware. |
| `0.3.0` | Cache and coordination: near cache, Redis locks, token-bucket rate limiting. |
| `0.4.0` | State machine and lightweight workflow primitives. |
| `0.5.0` | Batch processing with checkpoints and leader-guarded examples. |
| `0.6.0` | Portable utilities: ID generation, JWT, measured values, money, probabilistic structures, rule engine. |
| `0.7.0` | Research gate for larger domains. |
| `0.8.0` | Graph packages and examples. |
| `0.9.0` | Text search, blockword masking, tokenizer research. |
| `0.10.0` | Audit and event packages inspired by bluetape4k-javers patterns. |
| `0.11.0` | AWS helper packages and LocalStack examples. |

See the [GitHub milestones](https://github.com/bluetape4k/bluetape-go/milestones)
and [`docs/research`](docs/research/) for the current planning record.

## Development

```bash
make test
make ci
```

Common commands:

| Command | Purpose |
|---|---|
| `make fmt` | Format Go sources with `gofmt`. |
| `make fmt-check` | Fail when Go sources are not formatted. |
| `make tidy` | Run `go mod tidy`. |
| `make tidy-check` | Fail when `go.mod` or `go.sum` drift after `go mod tidy`. |
| `make vet` | Run `go vet ./...`. |
| `make lint` | Run `golangci-lint run ./...`. |
| `make test` | Run `go test -count=1 ./...` so Testcontainers tests execute. |
| `make race` | Run `go test -race -count=1 ./...` so Testcontainers tests execute under the race detector. |
| `make bench-cache` | Run opt-in cache, Redis NearCache, and Redis coordinator benchmarks. |
| `make ci` | Run the local CI gate. |

Redis integration tests use Testcontainers and require Docker. The regular CI
and Nightly workflows both run these tests against real containers.

Async test assertions use Gomega-backed helpers from `testing`:

```go
bttesting.Eventually(t, time.Second, func() bool {
    return elector.IsLeader()
})

bttesting.Consistently(t, 200*time.Millisecond, elector.IsLeader)
```

Testcontainers fixtures expose small `Start(ctx, t)` helpers that register
cleanup with `t.Cleanup` and return service connection details:

```go
redisAddr := redistestcontainer.Start(ctx, t)
postgresURL := postgrestestcontainer.Start(ctx, t)
mysqlDSN := mysqltestcontainer.Start(ctx, t)
natsURL := natstestcontainer.Start(ctx, t)
kafkaBrokers := kafkatestcontainer.Start(ctx, t)
```

## Project Management

- [Changelog](CHANGELOG.md)
- [Current WIP](WIP.md)
- [Research index](docs/research/README.md)
- [Package layout policy](docs/package-layout.md)
- [Release guide](docs/release.md)

## Project Rules

- Keep APIs idiomatic to Go. Do not mechanically copy Kotlin extension-style
  APIs.
- Prefer small packages with clear service value over catch-all utility bags.
- Use proven Go dependencies where they reduce risk, but avoid wrapping mature
  SDKs without a bluetape-specific reason.
- Add Testcontainers-backed smoke tests for infrastructure packages.
