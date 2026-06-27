# bluetape-go

[English](README.md) | [한국어](README.ko.md)

![bluetape-go hero](docs/assets/bluetape-go-hero.png)

Idiomatic Go backend utilities and distributed infrastructure packages for the
bluetape ecosystem.

`bluetape-go` complements the Kotlin/JVM bluetape4k libraries without trying to
mirror their API surface. It gives Go teams small, focused packages for service
infrastructure, coordination, test fixtures, resilience, caching, workflows,
batch processing, portable values, and Redis-backed adapters.

## Architecture

![bluetape-go Architecture Overview](docs/assets/bluetape-go-architecture-overview.png)

## Current Status

`bluetape-go` has published the `v0.6.7` release line. The repository now covers
foundation helpers, codecs, compression, context-aware concurrency, serializer
contracts, Redis-backed leader election and locks, resilience policies, cache
coordination, token-bucket rate limiting, finite state machines, workflow
reports, lightweight workflow runners, checkpointed batch jobs, and portable
service values.

The `v0.6.x` portable utilities scope includes UUID, ULID, KSUID, and Snowflake
ID generation; explicit-algorithm JWT signing, parsing, validation, and local
or distributed key rotation backed by in-memory, Redis, or MongoDB KeyChain
repositories; typed units and measured values; ISO currency and decimal-backed
money operations; and in-memory or Redis-backed Bloom filters. The current
`0.6.7` line includes the corrective `0.6.3` through `0.6.6` implementation
series plus MongoDB-backed JWT KeyChain storage.

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
| [`dynamodb/batchwrite`](dynamodb/batchwrite/README.md) | active | Narrow AWS SDK for Go v2 BatchWriteItem chunking and unprocessed-item retry helper. |
| [`examples/integration`](examples/integration/README.md) | example | Compile-checked end-to-end recipes across corrected `0.6.x` packages. |
| [`examples/s3`](examples/s3/README.md) | example | Compile-checked AWS SDK for Go v2 S3 examples backed by the Floci fixture. |
| [`examples/sqs-sns`](examples/sqs-sns/README.md) | example | Compile-checked AWS SDK for Go v2 SQS/SNS examples backed by the Floci fixture. |
| [`leader`](leader/README.md) | active | Leader election API, including single, group, and strategy-based contracts. |
| [`leader/redis`](leader/redis/README.md) | active | Redis-backed single, group, and strategic leader election using TTL renewal, ZSET slot tokens, and candidate registries. |
| [`resilience`](resilience/README.md) | active | First-party composable retry, timeout, circuit breaker, and bulkhead policies with synchronous observability hooks and `net/http` adapters. |
| [`cache`](cache/README.md) | active | Generic in-process TTL cache interfaces with context-aware loaders and same-key stampede protection. |
| [`cache/redisnear`](cache/redisnear/README.md) | active | Redis Pub/Sub near-cache invalidation for process-local loading caches. |
| [`cache/rediscoord`](cache/rediscoord/README.md) | active | Opt-in Redis coordination wrapper that shares one loader result across process-local caches during a cold burst. |
| [`lock/redis`](lock/redis/README.md) | active | Redis single-instance owner-token lock with TTL acquisition and owner-safe Lua unlock. |
| [`ratelimit`](ratelimit/README.md) | active | Process-local keyed token-bucket limiter and `net/http` middleware. |
| [`ratelimit/redis`](ratelimit/redis/README.md) | active | Redis-backed token-bucket limiter with atomic Lua consume/refill and idle key expiration. |
| [`state`](state/README.md) | active | Small finite state machine primitives with typed transitions, guards, final states, and sentinel errors. |
| [`workreport`](workreport/README.md) | active | Status, failure-policy, and report-tree values for lightweight workflow code. |
| [`workflow`](workflow/README.md) | active | Sequential, conditional, and all-branches parallel runners built on `context.Context` and `workreport`. |
| [`batch`](batch/README.md) | active | Chunk-oriented batch steps, sequential jobs, retry/skip policies, reports, and checkpoints. |
| [`id`](id/README.md) | active | UUID v4/v7, random and monotonic ULID, standard KSUID, Kotlin-compatible KSUID millis, and Snowflake ID generators. |
| [`jwt`](jwt/README.md) | active | JWT signing, parsing, validation, typed claim reading, in-memory/distributed `kid` key rotation, and optional provider cache adapters with explicit algorithms. |
| [`jwt/redis`](jwt/redis/README.md) | active | Redis-specific facade for distributed JWT key-chain repository construction. |
| [`jwt/mongo`](jwt/mongo/README.md) | active | MongoDB-specific facade for distributed JWT key-chain repository construction. |
| [`measure`](measure/README.md) | active | Typed units, measured values, compound units, parsing, formatting, and affine temperature helpers. |
| [`money`](money/README.md) | active | ISO 4217 currency values, CLDR-backed locale currency lookup, decimal-backed money amounts, aggregation, serialization, caller-supplied exchange-rate conversion, and ECB-backed provider conversion. |
| [`sqlkit`](sqlkit/README.md) | active | Runtime-first `database/sql` transaction helpers, explicit row mapping/cardinality helpers, and PostgreSQL-first inspectable SQL builders. |
| [`probabilistic`](probabilistic/README.md) | active | Goroutine-safe in-memory Bloom filters with deterministic config, merge compatibility checks, and stress/race coverage. |
| [`probabilistic/redis`](probabilistic/redis/README.md) | active | Redis-backed shared Bloom filters with static Lua scripts, immutable config metadata, and operator runbook boundaries. |

Next planned package families include SQL generator/migration guidance,
blockword masking, tokenizer research, audit, and graph packages. Redis-backed
Cuckoo and HyperLogLog/HLL support is tracked separately after the Redis Bloom
scope.

## Install

```bash
go get github.com/bluetape4k/bluetape-go
```

## Package Documentation

Package-level READMEs contain the practical details: usage examples, operational
boundaries, benchmark notes, and the constraints that do not belong in a root
overview.

- Foundation: [`core`](core/README.md), [`collections`](collections/README.md),
  [`concurrency`](concurrency/README.md), [`codec`](codec/README.md),
  [`compression`](compression/README.md), and
  [`serialization`](serialization/README.md).
- Test support: [`testing`](testing/README.md),
  [`testing/concurrency`](testing/concurrency/README.md), and the fixture
  package READMEs listed above. Focused examples in `testing` cover
  table-driven tests, package-local builders, golden files, deterministic random
  data, and cancellation assertions without adding an assertion DSL.
- AWS/Floci: [`dynamodb/batchwrite`](dynamodb/batchwrite/README.md) and
  compile-checked examples under [`examples/integration`](examples/integration/README.md),
  [`examples/s3`](examples/s3/README.md), and
  [`examples/sqs-sns`](examples/sqs-sns/README.md).
- Text: [`textsearch`](textsearch/README.md) for deterministic multi-pattern
  search, tokenizer core interfaces, blockword detection/masking, severity
  metadata, normalization, boundary-aware matching, and the optional
  [`textsearch/japanese`](textsearch/japanese/README.md) Kagome adapter plus
  [`textsearch/language`](textsearch/language/README.md) Lingua-Go detector.
- Coordination: [`leader`](leader/README.md),
  [`leader/redis`](leader/redis/README.md), and
  [`lock/redis`](lock/redis/README.md).
- Runtime policies, cache, state, and workflow: [`resilience`](resilience/README.md),
  [`cache`](cache/README.md), [`cache/redisnear`](cache/redisnear/README.md),
  [`cache/rediscoord`](cache/rediscoord/README.md), [`ratelimit`](ratelimit/README.md),
  [`state`](state/README.md), [`workreport`](workreport/README.md), and
  [`workflow`](workflow/README.md), and [`batch`](batch/README.md).
- Portable utilities: [`id`](id/README.md), [`jwt`](jwt/README.md),
  [`jwt/redis`](jwt/redis/README.md), [`jwt/mongo`](jwt/mongo/README.md),
  [`measure`](measure/README.md), [`money`](money/README.md), and
  [`probabilistic`](probabilistic/README.md), including
  [`probabilistic/redis`](probabilistic/redis/README.md).
- Data access: [`sqlkit`](sqlkit/README.md) and the optional
  [SQL generator/migration guide](docs/sql-generator-migration-guidance.md).

## Roadmap

| Milestone | Theme |
|---|---|
| `0.1.0` | Core support, collections, goroutine helpers, codecs, compression, Redis leader election, Testcontainers. |
| `0.2.0` | Resilience primitives: retry, timeout, circuit breaker, bulkhead, HTTP middleware. |
| `0.3.0` | Cache and coordination: near cache, Redis locks, token-bucket rate limiting, strategic leader election. |
| `0.4.0` | State machine and lightweight workflow primitives. |
| `0.5.0` | Batch processing with checkpoints and leader-guarded examples. |
| `0.6.0` | Portable utilities: ID generation, JWT, measured values, money, probabilistic structures. |
| `0.6.1` | Portable utility hardening: Redis Bloom filters, provider caches, exchange-rate providers, locale currency mapping, and compatibility evidence. |
| `0.6.2` | Corrective source-parity matrix and hardening plan for core, testing, and Testcontainers. |
| `0.6.3` | Core foundation parity and hardening. |
| `0.6.4` | JUnit5-inspired Go testing helper parity. |
| `0.6.5` | Testcontainers contract hardening and service coverage expansion. |
| `0.6.6` | Developer-experience parity, integration examples, and corrective-series closure. |
| `0.7.0` | Relational SQL DSL and repository helpers. |
| `0.8.0` | Text search, blockword masking, tokenizer adapters. |
| `0.9.0` | Audit and event packages inspired by bluetape4k-javers patterns. |
| `0.10.0` | Graph packages and examples. |
| `0.11.0` | Image, encryption, and utility follow-ups. |

The closed `0.7.0 Research Gate` milestone recorded the larger-domain scope
decisions and was not tagged as a release.

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
| `make test` | Run `go test -p 1 -count=1 ./...` so Testcontainers tests execute with serial package scheduling. |
| `make race` | Run `go test -race -p 1 -count=1 ./...` so Testcontainers tests execute under the race detector with serial package scheduling. |
| `make coverage` | Generate Go coverage profile, package subtotal table, text summary, and HTML report under `coverage/`. |
| `make bench-cache` | Run opt-in cache, Redis NearCache, and Redis coordinator benchmarks. |
| `make bench-ratelimit` | Run opt-in local rate limiter benchmarks. |
| `make bench-id` | Run opt-in id generator benchmarks. |
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
