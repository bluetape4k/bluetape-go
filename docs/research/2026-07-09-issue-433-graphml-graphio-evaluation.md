# Issue #433 GraphML Import/Export Evaluation

Issue: [#433](https://github.com/bluetape4k/bluetape-go/issues/433)  
Milestone: Backlog  
Date: 2026-07-09  
Decision: **defer GraphML implementation in `graph/graphio`**

## Decision

Do not add GraphML import/export to `graph/graphio` now. Keep NDJSON and paired
CSV as the supported record-stream formats, and revisit GraphML only after a
Go caller or workshop example proves that XML GraphML compatibility is the
right interchange boundary.

GraphML remains a valid future candidate, but it should be a separate optional
slice or subpackage, not a silent expansion of the current core `graphio`
record boundary.

## Current Repo Evidence

| Evidence | Result |
|---|---|
| `graph/graphio/doc.go` | GraphML, compression, encryption, path ownership, atomic replacement, and backend integration are intentionally deferred. |
| `graph/graphio/README.md` / `.ko.md` | Unsupported-capability table says GraphML follows NDJSON/CSV adoption evidence. |
| `graph/README.md` / `.ko.md` | Capability matrix keeps GraphML as a deferred follow-up. |
| `docs/research/2026-06-25-issue-38-graph-scope.md` | Prior graph scope rated GraphML as medium fit but medium/high maintenance cost. |
| `docs/lessons/2026-06-30-graph-io-boundaries.md` | The accepted boundary is bounded record streams, not filesystem, backend, or XML compatibility ownership. |
| `bluetape-go-workshop` issue #52 | The downstream Go workshop request still allows CSV, NDJSON, or GraphML-style fixtures; it does not require GraphML specifically. |

## External Compatibility Evidence

| Source | Useful signal for `graphio` |
|---|---|
| NetworkX GraphML docs | NetworkX supports GraphML read/write, but explicitly leaves mixed directed/undirected graphs, hypergraphs, nested graphs, and ports unsupported. Its reader also warns that Python XML parsing should be limited to trusted files. |
| NetworkX `read_graphml` / `write_graphml` docs | Edge IDs, multigraph behavior, yEd extensions, defaults, compression, and numeric type inference all affect compatibility expectations beyond simple nodes and edges. |
| Gephi GraphML docs | Gephi supports only a limited GraphML subset and excludes subgraphs and hyperedges; it supports `boolean`, integer, floating, and string attribute types plus defaults. |
| Neo4j APOC GraphML export/import docs | APOC uses GraphML for interoperability but loses some property-graph fidelity: mixed property value types and unsupported value types become strings. Import also exposes label, relationship-type, node-ID, and batch-size configuration. |
| yWorks/yFiles GraphML docs | yFiles extends the standard data/key mechanism to arbitrary complex data; yEd visual GraphML compatibility is therefore not just the structural GraphML subset. |
| `bluetape4k-graph` PR #272 / issue #235 | The Kotlin line needed explicit fixtures, skip/fail behavior, and unsupported-construct documentation before claiming GraphML compatibility. |
| `bluetape4k-graph` PR #349 | Typed GraphML values required dedicated error-reporting fixes, reinforcing that GraphML is not a low-cost parser addition. |

## Classification

| Question | Answer |
|---|---|
| Implement now? | No. There is no current bluetape-go caller that requires GraphML over NDJSON/CSV. |
| Defer or reject? | Defer. GraphML has ecosystem value, but only as a constrained compatibility slice. |
| Placement if revived? | Prefer `graph/graphio/graphml` or a similarly optional package so XML-specific options, fixtures, and limitations do not expand the core NDJSON/CSV API. |
| First supported subset? | Directed property graph subset: graph, node, edge, key/data attributes, scalar values, edge IDs, duplicate ID checks, missing endpoint checks, and explicit input limits. |
| Explicit non-goals for first slice | Nested graphs, mixed directed/undirected graphs in one document, hyperedges, ports, yFiles visual styling, arbitrary XML extension payloads, path ownership, compression/encryption wrappers, and graph database import/export semantics. |

## Risk Matrix

| Risk | Severity | Reason |
|---|---|---|
| XML parser safety and untrusted input limits | High | GraphML is XML, so any implementation must reject unsafe constructs, bound input, and preserve caller-owned reader/deadline behavior. |
| Compatibility overclaim | High | Common producers support different subsets; accepting one simple file does not prove yEd, Gephi, NetworkX, and APOC compatibility. |
| Type conversion drift | Medium/high | GraphML scalar declarations, defaults, mixed value types, and Neo4j property coercions can silently change property values. |
| API creep | Medium | A complete GraphML surface would pull schema/key/default/extension semantics into a package currently scoped to simple records. |
| Fixture maintenance | Medium | Real producer samples are required to avoid compatibility claims based only on hand-written minimal XML. |

## Follow-Up Gate

Open an implementation issue only when one of these becomes true:

- `bluetape-go-workshop` issue #52 chooses GraphML, not NDJSON or CSV, for its
  scenario-shaped example.
- A `graph/neo4j` or migration workflow needs GraphML interchange against a
  named producer such as Gephi, NetworkX, Neo4j APOC, or yEd.
- A downstream repository provides representative GraphML fixtures that cannot
  be handled reasonably through NDJSON/CSV conversion.

The follow-up issue should define:

- package placement and dependency policy;
- accepted GraphML subset;
- XML decoder safety limits and rejected constructs;
- typed property conversion and defaults;
- duplicate IDs, missing endpoints, unknown keys, and malformed value errors;
- fixture corpus from accepted producers;
- round-trip and fail-closed tests;
- verification commands:
  - `go test -count=1 ./graph ./graph/graphio`
  - `go test -race -count=1 ./graph/graphio`

## Outcome

Keep README wording as a deferred follow-up and link this note as the current
decision. No production Go API or behavior changes are required for #433.
