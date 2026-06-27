# Issue #346 Spec: SQL Audit Outbox

## Scope

- Repository: `bluetape4k/bluetape-go`
- Issue: #346 `Implement SQL audit outbox store and relay contract`
- Milestone: `0.9.0`

## Requirements

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
