# Issue #406 Audit Publisher Contract

Issue #406 stabilizes the `audit/sqloutbox.Publisher` boundary before the
first helper adapter package in #407.

## Decision

Keep the publisher boundary on `audit/sqloutbox.Record` and
`context.Context`. Do not introduce a generic broker abstraction in this slice.

The contract is:

- delivery is at-least-once, so publishers and consumers must handle duplicate
  publish attempts;
- `Record.EventID` and `Record.IdempotencyKey` are the stable deduplication
  keys across retries and expired claim leases;
- caller-owned context cancellation or deadline errors stop `RunOnce` without
  calling `MarkFailed`;
- non-cancellation publish errors are persisted through retry/dead-letter state
  with bounded failure text;
- broker topology, authentication, TLS, retention, replay, logging, metrics,
  and redaction remain adapter/application responsibilities.

## Implementation Scope

- Expanded the public `Publisher` doc comment with at-least-once,
  idempotency, and caller cancellation rules.
- Updated `Relay.RunOnce` to return wrapped caller context cancellation errors
  directly instead of storing them as retry/dead-letter failures.
- Added cancellation, duplicate retry envelope, and concurrent `RunOnce`
  stress tests over the PostgreSQL-backed store.
- Updated `audit/sqloutbox` README files and the parent `audit` README files in
  English and Korean.

## Diagram Decision

No new README diagram is required for #406.

Evidence:

- The existing class contract map already shows the public participants:
  source caller, `Store`, `Relay`, `Publisher`, and `Record`.
- The existing sequence diagram already shows claim, publish error,
  retry/dead-letter, and publish success branches.
- #406 changes the cancellation exception and duplicate/idempotency wording,
  but it does not add a new public type, transport participant, or sequence
  branch that cannot be explained by the existing retry/error branch plus README
  prose.
- Full-size PNG inspection was performed for both existing diagrams and found
  no overlap, truncation, or readability defects.

## Validation Plan

- `go test -count=1 ./audit/sqloutbox`
- `go test -race -count=1 ./audit/sqloutbox`
- `git diff --check`
- 7-Tier review artifact with P0/P1 findings before PR.
