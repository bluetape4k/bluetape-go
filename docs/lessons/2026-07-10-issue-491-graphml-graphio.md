# Issue #491 GraphML Graph I/O

The first GraphML slice should stay outside core `graphio`. A subpackage keeps
XML-specific parser limits, compatibility claims, and unsupported construct
tests from expanding the NDJSON/CSV record boundary.

Lesson: GraphML compatibility must be phrased as a subset, not a format-wide
claim. Supporting `graph`, `key`, `data`, `node`, and `edge` is useful, but it
does not imply yEd/yFiles visual payload, nested graph, hyperedge, port, or
mixed directed/undirected graph compatibility.

Lesson: XML safety is part of the graph contract. Reject directives, extension
payloads, unknown keys, and unsupported elements before converting into
`graph.Properties`; do not let arbitrary XML become caller metadata.

Prevention: future producer-specific compatibility work must add named fixtures
and document the producer/version before broadening the accepted subset.
