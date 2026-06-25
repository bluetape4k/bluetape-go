# Issue 41 Audit And Outbox Research Scope

Issue #41 is the 0.7.0 research gate for deciding how
`bluetape4k-javers` patterns should shape the Go audit/event milestone. The Go
track should port the audit/history and event-boundary concepts, not JaVers'
object diff engine or JVM framework integrations.

## Source Inventory

Source repository: `/Users/debop/work/bluetape4k/bluetape4k-javers`

- `javers-core` provides typed JaVers helpers, snapshot codecs, repository base
  contracts, local cache repositories, composite repository fanout, head
  tracking, sequence assignment, metadata helpers, and snapshot event
  envelopes.
- `javers-ddd` provides aggregate root and domain event contracts, aggregate
  repository orchestration, synchronous fail-fast domain event publishers, and
  optional Spring, Kafka, and NATS publisher adapters. It explicitly does not
  replace the source-of-truth repository and is not a durable outbox.
- `javers-exposed` provides SQL-backed CDO snapshot persistence with small
  commit/snapshot tables, repository head restoration, SQL filter pushdown,
  optional Exposed DAO lifecycle hooks, H2/PostgreSQL/MySQL support, and
  migration-owned schema options.
- `javers-persistence-redis` provides direct Redis-backed snapshot stores. It
  is a possible durable audit store only when Redis is accepted as the audit
  source; it should not be treated as SQL write-behind.
- `javers-persistence-kafka` is intentionally write-only. It publishes encoded
  snapshot events to Kafka and requires a projector or read-capable repository
  for history queries.
- Examples cover Exposed DDD CQRS, Ktor REST audit history, Spring Boot REST
  audit history, Kafka domain events, and Redis read-model projections.

## Current Go Repository Evidence

- `bluetape-go` already has Redis, Kafka, NATS, PostgreSQL/MySQL, MongoDB, AWS,
  testcontainers, and eventual consistency testing dependencies, but the audit
  package does not exist yet.
- The 0.11.0 placeholder already says the track should become a Go audit/event
  package without depending on JaVers.
- Issue #46 and #56-#59 already exist, so #41 should update those issues rather
  than create another broad implementation issue.

## Ranking

| Area | Go fit | Risk | Decision |
|---|---:|---:|---|
| Aggregate/domain event/audit model | High | Medium | Implement #56 first with stable IDs, revisions, metadata, and serialization rules. |
| Audit repository/history query interface | High | Medium | Implement #57 with storage-neutral interfaces and in-memory conformance tests first. |
| SQL audit repository | Medium/high | Medium/high | Defer concrete adapter until #100 defines relational SQL boundaries. |
| Durable outbox contract | Medium/high | High | Design in #58, but keep concrete adapters separate and explicit. |
| Kafka/NATS publishers | Medium | High | Use after outbox/idempotency contracts exist; they are delivery adapters, not history stores. |
| Redis audit store | Medium | High | Defer direct durable Redis store until repository semantics and replay are proven. |
| JaVers-style object diff | Low | High | Defer; use explicit event/audit payloads first. |
| Ktor/Spring/Exposed example parity | Low | Medium | Translate to Go runnable examples, not framework auto-configuration. |
| Full event sourcing framework | Low | High | Non-goal for 0.11.0. |

## Implement

- #56 should define the core model:
  aggregate id, aggregate type, revision, domain event, audit entry, snapshot
  metadata, author, occurred/recorded timestamps, idempotency key, and
  serialization compatibility rules.
- #57 should define repository and history query interfaces with an in-memory
  implementation and reusable conformance tests. Query history by aggregate,
  type, revision/time range, newest/previous entries, and metadata filters
  only where the model can support stable behavior.
- #59 should add a small runnable Go example after #56/#57 exist. It should
  demonstrate command-side persistence boundaries, audit history queries, and
  optional publisher hooks with in-memory or file-backed fixtures.

## Adopt Later

- SQL persistence should wait for #100 or reuse its eventual repository
  boundary. SQL is the most natural durable audit source for history queries,
  but its schema and migration story should not be designed twice.
- Kafka and NATS adapters should wait until #58 defines at-least-once delivery,
  idempotency, retry/dead-letter, ordering, serialization, and application-owned
  responsibilities.
- Redis can be considered for projections or a direct audit store only after
  replay, ordering, and head/restoration semantics are explicit.

## Example-only

- A Go example can mirror the source examples' responsibilities: command-side
  writes, audit entry recording, history lookup, and optional event publication.
- Framework parity is not needed. Use plain Go package tests or a small
  `net/http` example if HTTP adds real caller value.

## Defer

- JaVers object graph diffing, shadow reconstruction, CDO snapshot internals,
  Exposed DAO lifecycle hooks, Spring transaction synchronization, Ktor/Spring
  auto-wiring, and full event-sourcing framework behavior.
- Direct Kafka history queries. Kafka remains a delivery stream unless a
  projector materializes events into a read-capable repository.
- Redis write-behind for SQL audit writes. Redis should be a projection/cache
  path or an explicitly accepted audit source, not hidden persistence.

## Issue Updates Required

- #46: record the issue #41 decision and keep the epic framed as audit/history
  support, not event sourcing or JaVers clone work.
- #56: keep as the first implementation issue for core model contracts and
  serialization expectations.
- #57: keep as repository/history query interface plus in-memory conformance;
  defer SQL/Redis/Kafka concrete stores.
- #58: keep as outbox/publisher research; prefer SQL outbox boundary first and
  treat Kafka/NATS/Redis as adapters after delivery semantics are explicit.
- #59: keep as runnable example after #56/#57; avoid framework parity claims.

## Validation Plan

- Documentation-only PR: `git diff --check` and targeted `rg`.
- Verify #46 and #56-#59 issue bodies contain the #41 research update.
- No Go tests are required for this PR because no Go code changes.

## Follow-up Recommendation

Start #56 with explicit audit/event records and no hidden diffing. Then make
#57 prove repository semantics through in-memory conformance tests before
selecting SQL, Redis, Kafka, or NATS adapters.
