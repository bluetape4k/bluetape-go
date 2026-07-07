# Issue #407 Audit Publisher Adapter

Issue: #407
Milestone: 0.15.0
Decision date: 2026-07-07

## Decision

Implement the first audit publisher adapter as
`audit/sqloutbox/sqloutboxtest`, a deterministic helper package for tests,
local examples, and workshop adoption.

This is intentionally not a durable broker adapter. The package implements the
existing `sqloutbox.Publisher` contract with:

- `DiscardPublisher` for store/relay tests that do not assert published output.
- `PublisherFunc` for small function adapters.
- `RecordingPublisher` for attempt-order assertions and deterministic
  per-event failure injection.

## Source Context

The prior #405 research selected the first adapter direction after comparing
the audit outbox contract with broader broker candidates. The #406 relay
contract then pinned the runtime behavior:

- At-least-once delivery.
- Caller-owned context cancellation must not become retry/dead-letter state.
- Non-cancellation publish errors are persisted as bounded failure text.
- Duplicate publish attempts are allowed and must preserve `Record.EventID` and
  `Record.IdempotencyKey`.
- Durable broker topology, authentication, retention, replay, redaction, and
  consumer idempotency remain caller or later-adapter responsibilities.

## Candidate Modules

| Candidate | Decision | Reason |
|---|---|---|
| `audit/sqloutbox/sqloutboxtest` | Selected | Narrow package that implements the current `Publisher` interface without implying durable transport support. |
| `audit/sqloutbox/publisher` | Rejected | Too generic; reads like production adapter surface instead of test/example support. |
| `audit/publisher` | Rejected | Too broad; would blur storage-neutral audit values with SQL outbox relay semantics. |
| `audit/sqloutbox/kafka` or similar broker package | Deferred | Requires topology, auth/TLS, retry/replay, idempotency, and operator contract beyond #407. |

## Diagram Decision

No new README diagram is required for this package. The helper adds no new
runtime topology or sequence beyond the existing `sqloutbox.Publisher`
participant. The source-backed reader question is answered by README prose plus
the existing `audit/sqloutbox` class contract and relay sequence diagrams.

## Test Contract

The implementation should prove:

- Context cancellation and nil helper surfaces are bounded.
- `PublisherFunc` preserves function-owned behavior.
- `RecordingPublisher` returns defensive snapshots.
- Failure injection is deterministic across retry attempts.
- Concurrent `Publish` calls remain race-free under `GoroutineStressTester`.
- `sqloutbox.Relay` can drive retry and dead-letter behavior through the helper
  with PostgreSQL Testcontainers.
