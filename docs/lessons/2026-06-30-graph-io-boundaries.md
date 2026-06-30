# Graph I/O Boundaries

Date: 2026-06-30
Issue: #49
Milestone: 0.10.0

## Lesson

Graph import/export should start at bounded record streams, not at filesystem
or backend ownership. NDJSON and paired CSV are enough to prove vertex/edge
interchange behavior while keeping GraphML, compression, encryption, atomic
file replacement, repository/session APIs, and traversal semantics out of the
first public contract.

## Applied Contract

- `graph/graphio` writes vertices before edges in finite helpers.
- Streaming CSV readers require callers to consume vertices before edges.
- Readers fail closed by default for duplicate vertices and missing endpoints.
- CSV record byte limits are enforced before `encoding/csv` parses the logical
  record.
- CSV formula escaping is the default for caller-facing exports; raw output is
  explicit.

## Follow-up

Backend adapter evaluation (#50) and domain examples (#51) should reuse the
record contracts directly instead of introducing repository, session, or schema
contracts prematurely.
