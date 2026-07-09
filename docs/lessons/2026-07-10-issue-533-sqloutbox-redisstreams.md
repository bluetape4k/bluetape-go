# Issue 533 - Redis Streams sqloutbox publisher

## Context

Issue #533 adds the first broker-backed `audit/sqloutbox.Publisher` provider.
Prior audit research selected SQL outbox as the durable source of truth and
deferred Redis Streams until the `Publisher` contract was stable.

## Decision

- Add `audit/sqloutbox/redisstreams` as a narrow provider package.
- Accept a caller-owned Redis client through a small `XAdd` interface instead
  of a generic message-bus abstraction.
- Publish one Redis stream entry per sqloutbox publish attempt.
- Preserve `Record.EventID`, `Record.IdempotencyKey`, aggregate identity, event
  type, schema version, attempt count, and full `entry_json` payload.
- Leave stream trimming, retention, consumer groups, replay, auth, TLS, and
  topology to callers.

## Validation Notes

- Unit tests use a fake `XAdd` client to prove field preservation,
  cancellation short-circuiting, and Redis error propagation.
- Testcontainers Redis tests prove real stream append and duplicate attempts.
- A relay integration test combines PostgreSQL sqloutbox and Redis Streams to
  prove non-cancellation publish errors surface to relay retry behavior and the
  successful retry carries `attempts=2`.

## Follow-up Guardrails

- Future Kafka, NATS, RabbitMQ, Redpanda, or Pulsar providers should keep the
  same provider-local shape instead of introducing a shared broker abstraction
  before repeated call-site evidence exists.
- Redis Streams remains a transport provider, not an audit storage backend.
- If caller-managed stream retention becomes a repeated problem, research that
  as an explicit options/operations issue before adding trimming flags to this
  publisher.
