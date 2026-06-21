# Issue #202 Source Parity Matrix

Issue: [#202](https://github.com/bluetape4k/bluetape-go/issues/202)  
Milestone: `0.6.2`  
Date: 2026-06-21

## Decision

Use this matrix as the shared evidence source for the corrective `0.6.x`
series. The goal is selective Go-native parity, not a Kotlin/JVM API clone.

High-value gaps already have receiving epics:

- Core foundation gaps map to [#204](https://github.com/bluetape4k/bluetape-go/issues/204) / `0.6.3`.
- Testing helper gaps map to [#209](https://github.com/bluetape4k/bluetape-go/issues/209) / `0.6.4`.
- Testcontainers gaps map to [#215](https://github.com/bluetape4k/bluetape-go/issues/215) / `0.6.5`.
- Cross-cutting docs, examples, assertions, logging, time, and math closure maps
  to [#221](https://github.com/bluetape4k/bluetape-go/issues/221) / `0.6.6`.

No new issue is required from this pass. The uncovered high-value work fits the
existing `0.6.3` through `0.6.6` epics.

## Status Legend

| Status | Meaning |
|---|---|
| Implemented | Existing Go package covers the useful behavior. |
| Partial | Existing Go package covers a narrow slice, but source parity shows a meaningful gap. |
| Missing | No equivalent Go package or helper exists yet. |
| Excluded | Deliberate non-goal for Go or for the current library scope. |

## Core Matrix

| Source family | Kotlin source path | bluetape-go path | Current status | Quality status | Missing work | Target | Non-goal rationale |
|---|---|---|---|---|---|---|---|
| Validation / require support | `bluetape4k/core/.../support/RequireSupport.kt` | `core/validate.go`, `core/README.md` | Partial | Narrow and idiomatic; error-return style is correct for Go. | Broaden validation coverage only where repeated package code proves need; add table tests and docs before exporting more helpers. | #204 / #205 | Kotlin assertion DSL and exception-first contracts are not copied. |
| String helpers | `core/.../support/StringSupport.kt` | `core/string.go`, `core/README.md` | Partial | UTF-8 byte truncation and hex helpers exist; API is intentionally small. | Audit high-value trimming, blank, byte-length, and safe conversion helpers before expanding. | #204 / #205 | Extension-method style string APIs are excluded. |
| Range semantics | `core/.../ranges/*` | None | Missing | No public Go range primitive yet. | Add Go-shaped typed range helpers if they simplify validation and collection APIs. | #204 / #206 | Kotlin operator overload semantics and DSL constructors are excluded. |
| Collections | `core/.../collections/*` | `collections/slices.go`, `collections/maps.go` | Partial | Existing helpers are small and stable. | Evaluate `BoundedStack`, `RingBuffer`, `PaginatedList`, and lazy permutation support with race/stress evidence where shared state exists. | #204 / #206 | Java/Kotlin collection extension parity is excluded. |
| Time helpers | `core/.../javatimes/*`, `utils/javatimes/*` | None beyond standard library use | Missing | Current code correctly leans on `time`, but no shared helpers exist. | Add only repeated Go `time` helpers that improve call sites; document exclusions. | #204 / #208, #221 / #223 | Java Time DSL, `Period`, and JVM temporal type mirroring are excluded. |
| Hashing / wildcard | `core/.../utils/XXHasher.kt`, `Wildcard.kt` | Hashing mostly package-local; no wildcard helper | Partial | Package-local hashing is acceptable; no shared wildcard contract exists. | Decide whether shared wildcard matching and non-crypto hash helpers reduce duplication. | #204 / #207 | Broad hashing framework or JVM-compatible hash API is excluded. |
| Resource / system utilities | `core/.../utils/Resourcex.kt`, `Systemx.kt`, `ShutdownQueue.kt` | Mostly standard library and package-local cleanup | Partial | Go standard library is preferred; exported surface should stay small. | Add resource helpers only for recurring filesystem/test fixture needs. | #204 / #207 | Classpath resource loading, JVM shutdown hooks, and system property APIs are excluded. |
| Concurrency foundation | `core/.../concurrent/*`, virtual thread helpers | `concurrency/*`, `testing/concurrency/*` | Partial | Worker/group helpers exist and use Go contexts. | Audit cancellation, cleanup, and race behavior before any new primitive. | #204 / #205, #209 / #210 | Java executor, virtual thread, and `CompletableFuture` shapes are excluded. |
| Functional / Result helpers | `core/.../functional/*`, `ResultSupport.kt` | None | Excluded | Existing Go packages use explicit values and errors. | None unless a concrete call site proves repeated boilerplate. | #221 / #225 closure record | Kotlin `Result`, higher-order DSLs, and monadic wrappers are not a Go goal. |

## JUnit5 / Testing Matrix

| Source family | Kotlin source path | bluetape-go path | Current status | Quality status | Missing work | Target | Non-goal rationale |
|---|---|---|---|---|---|---|---|
| Await / polling | `testing/junit5/.../awaitility/*` | `testing/eventually.go` | Partial | Existing `Eventually` / `Consistently` style helpers are Go-native. | Add context-aware diagnostics and async cleanup contracts if #210 finds gaps. | #209 / #211 | Awaitility API compatibility and coroutine-specific names are excluded. |
| Stress testers | `testing/junit5/.../concurrency/*`, `StressTester.kt` | `testing/concurrency/*` | Partial | Existing goroutine stress helpers cover the right domain. | Harden race, cancellation, and failure diagnostics before broadening. | #209 / #210 | Java thread/structured task scope API parity is excluded. |
| Cancellation contracts | `testing/junit5/.../coroutines/*`, cancellation contract helpers | `testing/concurrency/*`, package-specific tests | Partial | Go context cancellation is the correct contract boundary. | Add reusable assertions for async APIs, goroutine cleanup, and bounded waiters. | #209 / #213 | Kotlin coroutine cancellation semantics are not copied directly. |
| Temp/output/env helpers | `TempFolderExtension`, `OutputCapture`, `SystemProperty` helpers | Standard `testing.T.TempDir`, package-local helpers | Partial | Standard library covers temp dirs; output/env helpers are scattered. | Add scoped output capture and env restoration helpers only if failure diagnostics remain clear. | #209 / #212 | JUnit extension lifecycle and global system property style are excluded. |
| Random/faker / parameter source | `FakerExtension`, `RandomExtension`, `FieldSource` | Table tests and package-local generators | Missing | Current tests use ordinary table-driven Go. | Research small test-data builders and fixture patterns without adding a framework. | #209 / #214, #221 / #222 | Reflection-heavy parameter-source extension APIs are excluded. |
| Stopwatch / timing | `StopwatchExtension` | Package-local benchmark/timing code | Missing | `testing` benchmarks remain primary. | Add timing helper only when ordinary benchmark/test output is insufficient. | #209 / #212, #221 / #223 | JUnit lifecycle timing extension parity is excluded. |
| Reporting / Mermaid | `testing/junit5/.../report/*` | None | Missing | No reporting framework exists. | Consider examples or generated docs only if they materially improve debugging. | #209 / #214, #221 / #224 | Replacing `go test` output with a reporting framework is excluded. |

## Testcontainers Matrix

| Source family | Kotlin source path | bluetape-go path | Current status | Quality status | Missing work | Target | Non-goal rationale |
|---|---|---|---|---|---|---|---|
| Existing wrappers | `testing/testcontainers/...` broad wrapper set | `testcontainers/postgres`, `redis`, `mysql`, `kafka`, `nats` | Partial | Existing wrappers cover the first roadmap services. | Harden lifecycle, readiness, cleanup timeout, and connection detail contracts first. | #215 / #216 | Mirroring every Kotlin wrapper before demand exists is excluded. |
| Generic server | `GenericServer`, generic container helpers | None | Missing | Current wrappers duplicate some lifecycle ideas. | Add a small generic server abstraction if it reduces wrapper drift. | #215 / #217 | A large inheritance-like container framework is excluded. |
| Property export | `PropertyExportingServer`, exported connection keys | Package-specific connection methods | Missing | Package-specific methods are usable but inconsistent. | Standardize property/export contracts and docs for server fixtures. | #215 / #217 | JVM system property export is excluded; Go should return typed values or maps. |
| Database / storage | PostgreSQL, MySQL, MariaDB, CockroachDB, ClickHouse, Trino, MongoDB, Redis, MinIO, pgvector/PostGIS | PostgreSQL, MySQL, Redis | Partial | Current DB/storage wrappers are focused. | Add roadmap-driven DB/storage fixtures for SQL, cache, AWS/IO, and graph work. | #215 / #218 | Low-value service coverage without a package consumer is excluded. |
| Messaging | Kafka, Redpanda, RabbitMQ, NATS, Pulsar | Kafka, NATS | Partial | Current messaging coverage matches early roadmap needs. | Add RabbitMQ/Redpanda/Pulsar only when audit/outbox or examples require them. | #215 / #219 | Full broker catalog parity is excluded. |
| HTTP mock | WireMock, Nginx, embedded HTTP helpers | Package-local HTTP tests | Missing | Existing package tests avoid global fixtures. | Add HTTP mock fixtures only if upcoming IO/HTTP integration recipes require them. | #215 / #219, #221 / #224 | Kotlin WebFlux/Ktor mock-server API parity is excluded. |
| Fault injection | Toxiproxy and infra wait strategies | Package-local tests | Missing | Fault injection is not centralized yet. | Add Toxiproxy or similar only for resilience/cache/workflow tests that need network fault proof. | #215 / #219 | Always-on Docker fault tests are excluded. |
| AWS / emulators | MiniStack, Floci, LocalStack, DynamoDB Local, ElasticMQ | None | Missing | AWS packages are future roadmap work. | Choose emulator fixtures based on #47/#60-#64 before implementing. | #215 / #220 | Deprecated LocalStack-first parity is excluded unless a design issue selects it. |
| Infrastructure / graph / observability | Keycloak, Vault, Consul, etcd, Zipkin, Grafana, K3s, Neo4j, Memgraph, FalkorDB, AGE | None | Missing | No current Go package consumes these fixtures. | Add only when graph, audit, security, or integration examples need them. | #215 / #220, #221 / #224 | Infrastructure wrappers without roadmap demand are excluded. |

## Broader bluetape4k-projects Routing

| Source family | Source modules | Routing decision |
|---|---|---|
| Assertions / fixtures | `testing/assertions`, JUnit5 fixtures | Track in #221 / #222 after testing helper hardening lands. |
| Logging / math / science / state utilities | `logging`, `utils/math`, `utils/science`, `utils/states` | Track in #221 / #223 only when small Go helpers or examples add value. |
| Ktor / Spring Boot testing | `ktor/*`, `spring-boot/*`, mock web server modules | Use as recipe evidence for #221 / #224; do not port framework-specific APIs. |
| Data / SQL / repository | `data/*`, SQL/repository examples | Defer to later SQL roadmap issues; Testcontainers support routes through #215. |
| AWS / IO | `aws/*`, `io/*` | Defer to existing AWS/IO roadmap issues; emulator fixtures route through #215. |
| Text / tokenizer | `text/*`, tokenizer examples | Defer to `0.10.0` text roadmap; only shared test helpers route through #209/#221. |
| Audit / outbox | `audit/*`, messaging examples | Defer to `0.11.0`; messaging fixtures route through #215. |
| Graph | `graph/*` | Defer to `0.12.0`; graph fixtures route through #215. |

## Acceptance Check

- Implemented, partial, excluded, and missing states are separated above.
- Every high-value missing item maps to #204, #209, #215, #221, or a later
  roadmap family.
- JVM-only, framework-only, and DSL-shaped items have explicit non-goal
  rationale.
- No behavior change is made by this note; implementation work remains in the
  receiving issues.
