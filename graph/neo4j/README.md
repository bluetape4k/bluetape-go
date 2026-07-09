# graph/neo4j

[English](README.md) | [한국어](README.ko.md)

Proof adapter from the official Neo4j Go driver to `graph.Vertex` and
`graph.Edge`.

This package is intentionally small. It adapts `dbtype.Node` and
`dbtype.Relationship` values from `github.com/neo4j/neo4j-go-driver/v6` and
adds minimal read/write helpers around a caller-owned driver. It does not define
a backend-neutral repository, session abstraction, schema DSL, transaction
manager, or Cypher DSL.

## Import

```go
import neo4jgraph "github.com/bluetape4k/bluetape-go/graph/neo4j"
```

## Usage

```go
driver, err := neo4j.NewDriver(uri, neo4j.NoAuth())
if err != nil {
	return err
}
client, err := neo4jgraph.NewClient(driver)
if err != nil {
	return err
}
defer client.Close(ctx)

if err := client.ExecuteWrite(ctx, `
CREATE (a:Service {name: $source})-[r:CALLS]->(b:Service {name: $target})
`, map[string]any{
	"source": "checkout",
	"target": "payments",
}); err != nil {
	return err
}

vertices, err := client.ReadVertices(ctx, `
MATCH (n:Service {name: $name})
RETURN n
`, map[string]any{"name": "checkout"}, "n")
if err != nil {
	return err
}
_ = vertices
```

## Diagram

![graph/neo4j adapter contract map](../../docs/images/readme-diagrams/graph-neo4j-adapter-contract-map.png)

The contract map shows the adapter boundary: `Client` wraps a caller-owned
Neo4j driver, reads one named result column, adapts Neo4j records into
`graph.Vertex` / `graph.Edge`, and keeps backend-neutral repository, schema, and
Cypher DSL contracts deferred.

![graph/neo4j read write sequence](../../docs/images/readme-diagrams/graph-neo4j-read-write-sequence.png)

The sequence follows `NewClient`, `ExecuteWrite`, `ReadVertices`/`ReadEdges`,
record collection, deterministic mapping, and redacted error wrapping without
retaining raw Cypher, parameters, or property values.

## Mapping Rules

- Neo4j `ElementId` is required. Deprecated numeric IDs are not used.
- Neo4j nodes can have multiple labels; `graph.Vertex` has one label. The
  adapter trims, de-duplicates, sorts labels lexicographically, and chooses the
  first label for deterministic mapping.
- Relationships map to directed `graph.Edge` values with start and end element
  IDs.
- Properties are passed through the `graph.Properties` shallow-copy boundary.
  Nested mutable values remain caller-owned.

## Client Boundary

`Client` owns no network pool. It wraps a caller-owned `neo4j.Driver` and closes
that driver only when `Client.Close` is called. Driver construction, auth,
TLS/routing configuration, cluster behavior, retries beyond the driver defaults,
and lifecycle ordering remain caller-owned.

`ReadVertices` and `ReadEdges` collect one named result column from a read
transaction. Use lower-level Neo4j sessions directly when a query needs mixed
columns, streaming result processing, custom transaction configuration, or
backend-specific behavior.

## Errors

```go
if errors.Is(err, neo4jgraph.ErrInvalidRecord) {
	// result column was missing, had the wrong type, or did not satisfy graph invariants
}
if errors.Is(err, neo4jgraph.ErrDriver) {
	// driver, session, transaction, query, or connectivity failure
}
```

Rendered errors include operation and column names but do not retain raw Cypher,
parameters, or property values.

## Test

The integration test starts one Neo4j container through Testcontainers for Go.
Run it serially with the package test:

```bash
go test -p 1 -count=1 ./graph/neo4j
go test -p 1 -race -count=1 ./graph/neo4j
```

The package test covers node/relationship mapping, bad records, query failure,
context cancellation, and resource cleanup.

## Benchmark

![graph/neo4j benchmark summary](../../docs/images/readme-charts/graph-neo4j-benchmark-summary.png)

Default benchmarks cover pure mapping and in-memory record adaptation without
starting Docker:

```bash
go test -run '^$' -bench . -benchmem ./graph/neo4j
```

Neo4j and Memgraph read/write benchmarks are serial and opt-in because they
start Testcontainers-backed databases:

```bash
BLUETAPE_GRAPH_NEO4J_BENCH=1 go test -p 1 -run '^$' -bench '^BenchmarkGraphNeo4jContainers' -benchtime=100x -benchmem ./graph/neo4j
```

Those rows use `neo4j:5.26.0` and `memgraph/memgraph:3.5.0`. They measure local
Testcontainers latency for create/read nodes, create/read relationships, empty
reads, and syntax-error overhead. Treat them as local regression evidence, not
production database ranking.

## Memgraph Compatibility

Memgraph starts as Neo4j-driver compatibility for this package, not as a
separate `graph/memgraph` backend abstraction. Memgraph exposes Bolt and Cypher
compatibility for the official Neo4j Go driver, so the proof stays on the same
`Client`, `VertexFromNode`, `EdgeFromRelationship`, `ReadVertices`, and
`ReadEdges` surface.

The package includes a generic Testcontainers-backed Memgraph matrix because
Testcontainers for Go does not publish a dedicated Memgraph module yet.

| Runtime | Image | Covered behavior |
|---|---|---|
| Neo4j | `neo4j:5.26.0` | create/read node and relationship, result mapping, bad query, cancellation, driver cleanup |
| Memgraph | `memgraph/memgraph:3.5.0` | same Neo4j-driver adapter surface: create/read node and relationship, result mapping, bad query, cancellation, driver cleanup |

Observed guardrails:

- Keep queries in the shared Cypher subset used by both runtimes.
- Require Bolt-returned `ElementId` values; deprecated numeric IDs remain out of
  the adapter contract.
- Add a dedicated `graph/memgraph` package only if a future compatibility test
  proves the Neo4j-driver surface is insufficient.

## Deferred Scope

| Capability | Owner |
|---|---|
| IAM access graph example | #368 |
| Backend-neutral repository/session/schema/transaction abstraction | Deferred until multiple adapters prove a common contract |
| GraphML, AGE, FalkorDB, TinkerPop, Neptune, broad Cypher DSL | Out of scope for this proof |
