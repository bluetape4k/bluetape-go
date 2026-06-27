Closes #58.

## Why

The audit package now has model and repository/history contracts, but outbox
publisher work still needed a durable boundary decision before adding Kafka,
NATS, Redis Stream, or SQL adapter code.

## What Changed

- Select SQL outbox store plus relay contracts as the first concrete durable
  publisher target.
- Create follow-up implementation issue #346 for the SQL audit outbox store and
  relay contract.
- Document at-least-once delivery, event ID/idempotency key dedupe, retry and
  dead-letter semantics, per-aggregate ordering limits, serialization, and
  application-owned transaction responsibilities.
- Update audit README files, research index, WIP, CHANGELOG, review, and
  lessons artifacts.

## Deferred

- Kafka, NATS, Redis Streams, RabbitMQ, Redpanda, Pulsar, and direct Redis audit
  storage remain deferred until #346 proves the durable outbox contract or a
  concrete example requires a transport adapter.

## Validation

- `git diff --check`
- `rg -n '#346|2026-06-27-issue-58-audit-outbox-design|P0=0 P1=0|SQL outbox' ...`
- `go test -count=1 ./audit ./audit/audittest`
- `go test -race -count=1 ./audit ./audit/audittest`

## Review

- 7-Tier local review artifact:
  `docs/review/2026-06-27-issue-58-audit-outbox-design-review.md`
- Verdict: `P0=0 P1=0`

## DoD Status

| Step | Status | Evidence |
|---|---|---|
| Merge previous PR | PASS | #345 merged as `de878226904b8b83ec3a4678478983ce81d41c50`; root develop synced before #58 worktree. |
| Route and discover | PASS | #58 acceptance, #41 audit scope, #57 lessons, #100 SQL boundary, audit README, and current 0.9.0 milestone issues reviewed. |
| Follow-up issue | PASS | Created #346 with milestone `0.9.0`, assignee `debop`, labels `type: task`, `priority: p1`, `area: audit`. |
| Design artifacts | PASS | Added `docs/research/2026-06-27-issue-58-audit-outbox-design.md`, review, lesson, README/WIP/CHANGELOG/index updates. |
| Review gate | PASS | `docs/review/2026-06-27-issue-58-audit-outbox-design-review.md` records `P0=0 P1=0`. |
| Validation | PASS | `git diff --check`; targeted `rg`; `go test -count=1 ./audit ./audit/audittest`; `go test -race -count=1 ./audit ./audit/audittest`. |
