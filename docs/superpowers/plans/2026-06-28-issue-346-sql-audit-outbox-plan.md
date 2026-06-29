# Issue #346 Plan: SQL Audit Outbox

## Steps

1. Add failing PostgreSQL-backed tests for store and relay behavior.
2. Implement `audit/sqloutbox` with explicit `sqlkit.Session` boundaries.
3. Add visible PostgreSQL DDL and `FOR UPDATE SKIP LOCKED` claim SQL.
4. Implement retry/dead-letter transitions and a cancellable relay loop.
5. Document public boundaries in package, audit, root, and changelog docs.
6. Run targeted package tests, race tests, formatting, vet, and local CI where
   practical.

## Design Decisions

- `Store.Enqueue` accepts `sqlkit.Execer` so callers can pass `*sql.Tx` and own
  source-write coupling.
- `Store.Claim` accepts `sqlkit.Session` because claiming needs query and update
  behavior in one statement.
- Claim SQL sets a bounded lease, can reclaim expired claimed rows, and excludes
  later revisions while lower revisions for the same aggregate are still pending
  or claimed.
- Publish/failure marking checks the current claim attempt so stale workers do
  not mutate reclaimed rows.
- `RunOnce` supports scheduler-owned polling; `Run` supports service-owned
  worker lifecycle with context cancellation.

## Risk Controls

- Bound stored `entry_json` before calling `audit.DecodeEntryJSON`.
- Keep failure state to bounded text and avoid storing payload copies as error
  metadata.
- Leave migrations explicit through optional `CreateSchema`; production rollout
  remains application-owned.
