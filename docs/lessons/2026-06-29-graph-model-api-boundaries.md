# Graph Model API Boundaries

Date: 2026-06-29
Issue: #48
Milestone: 0.10.0

## Decision

The first `graph` package is a model-only package. It defines validated graph
values for vertices, edges, paths, labels, IDs, shallow properties, and JSON
round trips, but it deliberately avoids repository, session, schema, query,
transaction, backend, algorithm, and capability interfaces.

## Why

The 0.10.0 graph milestone still needs proof from #49 graph I/O helpers, #50
backend adapter evaluation, and #51 domain examples before broader contracts can
be made stable. Adding those abstractions in #48 would freeze guesses before the
shared behavior is known.

## API Boundaries

- Edge endpoints use named structs so the directed roles are visible at the
  callsite.
- `PathStep` constructors validate vertex and edge values so invalid step shapes
  are not silently created through public helpers.
- `Path` validates step values and aggregate weight only. It does not validate
  endpoint continuity or traversal correctness; later algorithms and adapters
  own those invariants.
- Struct fields stay unexported and accessors return shallow defensive copies.
- `Properties` copies only the map boundary. Nested mutable values remain
  caller-owned and must be copied or sanitized by future I/O/backend adapters
  before trust boundaries.
- `ValidationError` keeps typed category, field, redacted summary, and cause. It
  must not store raw values.
- `ErrUnsupportedCapability` is reserved for future capability boundaries and is
  not returned by #48 constructors.

## Verification Notes

The implementation must keep `go test -count=1 ./graph`,
`go test -race -count=1 ./graph`, `go doc ./graph`, README parity, and Step 6-R
P0/P1 gates green before PR creation.
