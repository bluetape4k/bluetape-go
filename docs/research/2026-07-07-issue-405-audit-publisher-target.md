# Issue #405 Audit Publisher Adapter Target

Issue #405 chooses the first audit publisher adapter target for the 0.15.0
audit publisher track.

## Decision

Select a standard-library, deterministic test/discard publisher package as the
first adapter slice.

The follow-up implementation issue (#407) should stay narrow:

- add a small audit publisher helper package, with the exact path finalized in
  #406, that implements the existing `audit/sqloutbox.Publisher` boundary
  without a broker dependency;
- include a discard publisher, a function adapter, and a recording publisher
  for unit tests and runnable examples;
- prove context cancellation, duplicate publish attempts, failure injection,
  retry/dead-letter handoff through `sqloutbox.Relay`, and deterministic
  shutdown behavior;
- document that the package is for contract tests, local examples, and workshop
  adoption, not durable transport.

Kafka, NATS, and Redis Streams remain later transport adapters. They should
consume records from `audit/sqloutbox` after the publisher contract and example
shape are stable.

## Evidence

- `docs/research/2026-06-27-issue-58-audit-outbox-design.md` selected the SQL
  outbox store and relay as the first durable audit publisher boundary. It
  explicitly deferred Kafka, NATS, and Redis Streams until the durable outbox
  contract was proven.
- `audit/sqloutbox` now provides the proven relay boundary: `Publisher`
  publishes one claimed `Record`, `RunOnce` marks publish success/failure, and
  `Run` owns cancellation-aware worker looping.
- `audit/sqloutbox/README.md` documents at-least-once delivery, duplicate
  publish attempts, stable event ID/idempotency-key dedupe, retry state, and
  caller-owned broker topology.
- `examples/audit/README.md` keeps the current example framework-free and says
  production code can adapt the example `EntrySink` to `audit/sqloutbox`.
- `bluetape-go-workshop`
  `docs/superpowers/research/2026-06-23-issue-48-audit-aws-sql-candidates-research.md`
  says durable outbox behavior should stay deterministic first and Kafka-backed
  publisher tests should be deferred unless the upstream package requires them.

## Candidate Comparison

| Candidate | Decision | Rationale |
|---|---|---|
| Standard-library test/discard publisher | Select first | Proves the publisher contract without adding broker lifecycle, Docker cost, or topology policy. It gives #406/#407 executable examples for retry, duplicate, cancellation, and shutdown behavior in one PR. |
| Kafka publisher | Defer | Strong eventual transport fit for ordered event fanout, but heavier startup, partition/topic policy, producer ack configuration, schema/header choices, and workshop Testcontainers cost would obscure the first contract. |
| NATS publisher | Defer | Lower fixture cost and useful fanout semantics, but subject naming, JetStream versus core NATS durability, ack behavior, and replay policy need a settled publisher envelope first. |
| Redis Streams publisher | Defer | Redis is already common in the repo, but stream trimming, consumer groups, pending-entry recovery, and projection versus durable audit semantics are easy to overclaim before the contract is stable. |

## Contract Consequences

- The first implementation should not introduce a generic message-bus
  abstraction.
- The first implementation should depend only on the existing `sqloutbox.Record`
  shape and `context.Context`.
- Transport adapters must not become audit history stores.
- Broker-specific packages must document topology, retention, authentication,
  TLS, consumer idempotency, and replay/poison-message ownership when they are
  eventually added.

## Rejected For This Slice

- Do not choose Kafka merely because the Kotlin workshop has a transactional
  outbox example. The Go workshop notes explicitly keep broker emulation behind
  deterministic outbox behavior.
- Do not choose Redis Streams because Redis fixtures already exist. Redis stream
  semantics are projection/fanout semantics unless a later issue selects Redis
  as an explicit audit source.
- Do not choose NATS only to reduce fixture cost. A low-cost broker still adds
  subject and delivery semantics before #406 has stabilized the contract.

## Follow-Up Scope

#407 should implement the selected standard-library publisher helpers first.
After #407 and #408 prove the public docs and examples, create separate,
transport-specific issues only when a workshop or application scenario needs a
real broker.

## Validation Plan

- Documentation PR: `git diff --check`.
- Traceability: `rg -n "Issue #405|test/discard publisher|Kafka publisher|Redis Streams publisher" docs/research docs/lessons`.
- No code behavior changes in this slice; package tests are not required until
  #407.
