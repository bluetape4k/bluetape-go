# Issue #365 Neo4j Adapter Proof Design

## Goal

Implement the first graph backend proof around the official Neo4j Go driver.
The proof must show that Neo4j query results can cross into the existing
`graph.Vertex` and `graph.Edge` values without creating a broad repository or
query abstraction too early.

## Package Path

Use `graph/neo4j`.

Rejected alternatives:

- `graph/neo4jgraph`: avoids import-name collision but produces a less idiomatic
  path. Callers can alias the package as `neo4jgraph`.
- `graph/backend/neo4j`: implies a stable backend hierarchy before multiple
  backend packages exist.
- `graph/repository`: would recreate the broad JVM-shaped abstraction that #50
  explicitly deferred.

## Public Surface

- Conversion helpers:
  - `VertexFromNode(dbtype.Node) (graph.Vertex, error)`.
  - `EdgeFromRelationship(dbtype.Relationship) (graph.Edge, error)`.
  - `VerticesFromRecords([]*neo4j.Record, column string) ([]graph.Vertex, error)`.
  - `EdgesFromRecords([]*neo4j.Record, column string) ([]graph.Edge, error)`.
- Client helper:
  - `NewClient(neo4j.Driver, ...Option) (*Client, error)`.
  - `WithDatabase(name string) Option`.
  - `VerifyConnectivity`, `ExecuteWrite`, `ReadVertices`, `ReadEdges`, `Close`.
- Sentinel errors:
  - `ErrInvalidOptions`.
  - `ErrInvalidRecord`.
  - `ErrDriver`.

## Mapping Rules

- Require Neo4j `ElementId` and do not use deprecated numeric IDs.
- Map Neo4j multi-label nodes to one `graph.Label` by trimming,
  de-duplicating, sorting labels lexicographically, and choosing the first.
- Map Neo4j relationships to directed `graph.Edge` values.
- Preserve `graph.Properties` shallow-copy semantics; do not deep-copy or
  sanitize nested property values.

## Driver Boundary

`Client` wraps a caller-owned `neo4j.Driver`. It does not own driver
construction, authentication, TLS/routing, cluster policy, custom retry policy,
or streaming result handling. `Client.Close` closes the wrapped driver, so
callers must not close it concurrently with in-flight operations.

## Test Requirements

- Pure conversion tests for node, relationship, multi-label selection, missing
  element IDs, record-column failures, and redacted error strings.
- Testcontainers-backed Neo4j integration test covering successful write/read,
  query failure, context cancellation, and driver cleanup.
- Serial execution documented because the package starts a Neo4j container.

## Follow-Ups

- #366 proves Memgraph compatibility against this surface.
- #368 can use the package from an IAM access graph example when an example
  needs a real backend.
- Backend-neutral repository/session/schema contracts remain deferred until at
  least two backend packages prove a shared shape.
