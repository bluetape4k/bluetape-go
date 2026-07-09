# Issue #491 GraphML Graph I/O Review

Date: 2026-07-10

Scope:

- `graph/graphio/graphml`
- `graph/graphio` and `graph` README updates
- root README and changelog references

## Evidence

- Issue #491 requires an optional bounded GraphML import/export slice based on
  #433 research.
- Implementation keeps GraphML in `graph/graphio/graphml` and leaves existing
  NDJSON/CSV APIs unchanged.
- Tests cover round-trip export/import, a namespaced producer-style subset,
  malformed XML, duplicate IDs, missing endpoints, unknown keys, oversized
  input, context cancellation, and unsupported constructs.

## 7-Tier Lanes

| Lane | Verdict | Notes |
|---|---|---|
| Performance | Pass | Reads are bounded by `MaxInputBytes`; writes are finite-record exports with deterministic key and record ordering. |
| Stability | Pass | Unsupported XML constructs fail closed before graph conversion; duplicate and endpoint invariants remain explicit. |
| Security | Pass | DOCTYPE/directives, non-declaration processing instructions, nested data payloads, and extension elements are rejected. |
| Operator/Ops | Pass | No filesystem, compression, backend, or runtime service ownership added. |
| Developer/API | Pass | Optional subpackage avoids changing core `graphio` NDJSON/CSV APIs and reuses `graphio.Record`, reports, and errors. |
| User/Caller | Pass | Bilingual docs state supported subset, non-goals, input limits, and exact test commands. |
| Integration | Pass | Acceptance commands target `./graph ./graph/graphio/...` and `./graph/graphio/...` race coverage. |

## Findings

- P0: 0
- P1: 0

## Residual Risk

The named-producer evidence is limited to a documented, namespaced structural
fixture. Any stronger Gephi, NetworkX, Neo4j APOC, or yEd compatibility claim
needs producer-version fixtures in a follow-up.
