# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses semantic versioning once the first tag is published.

## [Unreleased]

### Added

- `money.NewIMFProvider` for IMF Exchange Rates SDMX-backed reference rates,
  with configurable period-average/end-of-period families, frequency, cache and
  stale fallback metadata, USD/EUR domestic pivot support, cancellation tests,
  and bilingual README/research documentation. SDR/XDR exposure remains deferred
  until the currency backend can construct XDR values safely.

## [v0.6.1] - 2026-06-21

### Added

- `probabilistic/redis` package with Redis-backed shared Bloom filters,
  Cluster-safe hash-tagged key pairs, immutable config metadata, static Lua
  bitmap operations, cancellation/race/stress coverage, compile-checked
  examples, and bilingual README/runbook documentation. Redis-backed Cuckoo and
  HLL/HyperLogLog constructors remain follow-up scope after #182.
- Optional JWT provider cache adapters with `NewCachedProvider` and
  `NewCachedDistributedProvider`, scoped token-digest cache keys, trusted
  `cache.Cache[string,*jwt.Reader]` backends, warm-hit key revalidation,
  same-key miss coalescing, cancellation/race/stress coverage,
  compile-checked examples, diagram-backed bilingual README documentation, and
  operator caveats for process-local clear scope and unsupported untrusted
  shared/external caches.
- `money` provider-backed exchange-rate conversion with `ExchangeRateProvider`,
  `ConvertWithProvider`, `NewECBProvider`, caller-visible source/freshness/stale
  fallback metadata, cancellation/retry/cache coverage, stress/race tests, and
  diagram-backed bilingual README documentation. IMF and Bloomberg providers
  remain follow-up issues #231 and #232.
- `money.CurrencyByLocale` CLDR-backed locale currency mapping for
  explicit-region BCP47 tags, with missing/no-tender/multi-tender rejection,
  stress/race coverage, and diagram-backed bilingual README documentation.
- `money` FastMoney evaluation benchmark evidence, with raw benchmark output,
  chart-backed bilingual README guidance, and a documented decision to keep
  `Money`, `NewMinor`, and `MinorUnits` as the public minor-unit path.

### Changed

- Documented the #174 JWT compression/JOSE decision: signed JWT compression is
  a non-goal for the current `jwt` helper, `zip=DEF` belongs to a future
  explicit JWE boundary, and `go-jose/go-jose/v4` is the preferred candidate if
  that optional JWE scope is ever implemented.

## [v0.6.0] - 2026-06-09

### Added

- `id` package with repo-owned UUID v4/v7 string generators, random and
  monotonic ULID generators, standard seconds-precision KSUID generation,
  parsing, and timestamp extraction, Snowflake int64 generation and decoding,
  sentinel/typed error contracts, stress/race coverage, benchmark smoke, and
  bilingual package README coverage. Kotlin-compatible millisecond KSUID remains
  deferred to #171.
- `jwt` package with explicit HS/RS/PS algorithm providers, fixed and in-memory
  rotating KeyChains, typed claim/header readers, issuer/subject/audience/exp
  validation helpers, `kid` lookup, weak-secret rejection, unsupported JOSE
  header rejection, sentinel error contracts, stress/race coverage, and
  bilingual package README coverage. Distributed repositories, JOSE compression,
  and provider cache adapters remain deferred to #173, #174, and #175.
- `measure` package with typed `Unit[D]` and `Measure[D]`, built-in length,
  time, mass, area, volume, storage, binary size, frequency, energy, power,
  pressure, angle, graphics length, velocity, acceleration, affine temperature,
  generic and family parsers, compound unit helpers, source-parity named
  helpers, sentinel error contracts, stress/race coverage, and bilingual README
  coverage. Decimal money precision remains deferred to the future money
  package.
- `money` package with ISO 4217 currency wrappers, decimal-backed `Money`
  values, same-currency arithmetic, half-even rounding, minor-unit helpers,
  JSON/text serialization, caller-supplied `ExchangeRate` conversion, typed
  sentinel errors, goroutine stress/race coverage, and bilingual package README
  coverage. Provider-backed exchange rates, full locale mapping, and separate
  long-backed FastMoney remain deferred to #178, #179, and #180.
- `probabilistic` package with goroutine-safe in-memory Bloom filters,
  deterministic config sizing, SHA-256 double hashing, explicit generic hasher
  keys, compatible filter merge, false-positive and no-false-negative contract
  tests, sentinel errors, stress/race coverage, opt-in benchmark smoke, and
  bilingual package README coverage. Redis-backed Bloom, Cuckoo, and HyperLogLog
  remain deferred to #182.

## [v0.5.1] - 2026-06-08

### Fixed

- Checkpointed `batch.Step` writer failures that match `SkipPolicy` now fail
  with `ErrUnsafeWriterSkipCheckpoint` instead of advancing the checkpoint after
  an unsafe skipped writer chunk. Restarts replay from the last safe checkpoint
  and preserve the original writer error for `errors.Is` checks.

## [v0.5.0] - 2026-06-08

### Added

- `batch` package with first-party reader/processor/writer chunk steps,
  sequential jobs, reports, filtering, context cancellation, resource cleanup,
  and stress/cancellation coverage.
- Batch retry and skip policies for processor/write failures, with explicit
  context-cancellation preservation and retry/skip count reporting.
- Pluggable checkpoint support with `CheckpointReader`, `CheckpointStore`,
  in-memory checkpoint storage, restart coverage, and checkpoint persistence
  after committed progress.
- Leader-guarded batch examples in `leader/redis` showing scheduled batch work
  and migration workloads that only run under the current Redis leader.
- Runnable Redis Testcontainers commands and bilingual README coverage for
  leader-guarded batch examples.
- README architecture diagram refresh showing batch retry/skip policies and
  checkpoint restart scope.

### Changed

- Root README architecture assets now reflect the completed 0.5.0 batch
  recovery scope.
- WIP and release guide now reflect 0.5.0 release-preparation state.

## [v0.4.0] - 2026-06-06

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
- README diagram assets for 0.4.0 workflow primitives and complex Redis
  coordination packages, with PNG-only README embeds and adjacent SVG sources.

### Changed

- Every package-level `README.md` now has a sibling `README.ko.md` and a
  consistent `English | 한국어` language switch.
- Root README, WIP, and release guide now reflect the closed `0.4.0` milestone
  and `v0.4.0` release-preparation state.

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
