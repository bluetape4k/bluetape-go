# Issue 38 Graph Scope Research

## Context

Issue #38 is the 0.7.0 research gate for deciding whether the
`bluetape4k-graph` ecosystem should become Go packages, examples, or be
deferred before milestone 0.12.0 implementation starts.

This note supersedes the broad June 1 graph placeholder with source-backed
scope decisions for #44, #48, #49, #50, and #51.

## Source Inventory

Current `bluetape4k-graph` evidence:

- Root README describes a unified Kotlin API over Apache AGE, Neo4j, Memgraph,
  TinkerPop/TinkerGraph, and FalkorDB, plus graph I/O, Spring Boot/Ktor
  integrations, examples, benchmarks, and a BOM.
- `graph/graph-core` owns vertices, edges, paths, element IDs, labels,
  properties, repository contracts, traversal APIs, schema DSL, batch writes,
  optional schema/merge/transaction capabilities, weighted path hooks, and graph
  algorithm contracts.
- `graph-io` owns shared import/export reports and failure models, CSV paired
  files, NDJSON envelopes, GraphML XML/StAX subset, and OkIO streaming with
  compression/encryption adapters.
- Backend modules are `graph-neo4j`, `graph-memgraph`, `graph-age`,
  `graph-tinkerpop`, and `graph-falkordb`.
- Example modules cover code dependency, fraud detection, IAM access,
  knowledge graph, LinkedIn/social graph, observability incidents,
  recommendations, supply-chain impact, data lineage, network topology,
  security attack path, and Ktor integration.
- Benchmark docs contain already useful Go-facing signals: Neo4j is the safest
  production default, Memgraph is the fastest persistent backend in local
  latency/write rows, AGE timed out or underperformed in larger graph adoption
  runs, FalkorDB is lightweight but weak for edge-heavy repeated writes, and
  TinkerGraph remains valuable only as an in-memory contract/example baseline.

## Current Go Ecosystem Evidence

External sources checked on 2026-06-25:

- Neo4j has an official Go driver and current manual for `v6`
  (`github.com/neo4j/neo4j-go-driver/v6`).
- Memgraph's Go quick start explicitly uses the Neo4j Go driver because both
  products speak Bolt and Cypher. Memgraph keeps compatibility as the adoption
  path rather than a separate first-party Go driver surface.
- Apache AGE includes a Go driver/parser under the Apache AGE repository, but
  the surface is smaller than Neo4j's driver and remains tied to PostgreSQL AGE
  AGType/Cypher-over-SQL semantics.
- FalkorDB has `github.com/FalkorDB/falkordb-go/v2` plus FalkorDB docs listing
  Go client/OGM support.
- Testcontainers for Go has official modules for Neo4j and PostgreSQL. Current
  evidence did not show first-party Testcontainers Go modules for Memgraph,
  Apache AGE, or FalkorDB, so those candidates would need repo-local
  `GenericContainer` launchers or a custom module before implementation.

## Ranking

| Area | Go fit | Maintenance cost | Recommendation |
| --- | --- | --- | --- |
| Graph domain examples | High | Low/medium | Implement first as examples using a selected backend and in-memory fixtures. |
| Graph I/O NDJSON/CSV | High | Medium | Implement as backend-neutral helpers after core models settle. |
| Minimal graph models | Medium/high | Medium | Implement a small model package only if I/O/examples need it. |
| Neo4j adapter/examples | High | Medium | Adopt official driver directly; build thin examples/adapters only around repeated bluetape-go contracts. |
| Memgraph examples | Medium/high | Medium | Use Neo4j driver compatibility; keep as example/test matrix, not a distinct API first. |
| GraphML | Medium | Medium/high | Defer until CSV/NDJSON contracts prove value; XML edge cases raise maintenance cost. |
| Backend-independent repository API | Medium | High | Narrow heavily; avoid a lowest-common-denominator query abstraction. Use optional interfaces only after two adapters prove the same contract. |
| Apache AGE adapter | Low/medium | High | Defer; PostgreSQL-native recursive CTE or SQL examples may be better for Go services. |
| FalkorDB adapter | Low/medium | Medium/high | Defer implementation; keep as research/example-only until edge-heavy behavior and local test launcher are acceptable. |
| TinkerPop/TinkerGraph Go port | Low | High | Do not port directly; use pure in-memory test fixtures instead. |
| Spring/Ktor integrations | N/A for Go | High | Do not port; replace with workshop HTTP examples when needed. |
| Benchmarks | Medium | Medium | Add only after implementation candidates exist; use Go benchmarks and measured command output, not copied JMH claims. |

## Decisions

### Implement

- A small `graph` model surface only when required by #49 or #51:
  `Vertex`, `Edge`, `Path`, element IDs, labels, properties, and typed import
  errors. Keep it data-oriented and avoid repository/session abstractions in the
  first PR.
- NDJSON graph envelope helpers first. NDJSON has the best Go fit because it is
  streaming-friendly, line-oriented, easy to test with `encoding/json`, and does
  not require XML or paired-file coordination.
- CSV paired-file helpers second if NDJSON exposes a stable model and there is
  a clear bulk import/export example.
- One domain example first: observability incident or IAM access graph. Both
  map to Go service concerns and can be tested without adopting a broad graph
  abstraction.

### Adopt

- Neo4j official Go driver for real graph database examples and any future
  adapter experiments.
- Neo4j Go driver for Memgraph compatibility experiments; record Memgraph
  compatibility quirks separately instead of inventing another interface.
- Testcontainers Go Neo4j module for Neo4j integration tests.
- Testcontainers Go PostgreSQL module only for PostgreSQL/AGE exploratory tests;
  do not treat AGE as selected until a dedicated local launcher proves reliable.

### Example-Only

- Memgraph should start as a Neo4j-driver compatibility example or test matrix
  row, not a separate public package.
- Backend comparison should live in docs/workshop examples until two backends
  prove an identical Go-shaped contract.

### Defer

- Broad backend-independent repository/session abstractions.
- Schema/index DSL, merge/upsert, transaction DSL, and algorithm interfaces in
  base packages.
- AGE adapter, FalkorDB adapter, TinkerPop/TinkerGraph equivalent, Neptune, and
  GraphML until the first Go graph examples show user value.
- Spring Boot/Ktor integration analogues; use ordinary Go HTTP examples if a
  service integration is needed.

## Issue Updates Required

- #44: Make 0.12.0 graph epic implementation order explicit: examples and
  NDJSON/CSV before adapters, adapter work only after #50 proves local test
  maturity, and no Spring/Ktor parity port.
- #48: Narrow base abstraction to models plus optional capability follow-ups.
  Defer repository/session interfaces unless two selected adapters prove a
  shared contract.
- #49: Start with NDJSON, then CSV. Defer GraphML and compression/encryption
  chaining until the core import/export reports and streaming ownership policy
  are stable.
- #50: Rank backend feasibility as Neo4j first, Memgraph compatibility second,
  AGE/FalkorDB deferred, TinkerGraph not ported, Neptune research-only.
- #51: Select observability incident or IAM access graph as the first Go domain
  example; defer the large source example matrix until backend/I/O decisions are
  proven.

## Validation Plan for Future Implementation

- Public Go APIs need success, malformed input, cancellation, and zero-value
  tests.
- Streaming import/export must propagate `context.Context`, close all
  `io.Reader`/`io.Writer` resources according to ownership flags, and include
  large-record tests.
- Any adapter or graph server example must run Testcontainers-backed packages
  serially and include local cleanup evidence.
- Concurrency/race evidence is required for shared readers/writers, batched
  importers, caches, or goroutine-owned adapters.

## Follow-Up Recommendation

Do not begin #48 with the full Kotlin repository contract. Start #49/#51 with a
small shared model and NDJSON/example proof. If that proof exposes repeated
backend operations, create a narrower #48 implementation issue for only those
operations.
