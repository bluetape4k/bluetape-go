# Issue #346 Lesson: SQL Audit Outbox

## Decision

The first durable audit publisher boundary is a PostgreSQL-backed outbox store
and relay in `audit/sqloutbox`.

## Lessons

- Keep source transaction ownership visible. Passing `*sql.Tx` or `*sql.DB`
  into store methods is clearer than hiding transaction hooks inside an outbox
  repository.
- Per-aggregate ordering needs claim-time protection, not just `order by`.
  Later revisions are excluded while earlier revisions remain pending or
  claimed.
- `RunOnce` and `Run` serve different owners: schedulers can poll one batch,
  while service workers can run until context cancellation.
- Retry/dead-letter state belongs in the outbox row; broker-specific adapter
  logic should come later.

## Follow-ups

- Add concrete broker publisher adapters only after the SQL outbox contract has
  held through examples.
- Keep retention, replay tooling, migration rollout, and PII policy as explicit
  application/operator responsibilities.
