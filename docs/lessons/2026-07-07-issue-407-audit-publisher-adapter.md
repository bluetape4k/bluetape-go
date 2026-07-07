# Issue #407 Audit Publisher Adapter Lesson

Date: 2026-07-07
Scope: `audit/sqloutbox/sqloutboxtest`

## Lesson

For the first audit publisher adapter, a test/example helper package is the
safest first implementation. It exercises the `sqloutbox.Publisher` contract
without prematurely selecting Kafka, NATS, Redis Streams, RabbitMQ, Redpanda,
or Pulsar topology.

## Pattern

- Keep `audit` storage-neutral.
- Keep `audit/sqloutbox` responsible for PostgreSQL outbox state and relay
  semantics.
- Put deterministic publisher helpers under `audit/sqloutbox/sqloutboxtest`.
- Use `GoroutineStressTester` for concurrent helper state and a relay-backed
  Testcontainers test for retry/dead-letter handoff.
- Document when no new diagram is required: if a package only implements an
  existing participant and adds no new runtime sequence/topology, link the
  existing class/sequence diagrams instead of creating redundant art.

## Follow-up

Durable broker adapters should be separate packages with their own topology,
authentication/TLS, redaction, replay, idempotency, and operator runbook
contracts.
