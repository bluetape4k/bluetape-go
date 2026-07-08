# Issue #438 Graph Neo4j/Memgraph Benchmark Evidence

Issue #438 adds measured benchmark evidence for the `graph/neo4j` adapter
surface. The change is benchmark-only: graph value types, Neo4j client
semantics, driver ownership, and Memgraph compatibility contracts are
unchanged.

## Artifacts

- Pure mapping/default benchmark:
  `docs/research/outputs/issue-438/graph-neo4j-mapping-bench.txt`
- Neo4j/Memgraph Testcontainers opt-in benchmark:
  `docs/research/outputs/issue-438/graph-neo4j-containers-bench.txt`
- Environment and Docker metadata:
  `docs/research/outputs/issue-438/environment.md`

## Commands

- Local/default:
  `go test -run '^$' -bench . -benchmem ./graph/neo4j`
- Neo4j/Memgraph Testcontainers, serial and opt-in:
  `BLUETAPE_GRAPH_NEO4J_BENCH=1 go test -p 1 -run '^$' -bench '^BenchmarkGraphNeo4jContainers' -benchtime=100x -benchmem ./graph/neo4j`

Container benchmarks require Docker plus `neo4j:5.26.0` and
`memgraph/memgraph:3.5.0`. Normal benchmark runs skip container rows unless
`BLUETAPE_GRAPH_NEO4J_BENCH=1` is set.

## Pure Mapping Rows

| Case | Result |
|---|---:|
| `BenchmarkVertexFromNode` | `198.3 ns/op`, `400 B/op`, `3 allocs/op` |
| `BenchmarkEdgeFromRelationship` | `128.3 ns/op`, `336 B/op`, `2 allocs/op` |
| `BenchmarkVerticesFromRecords` | `16971 ns/op`, `37696 B/op`, `201 allocs/op` |
| `BenchmarkEdgesFromRecords` | `14386 ns/op`, `41792 B/op`, `201 allocs/op` |

Interpretation: single-value mapping stays sub-microsecond and record-batch
mapping cost is dominated by graph value construction plus shallow property
copying.

## Container Read/Write Rows

| Runtime | Case | Result |
|---|---|---:|
| Neo4j `neo4j:5.26.0` | `WriteNode` | `6333185 ns/op`, `11820 B/op`, `228 allocs/op` |
| Neo4j `neo4j:5.26.0` | `WriteRelationship` | `3396142 ns/op`, `11799 B/op`, `226 allocs/op` |
| Neo4j `neo4j:5.26.0` | `ReadVertices/Small10` | `2727240 ns/op`, `46024 B/op`, `691 allocs/op` |
| Neo4j `neo4j:5.26.0` | `ReadVertices/Medium100` | `2478667 ns/op`, `271583 B/op`, `4921 allocs/op` |
| Neo4j `neo4j:5.26.0` | `ReadEdges/Small10` | `2451548 ns/op`, `47312 B/op`, `681 allocs/op` |
| Neo4j `neo4j:5.26.0` | `ReadEdges/Medium100` | `2454960 ns/op`, `285177 B/op`, `4821 allocs/op` |
| Neo4j `neo4j:5.26.0` | `ReadEmptyResult` | `1507954 ns/op`, `23192 B/op`, `255 allocs/op` |
| Neo4j `neo4j:5.26.0` | `WriteSyntaxError` | `1360984 ns/op`, `11009 B/op`, `212 allocs/op` |
| Memgraph `memgraph/memgraph:3.5.0` | `WriteNode` | `579592 ns/op`, `8403 B/op`, `180 allocs/op` |
| Memgraph `memgraph/memgraph:3.5.0` | `WriteRelationship` | `518462 ns/op`, `9821 B/op`, `207 allocs/op` |
| Memgraph `memgraph/memgraph:3.5.0` | `ReadVertices/Small10` | `594059 ns/op`, `36621 B/op`, `566 allocs/op` |
| Memgraph `memgraph/memgraph:3.5.0` | `ReadVertices/Medium100` | `824707 ns/op`, `203936 B/op`, `3986 allocs/op` |
| Memgraph `memgraph/memgraph:3.5.0` | `ReadEdges/Small10` | `574915 ns/op`, `36973 B/op`, `556 allocs/op` |
| Memgraph `memgraph/memgraph:3.5.0` | `ReadEdges/Medium100` | `840016 ns/op`, `206407 B/op`, `3886 allocs/op` |
| Memgraph `memgraph/memgraph:3.5.0` | `ReadEmptyResult` | `491485 ns/op`, `18204 B/op`, `185 allocs/op` |
| Memgraph `memgraph/memgraph:3.5.0` | `WriteSyntaxError` | `478611 ns/op`, `7976 B/op`, `165 allocs/op` |

Interpretation: this is a local Testcontainers snapshot, not a production
ranking between databases. The rows include caller-side parameter map and
bounded context setup, and are useful as regression evidence for the current
caller-owned driver adapter boundary and shared Cypher subset.

## Decision

No optimization issue is opened from this run. The benchmark gap is closed, and
the measured rows do not show a correctness or public API problem that warrants
changing `Client`, adding a backend-neutral repository abstraction, or adding a
dedicated `graph/memgraph` package.
