# Issue #438 Graph Neo4j/Memgraph Benchmark Review

Issue: #438
Date: 2026-07-09
Scope: benchmark-only changes for `graph/neo4j` mapping and
Testcontainers-backed Neo4j/Memgraph read/write adapter paths.

## Findings

| Severity | Finding | Evidence |
|---|---|---|
| P0 | None | Production `graph/neo4j` files are untouched. |
| P1 | None | Neo4j/Memgraph container benchmarks are opt-in via `BLUETAPE_GRAPH_NEO4J_BENCH=1`, keeping default benchmark runs Docker-free. |
| P2 | None | Raw benchmark outputs and environment metadata are preserved under `docs/research/outputs/issue-438/`. |

## Lens Check

| Lens | Verdict | Evidence |
|---|---|---|
| Performance | Pass | Pure mapping plus Neo4j/Memgraph create/read/error rows cover the requested small and medium workloads. |
| Stability | Pass | `go test -count=1 ./graph/neo4j`, default benchmark, and opt-in serial container benchmark passed. |
| Security | Pass | Benchmark artifacts contain no connection secrets, Cypher parameters with sensitive values, or property payload dumps. |
| Operator/Ops | Pass | Container rows name Docker/Testcontainers requirements, image versions, env gate, serial command, and bounded per-operation context. |
| Developer/API | Pass | No public graph or Neo4j adapter API changes. |
| User/Caller | Pass | README documents default and opt-in benchmark commands without surprising default Docker startup. |

Final verdict: PASS. P0=0 P1=0.
