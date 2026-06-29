# Issue 58 Audit Outbox And Publisher Design

Issue #58 decides the first durable audit outbox direction after the #56 audit
model and #57 repository/history contracts.

## Decision

Select a SQL outbox store and relay contract as the first concrete adapter
target. The implementation issue is #346.

Kafka, NATS, Redis Streams, RabbitMQ, Redpanda, and Pulsar are not selected for
this slice. They remain publisher/projection adapters that should consume
records from a durable outbox after the SQL outbox contract is proven.

This keeps the audit track aligned with:

- #57: storage-neutral repository/history contracts and in-memory conformance.
- #100: runtime-first `database/sql`, visible SQL, PostgreSQL-first integration.
- #41: SQL is the likely durable boundary, while Kafka/NATS are delivery
  adapters and Redis is a projection or explicit audit store only after replay
  and head semantics are accepted.

## Minimal Contract Shape

The first implementation should keep the public surface small and Go-shaped.
The exact package name belongs to #346, but the contract should include these
roles:

| Role | Responsibility |
|---|---|
| Outbox store | Enqueue validated `audit.Entry` values, claim pending records, mark published records, and mark retry/dead-letter failures. |
| Outbox record | Preserve event ID, idempotency key, aggregate type/id, revision, event type, occurred/recorded timestamps, schema version, payload bytes, attempt count, availability time, and redacted failure state. |
| Publisher | Publish one claimed outbox record with `context.Context`; duplicate publish attempts are possible. |
| Relay | Claim records, call the publisher, mark success/failure, and shut down deterministically on context cancellation. |

The contract should avoid broad event-sourcing framework behavior. A caller that
only needs audit history should not need a publisher or relay.

## Delivery Semantics

- Delivery is at-least-once. Exactly-once is not promised.
- Idempotency is based on the stable audit event ID plus the caller-supplied
  idempotency key already present in `audit.DomainEvent`.
- SQL uniqueness should reject duplicate event IDs and duplicate idempotency
  keys at the durable boundary.
- Per-aggregate revision ordering should be preserved where a single store and
  relay can do so. No global ordering guarantee should be exposed.
- Failed publishes should update attempt count, next availability, and a
  redacted last error. Poison records should move to dead-letter state instead
  of blocking the whole relay indefinitely.
- Serialization should use the existing `audit.Entry` JSON validation contract.
  Adapters must bound untrusted bytes before calling `audit.DecodeEntryJSON`.
- Publisher adapters should map stable metadata to transport headers only when
  doing so does not leak sensitive payload or aggregate data.

## First SQL Adapter Boundary

#346 should implement a SQL outbox store and relay against the existing
runtime-first SQL direction.

- Use `database/sql` as the execution boundary.
- Use `sqlkit` helpers where they reduce boilerplate without hiding SQL.
- Use PostgreSQL as the first real database integration anchor.
- Keep migration ownership application-visible. The package can provide DDL
  guidance or fixtures, but should not run hidden migrations.
- Keep source write, audit repository append, and outbox enqueue choreography
  application-owned unless #346 explicitly introduces a caller-supplied
  transaction/session hook.

## Deferred Adapters

| Adapter | Decision | Rationale |
|---|---|---|
| Kafka publisher | Defer | Best after SQL outbox claims and retry/dead-letter state exist. Kafka should not become the history query store. |
| NATS publisher | Defer | Useful for low-latency fanout, but still needs durable outbox semantics first. |
| Redis Streams | Defer | Better as projection or stream fanout after replay/head semantics are explicit. |
| RabbitMQ/Redpanda/Pulsar | Defer | No current audit requirement selects these brokers; do not expand fixture scope before demand. |
| Direct Redis audit store | Defer | Redis must be an explicit audit source or projection, not SQL write-behind. |

## Application-Owned Responsibilities

Applications still own:

- Source-of-truth row/document persistence.
- Transaction boundaries across source writes, audit repository append, and
  outbox enqueue.
- Schema migrations and database permissions.
- Publisher client lifecycle, authentication, TLS, topic/subject/stream
  topology, and broker-specific retention.
- Payload redaction, PII policy, tenant isolation, and maximum payload size.
- Consumer idempotency and replay behavior.
- Observability labels and alerting thresholds.

## Follow-Up Issue

- #346 implements the SQL audit outbox store and relay contract selected here.

Do not create Kafka, NATS, or Redis Stream implementation issues until #346
proves the durable outbox contract or a concrete example requires a specific
transport.

## Validation Plan

- Documentation PR: `git diff --check` and targeted `rg` for linked issue and
  README references.
- Targeted audit tests: `go test -count=1 ./audit ./audit/audittest`.
- Race gate for current audit package: `go test -race -count=1 ./audit ./audit/audittest`.
