# Issue #346 Spec: SQL Audit Outbox

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

## 범위

- Repository: `bluetape4k/bluetape-go`
- Issue: #346 `Implement SQL audit outbox store and relay contract`
- Milestone: `0.9.0`

## 요구사항

- Add a minimal Go-shaped SQL outbox package for `audit.Entry` values.
- Keep application-owned transaction choreography explicit by accepting caller
  supplied `database/sql` sessions.
- Use PostgreSQL as the first concrete SQL target with inspectable SQL.
- Persist event ID, idempotency key, aggregate identity, revision, event type,
  timestamps, schema version, bounded entry JSON, attempts, retry/dead-letter
  state, and bounded failure text.
- Decode stored audit entries only after enforcing a byte limit.
- Provide a relay surface that supports one-shot polling and context-cancelled
  worker execution.
- Use bounded claim leases so rows claimed by a crashed relay can be reclaimed.
- Require the current claim attempt for publish/failure marking so stale relays
  cannot overwrite reclaimed rows.
- Promise at-least-once delivery only; duplicate publish attempts are expected.

## Non-Goals

- Kafka, NATS, Redis Streams, RabbitMQ, Redpanda, Pulsar, or direct Redis audit
  adapters.
- Hidden migrations or ORM adoption.
- Repository history reads or `audit.Repository` conformance.
- Exactly-once delivery.

## Acceptance

- PostgreSQL Testcontainers tests cover enqueue, claim, claim lease expiry,
  stale claim mark rejection, mark published, retry, dead-letter, duplicate
  IDs, relay failure handling, cancellation, and concurrent claim safety.
- Package docs state caller-owned transaction, migration, redaction, and
  idempotency responsibilities.
- Root and audit README links include `audit/sqloutbox`.
