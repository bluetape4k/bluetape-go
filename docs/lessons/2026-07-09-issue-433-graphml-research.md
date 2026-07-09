# Issue #433 GraphML Research Lesson

GraphML is a compatibility project, not just an XML parser task.

- Keep `graph/graphio` centered on bounded record streams until a real Go
  caller proves GraphML is required over NDJSON or paired CSV.
- When GraphML is revived, place XML-specific behavior behind an optional
  package boundary and define the subset before writing code.
- Compatibility claims need producer fixtures. Hand-written minimal GraphML is
  not enough to claim NetworkX, Gephi, Neo4j APOC, or yEd compatibility.
- Treat typed values, defaults, unknown keys, duplicate IDs, missing endpoints,
  nested graphs, hyperedges, ports, and yFiles visual payloads as explicit
  contract decisions.
- Any GraphML reader must be designed as untrusted XML input unless the caller
  explicitly opts into trusted files; bounded decoding and caller-owned
  deadline/close behavior are part of the acceptance criteria.
