Closes #346.

## Summary

- add `audit/sqloutbox` with PostgreSQL-backed enqueue, claim lease,
  claim-attempt-guarded publish/failure marks, retry, dead-letter, and relay
  APIs
- keep transaction ownership explicit by accepting caller-supplied
  `database/sql` sessions
- document at-least-once delivery, per-aggregate claim ordering, idempotency,
  migration, redaction, and operator boundaries

## Validation

- `go test -count=1 ./audit/sqloutbox`
- `make ci`

## DoD Status

- [x] Issue #346 scope implemented.
- [x] PostgreSQL Testcontainers coverage added for store, claim lease, stale mark rejection, and relay behavior.
- [x] Stress/cancellation helpers used for concurrent claim and relay lifecycle.
- [x] README, changelog, spec, plan, review, lesson, and PR artifacts updated.
- [x] P0=0 P1=0 review recorded.
- [x] Local CI passed.
