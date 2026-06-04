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
| [`core`](core/README.md) | active | Small shared validation, zero/default, pointer, string, and number helpers. |
| [`collections`](collections/README.md) | active | Focused generic slice/map helpers for chunking, grouping, distinct, and error-aware transforms. |
| [`concurrency`](concurrency/README.md) | active | Context-aware goroutine groups, worker pools, and bounded parallel helpers. |
| [`codec`](codec/README.md) | active | Base58, Base62, Base64, hex, and URL-safe encoding helpers. |
| [`compression`](compression/README.md) | active | gzip, deflate, zstd, lz4, snappy, and registry-backed compression helpers. |
| [`serialization`](serialization/README.md) | active | JSON and binary serializer interfaces with safe defaults. |
| [`testing`](testing/README.md) | active | Common test helpers for eventual consistency checks. |
| [`testing/concurrency`](testing/concurrency/README.md) | active | Stress and async job helpers for concurrent tests. |
| [`testcontainers/redis`](testcontainers/redis/README.md) | active | Redis fixture helpers based on Testcontainers for Go. |
| [`testcontainers/postgres`](testcontainers/postgres/README.md) | active | PostgreSQL fixture helpers based on Testcontainers for Go. |
| [`testcontainers/mysql`](testcontainers/mysql/README.md) | active | MySQL 8.4 fixture helpers based on Testcontainers for Go. |
| [`testcontainers/nats`](testcontainers/nats/README.md) | active | NATS fixture helpers based on Testcontainers for Go. |
| [`testcontainers/kafka`](testcontainers/kafka/README.md) | active | Kafka fixture helpers based on Testcontainers for Go. |
| [`leader`](leader/README.md) | active | Leader election API. |
| [`leader/redis`](leader/redis/README.md) | active | Redis-backed single and group leader election using TTL renewal and ZSET slot tokens. |
| [`resilience`](resilience/README.md) | active | First-party composable retry, timeout, circuit breaker, and bulkhead policies with synchronous observability hooks and `net/http` adapters. |
| [`cache`](cache/README.md) | active | Generic in-process TTL cache interfaces with context-aware loaders and same-key stampede protection. |
| [`cache/redisnear`](cache/redisnear/README.md) | active | Redis Pub/Sub near-cache invalidation for process-local loading caches. |
| [`cache/rediscoord`](cache/rediscoord/README.md) | active | Opt-in Redis coordination wrapper that shares one loader result across process-local caches during a cold burst. |
| [`lock/redis`](lock/redis/README.md) | active | Redis single-instance owner-token lock with TTL acquisition and owner-safe Lua unlock. |

Next planned package families include `workflow`, `batch`, `id`, `jwt`,
`graph`, `text`, `audit`, and AWS helper/example packages.

## Install

```bash
go get github.com/bluetape4k/bluetape-go
```

## Package Documentation

Detailed usage examples, operational boundaries, and package-specific
benchmarks live next to each package:

- Foundation: [`core`](core/README.md), [`collections`](collections/README.md),
  [`concurrency`](concurrency/README.md), [`codec`](codec/README.md),
  [`compression`](compression/README.md), and
  [`serialization`](serialization/README.md).
- Test support: [`testing`](testing/README.md),
  [`testing/concurrency`](testing/concurrency/README.md), and the fixture
  package READMEs listed above.
- Coordination: [`leader`](leader/README.md),
  [`leader/redis`](leader/redis/README.md), and
  [`lock/redis`](lock/redis/README.md).
- Runtime policies and cache: [`resilience`](resilience/README.md),
  [`cache`](cache/README.md), [`cache/redisnear`](cache/redisnear/README.md),
  and [`cache/rediscoord`](cache/rediscoord/README.md).

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
make coverage
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
| `make coverage` | Generate Go coverage profile, package subtotal table, text summary, and HTML report under `coverage/`. |
| `make bench-cache` | Run opt-in cache, Redis NearCache, and Redis coordinator benchmarks. |
| `make ci` | Run the local CI gate. |

Redis integration tests use Testcontainers and require Docker. The regular CI
and Nightly workflows both run these tests against real containers and publish
Go coverage report artifacts.

See [`testing`](testing/README.md), [`testing/concurrency`](testing/concurrency/README.md),
and the package-level Testcontainers README files for fixture usage.

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
