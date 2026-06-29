## Summary

- Add the `audit` package with `AggregateID`, `Revision`, `DomainEvent`,
  `Entry`, `SnapshotMetadata`, `ChangeMetadata`, `AggregateRecorder`, and
  `History`.
- Validate constructor and JSON decode paths, including schema version,
  aggregate/revision consistency, missing snapshot payloads, metadata taxonomy,
  redacted validation errors, duplicate event/idempotency handling, and
  pending-event ack behavior.
- Add bilingual audit READMEs, root README entries, CHANGELOG/WIP updates,
  Step 6-R review evidence, and lessons.

## Boundaries

- Repository interfaces, history query storage, outbox publishers, SQL/Redis/
  Kafka/NATS adapters, and JaVers-style object diffing remain out of scope for
  #56.
- Recorder pending events are an in-process retry aid only. Later repository
  and outbox issues must provide durable transaction, rollback, outbox, or
  reconciliation semantics before acking events.

Closes #56

## Verification

- `go test -count=1 ./audit`
- `go test -race -count=1 ./audit`
- `go test -run '^$' -bench 'BenchmarkAggregateRecorder(Record|PendingEvents|AckThrough)' -benchmem ./audit`
- `make lint`
- `make ci`
- `git diff --check`
- Step 6-R review: `docs/superpowers/reviews/2026-06-27-issue-56-audit-model-step-6r-review.md`, `P0=0 P1=0`

## DoD Status

| Gate | Status | Evidence |
|---|---|---|
| Issue scope | PASS | Implements #56 model/recording/history basics and keeps repository/outbox/diffing out of scope. |
| Tests | PASS | Targeted audit tests, race test, benchmark command, and `make ci` passed. |
| Review | PASS | Step 6-R seven-tier review closed with `P0=0 P1=0` after fixing the snapshot payload P1. |
| Docs | PASS | `audit/README.md`, `audit/README.ko.md`, root README pair, CHANGELOG, WIP, review, and lesson updated. |
| PR metadata | PASS | Assignee, milestone, and labels mirror #56. |
