# Issue 58 Audit Outbox Lessons

## Context

Issue #58 follows the #56 audit model and #57 repository/history contract. It
decides which outbox/publisher adapter should come first.

## Lessons

- Pick the durable outbox boundary before choosing brokers. Kafka, NATS, Redis
  Streams, RabbitMQ, Redpanda, and Pulsar are delivery/projection choices, not
  replacements for durable audit outbox state.
- The first outbox implementation should be SQL because #100 already established
  a runtime-first `database/sql` and PostgreSQL-first direction, and #41 ranked
  SQL as the likely durable history/outbox boundary.
- At-least-once delivery and duplicate publish attempts must be first-class
  contracts. Event ID and idempotency key uniqueness are the durable dedupe
  handles; exactly-once delivery remains out of scope.
- Keep transaction choreography application-owned unless an implementation issue
  explicitly accepts caller-supplied transaction/session hooks.
- Outbox relay code is concurrency-sensitive. When #346 adds async relay loops,
  it needs cancellation, retry/dead-letter, stress, and race coverage.
