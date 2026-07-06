# Issue #405 Lesson: Audit Publisher Target

## Decision

The first audit publisher adapter should be a deterministic standard-library
test/discard publisher, not Kafka, NATS, or Redis Streams.

## Lessons

- Keep the first publisher slice focused on the `audit/sqloutbox.Publisher`
  contract: cancellation, duplicate attempts, retry handoff, and shutdown.
- Use broker adapters only after the contract and examples are stable.
- Workshop examples need deterministic outbox behavior before broker emulation.
- Transport packages must not become audit history stores or hide topology,
  retention, replay, and idempotency ownership.

## Follow-Ups

- #407 should implement the selected test/discard publisher helpers.
- Create transport-specific issues later only when a concrete workshop or
  application scenario chooses Kafka, NATS, or Redis Streams.
