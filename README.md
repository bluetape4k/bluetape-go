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

`bluetape-go` is in early `0.1.0` development. The repository already contains
the first foundation packages and Redis-backed leader election, and the rest of
the roadmap is tracked through milestones and research notes.

## Packages

| Package | Status | Purpose |
|---|---:|---|
| `core` | active | Small shared validation, zero/default, pointer, string, and number helpers. |
| `collections` | active | Focused generic slice/map helpers for chunking, grouping, distinct, and error-aware transforms. |
| `testing` | initial | Common test helpers for eventual consistency checks. |
| `testcontainers/redis` | initial | Redis fixture helpers based on Testcontainers for Go. |
| `testcontainers/postgres` | initial | PostgreSQL fixture helpers based on Testcontainers for Go. |
| `testcontainers/mysql` | initial | MySQL 8.4 fixture helpers based on Testcontainers for Go. |
| `testcontainers/nats` | initial | NATS fixture helpers based on Testcontainers for Go. |
| `testcontainers/kafka` | initial | Kafka fixture helpers based on Testcontainers for Go. |
| `leader` | initial | Leader election API. |
| `leader/redis` | initial | Redis-backed leader election using `SET NX PX` and TTL renewal. |

Planned package families include `collections`, `concurrency`, `serialization`,
`resilience`, `cache`, `workflow`, `batch`, `id`, `jwt`, `graph`, `text`,
`audit`, and AWS helper/example packages.

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
| `make ci` | Run the local CI gate. |

Redis integration tests use Testcontainers and require Docker. The regular CI
and Nightly workflows both run these tests against real containers.

Async test assertions use Gomega-backed helpers from `testing`:

```go
bttesting.Eventually(t, time.Second, func() bool {
    return cache.IsReady()
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
