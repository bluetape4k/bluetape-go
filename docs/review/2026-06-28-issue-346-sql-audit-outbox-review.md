# Issue #346 Review: SQL Audit Outbox

## Verdict

P0=0 P1=0

## Findings

- No P0/P1 evidence-backed defects found in the current implementation window.
- Transaction ownership remains caller-visible: store methods accept
  `sqlkit.Execer` or `sqlkit.Session`; no hidden source-write transaction is
  created.
- Delivery semantics are explicit at-least-once. `Relay` marks failed attempts
  for retry or dead-letter and does not promise exactly-once delivery.
- Stored audit JSON is bounded before decode through `Options.MaxEntryBytes`.
- PostgreSQL claim SQL uses `FOR UPDATE SKIP LOCKED`, claim leases, expired
  claim reclamation, and excludes later aggregate revisions while earlier
  revisions remain pending or claimed.
- Publish and failure marking require the current claim attempt, preventing
  stale workers from overwriting a reclaimed row.

## Residual Risks

- `CreateSchema` is intended for explicit setup and tests. Production migration
  rollout, table ownership, retention, and operator replay tooling remain
  application responsibilities.
- Failure text is bounded but not a general-purpose PII scrubber; callers must
  keep publisher errors redacted.
- Broker-specific publisher adapters are intentionally outside this PR.

## Evidence

- `go test -count=1 ./audit/sqloutbox`
- `make ci`
