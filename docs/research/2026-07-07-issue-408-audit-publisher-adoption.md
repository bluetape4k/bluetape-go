# Issue #408 Audit Publisher Adoption

Issue: #408
Milestone: 0.15.0
Decision date: 2026-07-07

## Decision

Document audit publisher adoption in the existing audit example instead of
adding a new framework-style example API.

The example still owns only:

- command-side `audit.Entry` creation,
- explicit `audit.Repository` append before source-state mutation,
- history queries through the repository boundary, and
- replay into an `EntrySink`.

Production adoption is documented as an explicit handoff:

1. Caller-owned source writes and `audit/sqloutbox.Store.Enqueue` share the
   same `*sql.Tx`.
2. `sqloutbox.NewRelay` creates the relay.
3. `Relay.RunOnce` or `Relay.Run` claims durable rows.
4. `sqloutbox.Publisher.Publish` receives each `sqloutbox.Record`.
5. Publisher adapters preserve `Record.EventID` and `Record.IdempotencyKey` for
   duplicate-safe downstream consumers.

## Source Checks

The README names are source-checked against:

- `audit/sqloutbox/store.go`: `Store.Enqueue`, `Record.EventID`,
  `Record.IdempotencyKey`.
- `audit/sqloutbox/relay.go`: `Publisher`, `NewRelay`, `Relay.RunOnce`,
  `Relay.Run`.
- `audit/sqloutbox/sqloutboxtest/publisher.go`: `RecordingPublisher`,
  `NewRecordingPublisher`, `WithFailures`.

## Operator Contract

The documentation must keep the operational contract visible:

- Delivery is at-least-once.
- Non-cancellation publish errors drive retry/dead-letter state.
- Context cancellation and deadlines stop worker lifecycle without mutating
  retry/dead-letter state.
- Persisted failure text is operator-facing, so publisher errors must be
  bounded and redacted.
- Duplicate delivery is expected; consumers own idempotency.

## Diagram Decision

Update the existing `audit-example-service-flow` diagram rather than adding a
second package-local diagram. The existing image already owns the example
service narrative; #408 only extends the adoption path from durable outbox rows
to relay, publisher adapter, and downstream deduplication.

## Cross-Repo Follow-Up

Workshop-facing runnable coverage is tracked in
[bluetape-go-workshop#57](https://github.com/bluetape4k/bluetape-go-workshop/issues/57).
