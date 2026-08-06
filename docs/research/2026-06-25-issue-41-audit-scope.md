# Issue 41 Audit 및 Outbox 연구 범위

Issue #41은 `bluetape4k-javers` pattern이 Go audit/event milestone을 어떻게 형성해야
하는지 결정하는 0.7.0 research gate다. Go track은 JaVers object diff engine이나 JVM
framework integration이 아니라 audit/history와 event-boundary concept를 가져와야 한다.

## 소스 인벤토리

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

## 현재 Go Repository 증거

- `bluetape-go`에는 이미 Redis, Kafka, NATS, PostgreSQL/MySQL, MongoDB, AWS,
  testcontainers, eventual consistency testing dependency가 있지만 audit package는
  아직 없다.
- 0.10.0 placeholder는 이미 이 track이 JaVers에 의존하지 않는 Go audit/event package가
  되어야 한다고 말한다.
- Issue #46과 #56-#59가 이미 있으므로 #41은 또 다른 broad implementation issue를
  만들기보다 해당 issue들을 업데이트해야 한다.

## 순위

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
| Full event sourcing framework | Low | High | Non-goal for 0.10.0. |

## 구현

- #56은 core model을 정의해야 한다.
  aggregate id, aggregate type, revision, domain event, audit entry, snapshot
  metadata, author, occurred/recorded timestamps, idempotency key, and
  serialization compatibility rule이 포함된다.
- #57은 in-memory implementation과 reusable conformance test를 가진 repository 및
  history query interface를 정의해야 한다. History query는 model이 stable behavior를
  지원할 수 있는 범위에서 aggregate, type, revision/time range, newest/previous entry,
  metadata filter로 제한한다.
- #59는 #56/#57 이후 작은 runnable Go example을 추가해야 한다. In-memory 또는 file-backed
  fixture로 command-side persistence boundary, audit history query, optional publisher
  hook을 보여준다.

## 나중에 채택

- SQL persistence는 #100을 기다리거나 그 eventual repository boundary를 재사용해야 한다.
  SQL은 history query에 가장 자연스러운 durable audit source지만 schema와 migration
  story를 두 번 설계하면 안 된다.
- Kafka와 NATS adapter는 #58이 at-least-once delivery, idempotency, retry/dead-letter,
  ordering, serialization, application-owned responsibility를 정의할 때까지 기다린다.
- Redis는 replay, ordering, head/restoration semantic이 명시된 뒤에야 projection 또는
  direct audit store로 검토할 수 있다.

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

## 필요한 Issue 업데이트

- #46: record the issue #41 decision and keep the epic framed as audit/history
  support, not event sourcing or JaVers clone work.
- #56: keep as the first implementation issue for core model contracts and
  serialization expectations.
- #57: keep as repository/history query interface plus in-memory conformance;
  defer SQL/Redis/Kafka concrete stores.
- #58: keep as outbox/publisher research; prefer SQL outbox boundary first and
  treat Kafka/NATS/Redis as adapters after delivery semantics are explicit.
- #59: keep as runnable example after #56/#57; avoid framework parity claims.

## 검증 계획

- Documentation-only PR에서는 `git diff --check`와 targeted `rg`를 실행한다.
- #46과 #56-#59 issue body가 #41 research update를 포함하는지 확인한다.
- Go code change가 없으므로 이 PR에는 Go test가 필요하지 않다.

## 후속 권고

#56은 hidden diffing 없이 explicit audit/event record로 시작한다. 이후 SQL, Redis,
Kafka, NATS adapter를 고르기 전에 #57에서 in-memory conformance test로 repository
semantic을 증명한다.
