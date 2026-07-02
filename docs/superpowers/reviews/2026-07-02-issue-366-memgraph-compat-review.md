# Issue #366 Memgraph Compatibility Review

## Scope

- Diff base: `origin/develop`.
- Module slice: `graph/neo4j` Memgraph compatibility test plus README pair and
  design/review notes.
- Review mode: main-session six-lane review. Native subagents were not spawned
  because this Codex surface only spawns subagents on explicit user request.

## Six-Lane Findings

| Lane | Reviewed Evidence | P0 | P1 | P2 | P3 | Verdict |
|---|---|---:|---:|---:|---:|---|
| Performance | Compatibility test keeps one Memgraph container and one small graph fixture | 0 | 0 | 0 | 0 | PASS |
| Stability | Memgraph matrix covers create/read, mapping, bad query, cancellation, and driver close | 0 | 0 | 0 | 0 | PASS |
| Security | No new auth or credential abstraction; Memgraph test uses local no-auth container only | 0 | 0 | 0 | 0 | PASS |
| Operator/Ops | Generic container path used because no Testcontainers Go Memgraph module exists; image pinned to `memgraph/memgraph:3.5.0` | 0 | 0 | 0 | 0 | PASS |
| Developer/API | No `graph/memgraph` package added; existing `graph/neo4j` surface remains the single compatibility target | 0 | 0 | 0 | 0 | PASS |
| User/Caller | README/README.ko explain compatibility matrix, guardrails, and deferred standalone backend abstraction | 0 | 0 | 0 | 0 | PASS |

## Integration Verdict

P0 = 0, P1 = 0.

Memgraph compatibility is proven as coverage around the Neo4j-driver adapter
path. No separate Memgraph abstraction is justified by the current evidence.

## Validation

```bash
make fmt-check
make tidy-check
make vet
make lint
go test -run TestClientMemgraphCompatibilityWithGenericContainer -count=1 ./graph/neo4j
go test -p 1 -count=1 ./graph/neo4j
go test -p 1 -race -count=1 ./graph/neo4j
make test
```

All commands above passed locally. `golangci-lint cache clean` was run before
the final lint pass because the first lint run reported stale diagnostics from
a deleted previous worktree.
