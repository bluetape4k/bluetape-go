# Issue 58 Audit Outbox And Publisher Design

Issue #58은 #56 audit model과 #57 repository/history contract 뒤에 올 첫
durable audit outbox 방향을 결정한다.

## 결정

첫 concrete adapter target으로 SQL outbox store와 relay contract를 선택한다.
Implementation issue는 #346이다.

Kafka, NATS, Redis Streams, RabbitMQ, Redpanda, Pulsar는 이 slice에서
선택하지 않는다. 이들은 SQL outbox contract가 증명된 뒤 durable outbox에서
record를 소비해야 하는 publisher/projection adapter로 남긴다.

이 결정은 audit track을 다음과 맞춘다.

- #57: storage-neutral repository/history contracts and in-memory conformance.
- #100: runtime-first `database/sql`, visible SQL, PostgreSQL-first integration.
- #41: SQL is the likely durable boundary, while Kafka/NATS are delivery
  adapters and Redis is a projection or explicit audit store only after replay
  and head semantics are accepted.

## 최소 Contract 형태

첫 구현은 public surface를 작고 Go-shaped로 유지해야 한다. 정확한 package
name은 #346에서 정하되, contract에는 다음 role이 포함되어야 한다.

| Role | Responsibility |
|---|---|
| Outbox store | Enqueue validated `audit.Entry` values, claim pending records, mark published records, and mark retry/dead-letter failures. |
| Outbox record | Preserve event ID, idempotency key, aggregate type/id, revision, event type, occurred/recorded timestamps, schema version, payload bytes, attempt count, availability time, and redacted failure state. |
| Publisher | Publish one claimed outbox record with `context.Context`; duplicate publish attempts are possible. |
| Relay | Claim records, call the publisher, mark success/failure, and shut down deterministically on context cancellation. |

Contract는 broad event-sourcing framework behavior를 피해야 한다. Audit
history만 필요한 caller가 publisher나 relay를 필요로 하면 안 된다.

## Delivery Semantics

- Delivery는 at-least-once다. Exactly-once는 약속하지 않는다.
- Idempotency는 stable audit event ID와 `audit.DomainEvent`에 이미 있는
  caller-supplied idempotency key를 기반으로 한다.
- SQL uniqueness는 durable boundary에서 duplicate event ID와 duplicate
  idempotency key를 거부해야 한다.
- 단일 store와 relay가 보장할 수 있는 범위에서 per-aggregate revision
  ordering을 보존해야 한다. Global ordering guarantee는 노출하지 않는다.
- Failed publish는 attempt count, next availability, redacted last error를
  업데이트해야 한다. Poison record는 whole relay를 무기한 막지 말고
  dead-letter state로 이동해야 한다.
- Serialization은 기존 `audit.Entry` JSON validation contract를 사용한다.
  Adapter는 `audit.DecodeEntryJSON`을 호출하기 전에 untrusted bytes를
  제한해야 한다.
- Publisher adapter는 sensitive payload나 aggregate data를 유출하지 않을 때만
  stable metadata를 transport header로 매핑해야 한다.

## 첫 SQL Adapter 경계

#346은 기존 runtime-first SQL 방향을 기준으로 SQL outbox store와 relay를
구현해야 한다.

- Execution boundary는 `database/sql`을 사용한다.
- SQL을 숨기지 않으면서 boilerplate를 줄일 때만 `sqlkit` helper를 사용한다.
- PostgreSQL을 첫 real database integration anchor로 사용한다.
- Migration ownership은 application-visible로 유지한다. Package는 DDL
  guidance나 fixture를 제공할 수 있지만 hidden migration을 실행하면 안 된다.
- #346이 caller-supplied transaction/session hook을 명시적으로 도입하지 않는
  한, source write, audit repository append, outbox enqueue choreography는
  application-owned로 유지한다.

## 보류 Adapter

| Adapter | Decision | Rationale |
|---|---|---|
| Kafka publisher | Defer | SQL outbox claim과 retry/dead-letter state가 생긴 뒤가 적합하다. Kafka는 history query store가 되면 안 된다. |
| NATS publisher | Defer | Low-latency fanout에 유용하지만 durable outbox semantics가 먼저 필요하다. |
| Redis Streams | Defer | Replay/head semantics가 명확해진 뒤 projection 또는 stream fanout으로 더 적합하다. |
| RabbitMQ/Redpanda/Pulsar | Defer | 현재 audit requirement는 이 broker들을 선택하지 않는다. Demand 전 fixture scope를 넓히지 않는다. |
| Direct Redis audit store | Defer | Redis는 SQL write-behind가 아니라 explicit audit source 또는 projection이어야 한다. |

## 애플리케이션 소유 책임

Application은 계속 다음을 소유한다.

- Source-of-truth row/document persistence.
- Source write, audit repository append, outbox enqueue 전반의 transaction
  boundaries.
- Schema migrations와 database permissions.
- Publisher client lifecycle, authentication, TLS, topic/subject/stream
  topology, broker-specific retention.
- Payload redaction, PII policy, tenant isolation, maximum payload size.
- Consumer idempotency와 replay behavior.
- Observability labels와 alerting thresholds.

## 후속 이슈

- #346 implements the SQL audit outbox store and relay contract selected here.

#346이 durable outbox contract를 증명하거나 concrete example이 specific
transport를 요구하기 전까지 Kafka, NATS, Redis Stream implementation issue는
만들지 않는다.

## 검증 계획

- Documentation PR: `git diff --check` and targeted `rg` for linked issue and
  README references.
- Targeted audit tests: `go test -count=1 ./audit ./audit/audittest`.
- Race gate for current audit package: `go test -race -count=1 ./audit ./audit/audittest`.
