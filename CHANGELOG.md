# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses semantic versioning once the first tag is published.

## [Unreleased]

### Added

- Add mandatory public provider conformance runners in `leader/leadertest`,
  `lock/locktest`, and `ratelimit/ratelimittest`, with in-memory reference
  fixtures and Redis/Mongo/local provider adoption.
- Add the PostgreSQL-only `leader/sql` single-elector provider over a
  caller-owned `*sql.DB` and `public.bluetape_leader_leases` row leases, with
  mandatory `leader/leadertest` conformance, Testcontainers fault recovery,
  least-privilege role proof, bilingual operations guidance, and a verified
  row-lease sequence diagram.
- Add `leader/etcd` single-leader election over a caller-owned etcd v3 client
  and official Session/Election primitives, with server-granted TTL,
  exact-key/Proclaim fail-closed monitoring, mandatory conformance, authenticated
  range and lease-revoke proof, and bilingual shutdown guidance. The provider
  supplies no fencing token; TTL passage alone never proves remote cleanup.
- Add `ratelimit/sql` PostgreSQL atomic token buckets for moderate-QPS,
  database-only deployments. Callers own the fixed schema, `*sql.DB`, and
  bounded cleanup scheduler; Redis remains the high-QPS choice. Redis and SQL
  failures share `ratelimit.OperationError` and `ErrCommitUnknown` inspection,
  and commit-unknown debits must not be replayed automatically.
- Add `batch/sqlcheckpoint` for PostgreSQL durable checkpoints that commit a
  batch callback and consumed-input progress in one caller-owned transaction.
  Revision CAS rejects competing writers; commit-unknown permits only a fresh
  bounded load, while `ErrAtomicityUnknown` requires quiescing and manual
  reconciliation before any replay.

- Add `cache/redisfory` for bounded Go-native Apache Fory values stored
  directly in Redis with explicit profiles, BTFV envelopes, TTLs, and schema
  generation key isolation.
- Add `cache/redisvalue` for bounded generic serialized Redis L2 values and a
  reference-preserving process-local tiered decorator; RESP3-coherent
  invalidation remains excluded and tracked separately.
- Add `redis` foundation package with key, owner-token, lease script, TTL, and
  redacted Redis operation error primitives.

### Changed

- Unify single-leader campaign waiting, local-state sentinels, typed provider
  failures, and commit-unknown cleanup across Redis and Mongo.
- Require the `leader/sql` migration on the fixed `public` relation and route
  mutations, observations, and reconciliation probes to one writable primary.
  Indeterminate cleanup retries bounded `Resign` on the same elector before
  full-lease expiry fallback; the v0.19.0 provider does not support fencing,
  custom schemas, group election, or strategic election.
- Publish the bilingual [v0.19.0 provider rollout runbook](docs/release/v0.19.0-provider-conformance-runbook.md)
  with mixed-version constraints, telemetry labels, canary thresholds, and
  resign/TTL rollback completion gates.
- Preserve nonblank custom Redis lock token bytes without trimming. Lock callers
  must handle a non-nil lease together with `redis.ErrCommitUnknown`, retry the
  same release callback, and use TTL fallback. Redis rate-limit callers must not
  replay commit-unknown requests and should wait a full refill interval or
  account for one possible debit.

## [v0.18.0] - 2026-07-10

### Added

- `leader/mongo` group leader elector backend with bounded slot lease
  documents, exact `MaxLeaders` admission under concurrent acquisition,
  renewal-loss detection, Testcontainers stress coverage, and bilingual README
  documentation.
- `leader/mongo` strategic leader elector backend with MongoDB candidate
  registry documents, FIFO/random/scored strategy execution, atomic result
  updates, stale-candidate pruning, Testcontainers stress coverage, and
  bilingual README documentation.
- `graph/graphio/graphml` optional bounded GraphML import/export package for a
  directed property graph subset, including scalar key/data attributes, explicit
  XML input limits, fail-closed unsupported construct tests, and bilingual
  README documentation.
- `audit/sqloutbox/redisstreams` Redis Streams publisher provider with a narrow
  `XADD` client surface, stable sqloutbox event/idempotency metadata,
  Testcontainers-backed duplicate attempt and relay retry coverage, and
  bilingual README documentation.

## [v0.17.0] - 2026-07-09

### Added

- `leader/mongo` single leader elector backend with caller-owned MongoDB
  collections, owner-token lease documents, optional TTL cleanup index support,
  renewal-loss detection, contention tests, and bilingual README coverage.

### Changed

- Root and package README documentation now links source-checked workshop
  adoption examples, active cross-repo workshop issues, and the 0.17.0
  workshop adoption release-readiness note.
- `resilience` README guidance now names the official application-level
  `otelslog` bridge path while keeping OpenTelemetry exporters out of
  `bluetape-go` library packages.

## [v0.16.0] - 2026-07-08

### Added

- Redis HyperLogLog support in `probabilistic/redis`, including
  `NewHyperLogLog`, `NewStringHyperLogLog`, and `NewBytesHyperLogLog`
  constructors, SHA-256 value digests, `Add`, `Count`, and `Merge`
  operations, examples, and bilingual README coverage.
- Testcontainers-backed Redis probabilistic coverage for Bloom filters and
  HyperLogLog, including bounded container startup, live Redis cleanup,
  cancellation checks, stress coverage, and race validation evidence.
- Research and release lessons for Redis Cuckoo and HyperLogLog support,
  selecting core Redis HLL as the first follow-up structure while keeping
  RedisBloom `CF*` Cuckoo support module-gated.

### Changed

- `probabilistic/redis` README documentation and runtime diagrams now separate
  current core Redis Bloom/HLL assumptions from future RedisBloom module
  Cuckoo support.
- Root release state was reconciled with the `v0.15.0` main release tree before
  the 0.16.0 Redis probabilistic work continued.

## [v0.15.0] - 2026-07-08

### Added

- Audit publisher adoption track for `audit/sqloutbox`, including a
  documented `Publisher` retry contract, stable `Record.EventID` and
  `Record.IdempotencyKey` handoff guidance, and duplicate-safe at-least-once
  delivery examples.
- `audit/sqloutbox/sqloutboxtest` with deterministic `DiscardPublisher`,
  `PublisherFunc`, and goroutine-safe `RecordingPublisher` helpers for relay
  tests, examples, retry assertions, and duplicate-delivery evidence.
- Research and lessons for the first audit publisher adapter target, selecting
  a standard-library test/example helper before Kafka, NATS, Redis Streams, or
  other durable transport adapters.
- Retained profiling evidence for JSON repeated-collection decoding and zstd
  compression allocation cost under the 0.15.0 SerDe follow-up track.

### Changed

- `serialization.JSONSerializer` now uses `json.Unmarshal` on the default
  decode path while preserving strict trailing-payload rejection and
  `WithDisallowUnknownFields` behavior through the decoder path.
- `compression.Zstd().Compress` now reuses internal zstd stream encoders for
  byte-slice compression while keeping `NewWriter` caller-owned and
  independent.
- Root and package README guidance now links sqloutbox test publishers and
  records the audit publisher adoption boundary without promising durable
  broker topology.

## [v0.14.0] - 2026-07-07

### Added

- Cross-repo SerDe and compression benchmark baseline for `serialization`,
  `codec`, and `compression`, including shared fixture/scenario definitions,
  Go benchmark runners, raw `-benchmem` outputs, and environment metadata.
- Evidence-scoped recommendation matrix comparing Go, Rust, and JVM
  serialization/compression behavior while separating measured evidence from
  follow-up hypotheses.
- Benchmark artifact retention template and issue-specific output directory for
  reproducible future benchmark reports.

### Changed

- Root, serialization, codec, compression, and research READMEs now point to
  the 0.14.0 benchmark snapshot and raw evidence instead of making production
  ranking claims.
- Benchmark runners validate round-trip behavior before timing and include
  deterministic scenario names for stable downstream analysis.

## [v0.13.0] - 2026-07-07

### Added

- Retrospective 0.1.0 through 0.12.0 release-readiness audit with tracked
  7-tier review evidence, final P0/P1 counts, deferred P2/P3 routing, and
  release preflight state.
- Missing stress and async cancellation coverage for existing concurrency,
  resilience, DynamoDB batchwrite, and testing-helper contracts, including
  race-detector validation.
- `testcontainers/mongodb` package for reusable MongoDB integration fixtures
  based on Testcontainers for Go, with caller-owned MongoDB clients and
  environment-exportable connection details.

### Changed

- Cumulative lesson hardening now records bounded cleanup contexts and
  errcheck-shaped cleanup examples across Testcontainers, cache, Redis
  coordination, and JWT documentation.
- Feature-gap triage now classifies later audit, probabilistic, messaging,
  AWS, SQL, graph, and HTTP fixture ideas without blocking the 0.13.0 line.

### Fixed

- `cache.Memory.GetOrLoad` now preserves same-key caller cancellation isolation
  without writing late canceled loader results into the cache.
- `ratelimit/redis` now preserves caller-owned keys instead of normalizing
  distinct keys into the same Redis storage key.

## [v0.12.0] - 2026-07-06

### Added

- Core foundation parity pass with source-backed Go-native decisions for
  `core`, `collections`, `codec`, `concurrency`, observability conventions,
  and rule-engine boundaries, explicitly rejecting JVM-shaped broad helper
  surfaces.
- `core` string validation and UUID helper additions for blank checks,
  string predicates, canonical UUID parsing/rendering, and narrow caller-owned
  text utility behavior.
- `collections` helper additions for small slice-oriented primitives with
  copied-output behavior, deterministic examples, and table-driven coverage.
- `codec` canonical UUID URL62 helpers that reject non-canonical or oversized
  aliases and preserve round-trip compatibility evidence.
- `concurrency` round-robin primitive with goroutine-safe selection behavior,
  deterministic examples, stress coverage, and race validation.
- First-party `rules` package primitives with immutable facts, deterministic
  rule execution, composite rules, bounded inference, typed non-convergence
  errors, YAML/JSON expression-backed readers, and bilingual README diagrams.
- Package README diagram coverage for previously missing package docs, with
  paired SVG/PNG assets and visual/audit review evidence.

### Changed

- Public examples and package-local hooks now use caller-owned `log/slog`
  patterns without adding a global bluetape-go logger registry.
- Root and package READMEs now describe the 0.12.0 rule/core foundation scope
  and keep Korean docs aligned with English package behavior.

## [v0.11.0] - 2026-07-03

### Added

- `imagekit` package with dependency-light pure-Go resize, thumbnail, format
  conversion, bounded image decode/encode limits, explicit option validation,
  benchmark evidence, README usage docs, and checked transform-flow diagrams.
- Optional `examples/imagekit-govips` adapter proving where callers can place
  libvips-backed image processing without making `govips` a core module
  dependency.
- `encrypt` package with a stdlib AES-GCM facade for random nonce generation,
  AAD-bound authenticated encryption, nonce/ciphertext framing, key-size
  validation, and tamper/error coverage.
- `graph/neo4j` adapter proof with Neo4j-driver client options, graph value
  conversion, redacted connection/query errors, bilingual package docs, and
  Memgraph compatibility tests.
- `examples/graph/iamaccess` runnable IAM access graph example with principal,
  role, policy, and resource edges, bounded path analysis, root README links,
  and a source-backed architecture diagram.
- Rule-engine primitive research that keeps rule execution out of core until
  a Go-style evaluation boundary can be proven without importing JVM shapes.

## [v0.10.0] - 2026-07-01

### Added

- `graph` package with model-only vertex, edge, path, label, ID, shallow
  property, and validated JSON values for graph I/O helpers and examples. Graph
  repository/session/schema/query/transaction/backend contracts remain deferred
  until follow-up I/O, backend, and example issues prove shared behavior.
- `graph/graphio` package with stream-oriented NDJSON and paired CSV
  import/export helpers for graph vertices and edges, bounded read defaults,
  duplicate/missing endpoint policies, CSV formula escaping, redacted errors,
  and stateful reader/writer APIs.
- Graph backend adapter feasibility research that selects a Neo4j adapter proof
  first, routes Memgraph through Neo4j-driver compatibility coverage, and
  defers AGE, FalkorDB, TinkerPop/TinkerGraph, and Neptune until their Go driver
  or local-test boundaries are proven.
- `examples/graph/observability` runnable incident-response graph example with
  seed data, blast-radius queries, alert-boundary and ownership lookups,
  NDJSON graph I/O round-trip coverage, bilingual README docs, and a topology
  diagram.

## [v0.9.0] - 2026-06-29

### Added

- `audit` package with aggregate IDs, monotonic revisions, caller-owned domain
  event IDs, idempotency keys, validated JSON audit entries, pending event
  recorders, storage-neutral history reconstruction, repository/query
  interfaces, reusable adapter conformance tests, and a goroutine-safe
  non-durable in-memory repository.
- Audit outbox design selecting a SQL outbox store and relay contract as the
  first durable publisher target, with Kafka, NATS, Redis Streams, RabbitMQ,
  Redpanda, Pulsar, and direct Redis audit storage deferred until the durable
  outbox boundary is proven.
- `audit/sqloutbox` package with PostgreSQL-backed enqueue, claim,
  claim-attempt-guarded publish/failure marking, claim leases,
  retry/dead-letter state, per-aggregate claim ordering, and a
  context-cancellable at-least-once relay.
- `examples/audit` runnable order-service recipe demonstrating aggregate
  changes, audit repository history queries, and in-memory outbox replay
  boundaries.

## [v0.8.0] - 2026-06-27

### Added

- `textsearch` package with immutable Aho-Corasick multi-pattern matchers,
  first/all match modes, overlap policy, Unicode normalization, word-boundary
  filtering, replacement, masking, and concurrency stress coverage.
- `textsearch` blockword dictionaries with severity metadata, deterministic
  detection/masking responses, static rebuild semantics, and Korean/Japanese/
  ASCII stress coverage.
- `textsearch` tokenizer core interfaces with byte-span tokens, normalized text
  helpers, coarse POS extension points, dictionary providers, and a
  dependency-free deterministic tokenizer for tests and simple lexical flows.
- Optional `textsearch/japanese` Kagome v2 adapter with IPA dictionary defaults,
  byte-span preservation, Kagome POS metadata, noun/verb filters, blockword
  examples, and goroutine stress coverage.
- Optional `textsearch/language` Lingua-Go adapter with all/subset detector
  builders, lazy/preloaded and low-accuracy modes, mixed-language sections,
  Unicode script helpers, and goroutine stress coverage.

## [v0.7.0] - 2026-06-26

### Added

- `sqlkit` package with runtime-first `database/sql` transaction helpers,
  small `Session`/`Queryer`/`Execer` interfaces, explicit row mapping helpers,
  and cardinality-aware `QueryAll`, `QueryOptional`, and `QueryOne` functions.
- PostgreSQL-first inspectable SQL builders for `SELECT`, `INSERT`, `UPDATE`,
  and `DELETE`, including copied argument slices, validated quoted identifiers,
  full-table update/delete guards, and context-aware `Statement.Exec`.
- Testcontainers-backed PostgreSQL repository examples covering create, read,
  update, delete, rollback, and relational query behavior through `sqlkit`.
- SQL generator and migration guidance documenting when to choose direct
  `database/sql`, `sqlkit`, sqlc, Jet, ent, Bun, GORM, goqu, and Atlas while
  keeping sqlc, Jet, and Atlas outside the core runtime dependency boundary.

### Changed

- Root README and Korean README now list `sqlkit` as an active data-access
  package and link the optional SQL generator/migration guide.
- The 0.7.0 relational SQL epic records the runtime-first direction from #100,
  with mandatory code generation, broad ORM behavior, hidden migrations, and
  cross-database abstraction kept out of the first package slice.

## [v0.6.8] - 2026-06-25

### Added

- `compression.DecompressLimit` and `ErrDecompressedSizeExceeded` for callers
  that handle untrusted compressed payloads and need a hard expanded-output
  limit without changing the existing `Compressor` interface.
- `core.ErrInvalidArgument` and `collections.ErrInvalidArgument` sentinel
  contracts for caller-input validation failures in public helper APIs.

### Changed

- Root README release status now reflects the published `v0.6.7` line and the
  MongoDB-backed JWT KeyChain repository scope.
- Redis leader and lock examples now use bounded campaign/acquire contexts and
  separate bounded cleanup contexts.
- AWS S3, SQS/SNS, and DynamoDB batchwrite examples now show bounded contexts
  and preserve SDK errors instead of discarding them.
- Docker-backed tests now use explicit startup contexts in PostgreSQL, MySQL,
  MariaDB, NATS, Redis Bloom, and JWT Redis/Mongo fixtures.

### Fixed

- ECB exchange-rate XML fetches now cap response bodies before XML decoding.
- MongoDB JWT repository trim cursor cleanup now uses a bounded cleanup context.
- Redis leader and group elector `Resign` now honor caller cancellation while
  waiting for renewal workers, and renewal Redis calls are bounded per
  operation.
- Redis near-cache `Close` now tracks the `OnError` reporter goroutine and
  surfaces bounded shutdown failures.

## [v0.6.7] - 2026-06-25

### Added

- `jwt.MongoRepository` and `jwt/mongo` facade for MongoDB-backed distributed
  JWT key-chain storage, including shared-provider rotation, `kid` lookup,
  capacity trimming, expiry handling, cancellation, and Testcontainers MongoDB
  coverage.

## [v0.6.6] - 2026-06-25

### Added

- Focused testing fixture examples, assertion patterns, golden-file data, and
  bilingual testing README updates for developer experience parity.
- Utility parity boundary documentation for logging, time, and math helpers,
  keeping Go standard-library behavior preferred where it is clearer.
- `examples/integration` recipes across batch, workflow, cache, resilience, id,
  JWT, Redis lock/leader, and Testcontainers Redis with service-free, race, and
  Docker-backed smoke commands.
- Corrective-series closure audit documenting the rechecked 0.6.x parity matrix,
  `P0=0 P1=0` state, deferred follow-ups, and explicit Go non-goals.

### Changed

- Root README release roadmap now reflects the completed corrective 0.6.3
  through 0.6.6 series and separates later roadmap work from closed parity
  hardening.

## [v0.6.5] - 2026-06-25

### Added

- Shared Testcontainers server/property export abstraction with bounded
  startup error reporting and service connection metadata helpers.
- Testcontainers wrappers for MariaDB, Toxiproxy, and Floci, including
  service config smoke coverage for S3, SQS, SNS, and DynamoDB.
- Direct AWS SDK for Go examples for S3, SQS/SNS, and DynamoDB batch write
  retry helpers, with bilingual README coverage and explicit wrapper
  boundary decisions.

### Changed

- Hardened existing Testcontainers lifecycle and connection contracts before
  adding more service fixtures, including serial execution guidance and
  cleanup/startup diagnostics.

## [v0.6.4] - 2026-06-25

### Added

- `testing` async await/polling helpers with context-aware timeout behavior,
  interval control, examples, and focused tests.
- `testing` cancellation contract assertions for context-aware APIs, including
  success/failure helpers and examples for caller-owned cancellation behavior.
- Scoped temporary output and environment helpers for tests, with cleanup
  coverage and bilingual README documentation.
- Research notes for random data parameter sources and test reporting helpers,
  rejecting broad fixture dependencies for the current milestone.

### Changed

- Hardened `testing/concurrency` helper reporting so stress failures preserve
  useful caller-visible evidence without weakening race-compatible execution.

## [v0.6.3] - 2026-06-25

### Added

- `collections` bounded stack, ring buffer, pagination, and permutation helpers
  with Go-native APIs, table-driven tests, and bilingual README coverage.
- `core` range helpers, wildcard matching, XXH64 hashing, resource-style
  helper documentation, and quarter/time helpers inspired by bluetape4k-core
  where the Go standard library does not already provide the simpler contract.

### Changed

- Hardened `core`, `collections`, `codec`, and `serialization` text/binary
  contracts, including invalid UTF-8, nil/empty, malformed input, and
  documentation boundaries before adding more foundation parity APIs.

## [v0.6.2] - 2026-06-21

### Added

- `money.NewIMFProvider` for IMF Exchange Rates SDMX-backed reference rates,
  with configurable period-average/end-of-period families, frequency, cache and
  stale fallback metadata, USD/EUR domestic pivot support, cancellation tests,
  and bilingual README/research documentation. SDR/XDR exposure remains deferred
  until the currency backend can construct XDR values safely.
- Bloomberg-backed exchange-rate provider evaluation for `money`, documenting
  SAPI, B-PIPE, Data License, BLPAPI, entitlement, credential, freshness,
  failure-mapping, and test-strategy boundaries while keeping Bloomberg
  dependencies and paid access out of default `money` behavior and CI.

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
