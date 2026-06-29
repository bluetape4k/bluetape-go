# Issue #346 Superpowers Review: SQL Audit Outbox

## Scope

- Code: `audit/sqloutbox`
- Docs: package README files, root README files, audit README files, changelog

## 7-Tier Summary

- Performance: P0=0 P1=0. Claim path is bounded by limit and indexed by status,
  availability, aggregate, revision, and ID.
- Stability: P0=0 P1=0. Contexts flow through every public operation; claim
  leases allow expired claimed rows to be reclaimed; relay cancellation is
  tested with `AsyncJobTester`; stale claim attempts cannot mark reclaimed rows.
- Security: P0=0 P1=0. Entry decode is byte-bounded; failure text is bounded.
  Caller-owned PII policy remains documented.
- Ops: P0=0 P1=0. DDL is visible through `CreateSchema`; hidden migrations are
  avoided.
- Developer/API: P0=0 P1=0. APIs accept `database/sql`-compatible interfaces and
  avoid ORM or broker dependencies.
- User/Caller: P0=0 P1=0. At-least-once semantics and idempotency boundaries are
  documented in README files.
- Integration: P0=0 P1=0. Targeted Testcontainers tests cover the new durable
  path without pretending to satisfy `audit.Repository` reads.

## Evidence

- `go test -count=1 ./audit/sqloutbox`
- `make ci`
