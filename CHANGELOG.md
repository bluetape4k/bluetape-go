# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses semantic versioning once the first tag is published.

## [Unreleased]

### Added

- `state` package with first-party finite state machine primitives, explicit
  transitions, context-aware guards, final states, deterministic transition
  errors, stress/cancellation coverage, and compile-checked examples.
- `workreport` package with workflow status values, failure policies, report
  trees, deterministic aggregation, zero-value safety checks,
  stress/cancellation coverage, and compile-checked examples.
- `workflow` package with sequential, conditional, and all-branches parallel
  runners built on `context.Context` and `workreport`, including cancellation,
  stress, race, and compile-checked example coverage.
- 0.4.0 stress/cancellation gate documenting required race-compatible coverage
  for `state`, `workreport`, and `workflow`.
- Package README coverage and root README indexes for the 0.4.0 `state`,
  `workreport`, and `workflow` package surface.
- Package README links to compile-checked runnable examples for the 0.4.0
  `state`, `workreport`, and `workflow` APIs.

### Changed

- Every package-level `README.md` now has a sibling `README.ko.md` and a
  consistent `English | 한국어` language switch.

## [v0.3.0] - 2026-06-05

### Added

- `cache` package with generic cache interfaces, process-local TTL `Memory`,
  `ErrCacheMiss`, context-aware loaders, and `GetOrLoad` same-key stampede
  protection.
- `cache/redisnear` package with Redis Pub/Sub invalidation for process-local
  loading caches, including close semantics, malformed-message reporting,
  Testcontainers peer invalidation coverage, stress testing, and cancellation
  coverage.
- `lock/redis` package with single-Redis-instance owner-token locking, TTL
  acquisition, owner-safe Lua unlock, Testcontainers contention/expiration
  coverage, and stress/cancellation tests.
- `cache/rediscoord` package with opt-in Redis coordination for cross-process
  cache stampede protection, owner-token load leases, short-lived shared result
  envelopes, Testcontainers NearCache collapse coverage, lease-expiry tests, and
  stress/cancellation tests.
- `ratelimit` and `ratelimit/redis` packages with local and Redis-backed
  token-bucket limiting, HTTP middleware, Redis Lua atomic consume/refill,
  Testcontainers concurrency coverage, stress/cancellation tests, and local
  benchmark coverage.
- `leader` strategy APIs and Redis-backed `redisleader.NewStrategic` for
  candidate-registry leader election with FIFO, seed-stable random, scored
  strategies, Testcontainers coverage, and stress/cancellation tests.
- Cache stress and cancellation coverage using `GoroutineStressTester` and
  `AsyncJobTester`, plus zero-value `Memory` safety coverage.
- Type A research, spec, plan, review, and lessons artifacts for the initial
  cache contract.
- Go coverage reporting for CI and Nightly through native coverage profiles,
  package subtotal summaries, function-level text summaries, HTML reports,
  GitHub Step Summary output, and uploaded workflow artifacts.

### Changed

- Package documentation now lives in package-level `README.md` files, while
  root README files remain high-level indexes with links.
- README and WIP documentation now reflect the completed `0.3.0` release line,
  merged package surface, and open cache/coordination follow-up issues.
- `make bench-ratelimit` now exposes opt-in local rate limiter benchmark runs.

## [v0.2.0] - 2026-06-04

### Added

- `leader.GroupElector` and Redis-backed `redisleader.NewGroup` for
  semaphore-style multi-leader election with ZSET slot tokens.
- Circuit breaker and bulkhead policies for the first-party `resilience`
  package.
- Structured resilience events with stable policy type, event category, error
  category, retry attempt, circuit transition, timeout, and bulkhead data.
- HTTP client and server adapters for composing resilience policies with
  `net/http`.
- Redis Testcontainers smoke coverage for the reusable Redis fixture.

### Changed

- README examples now show retry, timeout, circuit breaker, bulkhead,
  observability hooks, HTTP adapters, and leader group election.

## [v0.1.1] - 2026-06-03

### Added

- Initial first-party `resilience` package with composable typed policies,
  retry, timeout, deterministic backoff, event hooks, and examples.
- Retrospective milestone evidence for the `0.1.0` foundation surface,
  including research, spec, plan, 7-tier review, and lessons artifacts.

### Fixed

- JSON deserialization now rejects trailing payloads after the first valid JSON
  value.

## [v0.1.0] - 2026-06-03

### Added

- Initial Go module with `core`, `testing`, `testcontainers/redis`, `leader`,
  and `leader/redis` packages.
- Redis-backed leader election with Testcontainers smoke coverage.
- Milestone research notes under `docs/research/`.
- English and Korean README files with roadmap, hero image, and architecture
  overview diagram.
- Project management scaffolding: `Makefile`, lint configuration, WIP log,
  package layout policy, and release guide.
- Nightly workflow that runs Testcontainers-backed tests on a scheduled smoke
  and full cadence.
- Core support helpers for validation, zero/default handling, pointers,
  strings, and small numeric checks.
- Collections helpers for chunking, grouping, distinct values, and error-aware
  slice transformations.
- Redis leader lifecycle tests for duplicate campaign, repeated resign, renewal
  loss, renewal failure, and leader lookup semantics.
- Testable Go examples for the `core`, `collections`, `codec`, `compression`,
  `concurrency`, `serialization`, and `testing/concurrency` packages.
- PostgreSQL, MySQL 8.4, NATS, and Kafka Testcontainers fixtures with smoke
  tests.
- Gomega-backed asynchronous test helpers for eventual and consistent
  conditions.
- Redis leader coordination examples for batch scheduling and migration gates.
- Redis leader key compatibility decision for Kotlin/Go mixed participants.

### Changed

- CI now validates formatting, module tidiness, vet, lint, tests, and race
  tests against real Testcontainers dependencies.
- `make test` and `make race` now pass `-count=1` so integration tests are not
  skipped by Go's test cache.
- `leader` API docs now define ownership, cancellation, idempotent resign,
  lost-leadership, and `errors.Is` comparison semantics.
