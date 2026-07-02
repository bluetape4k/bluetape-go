// Package neo4j adapts official Neo4j Go driver results to graph values.
//
// This package is intentionally a narrow backend proof. It maps Neo4j
// dbtype.Node and dbtype.Relationship values into graph.Vertex and graph.Edge
// and provides small read/write query helpers around a caller-owned driver. It
// does not define a backend-neutral repository, session, transaction, schema,
// or Cypher DSL.
package neo4j
