# Issue #48 Graph Abstraction Design

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

## 맥락

Issue #48 is the P0 task for milestone `0.10.0` graph packages and examples.
It asks for a Go graph abstraction designed against `bluetape4k-graph` source
capabilities without copying the Kotlin repository DSL or reducing backend
drivers to a lowest-common-denominator wrapper.

Current live issue evidence:

- #44 is the `0.10.0` graph epic and owns #48 through #51.
- #48 asks for minimal Go models for vertex, edge, path, labels, properties,
  IDs, typed errors, and compile-only examples.
- #38 and PR #307 narrowed the implementation order: keep the base graph
  surface model-focused, then let #49 graph I/O and #51 domain examples prove
  any broader repository or backend contract.

Current repository evidence:

- `bluetape-go` has no `graph` package yet. CodeGraph search for `Graph` only
  finds `GraphicsLength` symbols and docs asset metadata.
- Root README already lists `0.10.0` as graph packages and examples, but graph
  is still described as a planned family rather than an active package.
- The baseline command `go test ./...` passed in the feature worktree before
  any changes.

Source parity evidence from `bluetape4k-graph/graph/graph-core`:

- `GraphElementId.kt` defines a backend-independent string ID wrapper with
  constructors from strings and numeric backend IDs.
- `GraphVertex.kt` defines immutable vertices with ID, label, and nullable
  property values.
- `GraphEdge.kt` defines immutable directed edges with ID, label, start ID,
  end ID, and nullable property values. Self-loops are allowed.
- `GraphPath.kt` defines alternating vertex/edge path steps, derived vertices,
  derived edges, length, empty path, and total weight.
- `GraphSchemaModels.kt` defines schema/index metadata, but #38 and #48 keep
  schema/index contracts outside the first Go base API.
- `GraphExceptions.kt` defines graph-specific typed error categories.
- `GraphOperations.kt` and `GraphSession.kt` compose session and repository
  contracts, but #38 explicitly rejects starting Go with a broad repository or
  backend-independent session abstraction.

## Problem

The graph milestone needs a shared Go model package so #49 can encode graph
I/O records and #51 can build examples without each package inventing its own
vertex, edge, path, and error shapes. The first API must still avoid implying
that every graph backend supports the same transaction, schema, merge, query,
or algorithm semantics.

## 목표s

- Add a new package `graph` with small, documented, backend-neutral values.
- Provide `ElementID`, `Vertex`, `Edge`, `Path`, `PathStep`, labels,
  properties, validation helpers, and typed sentinel errors.
- Keep constructors defensive: trim labels and IDs, reject blank IDs/labels,
  copy mutable maps and slices at the package boundary, and avoid exposing
  mutable internal slices or maps.
- Keep path semantics explicit: path steps can represent alternating
  vertex/edge sequences, vertex-only paths, edge-only paths, weighted paths,
  and empty paths without requiring backend traversal APIs. Path construction
  must keep invariants behind constructors and accessors.
- Add compile-only examples that exercise model construction and validation
  without requiring a graph server.
- Add compile-only examples that show how raw backend node/relationship records
  can be adapted into graph values without adopting a backend driver contract.
- Document explicit omissions and follow-up routing for #49, #50, and #51.

## Non-Goals

- No repository, session, transaction, query, traversal, schema DSL, merge,
  batch insert, or algorithm interface in the base package.
- No Neo4j, Memgraph, AGE, FalkorDB, TinkerPop, GraphML, or Testcontainers
  dependency in this issue.
- No copied Kotlin DSL shape, coroutine shape, virtual-thread adapter, or
  TinkerPop-style abstraction.
- No performance, benchmark, or backend capability claims without Go
  measurement evidence.

## 설계 Options

### Option A: Minimal model package only

Create `graph` with values, constructors, validation, cloning, typed errors,
examples, and README guidance. Optional capabilities stay out of code and are
documented as follow-up boundaries.

Pros:

- Matches #38 and #48 narrowing.
- Gives #49/#51 stable shared values without overcommitting backend contracts.
- Avoids new dependencies.
- Keeps API review small enough to evolve before first release.

Cons:

- #49/#51 may need to add narrow optional interfaces later.
- Callers that already use Neo4j or Memgraph directly will still write their
  own repository code.

### Option B: Models plus optional capability interfaces

Add models and small optional interfaces such as `SchemaCapable`,
`TransactionCapable`, `BatchWriter`, or `PathFinder`.

Pros:

- Looks closer to `bluetape4k-graph` source inventory.
- Could reduce future API additions if two backends align quickly.

Cons:

- Violates the #38 ordering because no Go I/O package, domain example, or two
  backend experiments have proven a shared contract.
- Risks a lowest-common-denominator wrapper before backend behavior is known.
- Adds review and documentation burden without immediate caller value.

### Option C: No package yet; let #49/#51 define local models first

Defer `graph` package creation until I/O and examples independently reveal the
minimum shared model.

Pros:

- Maximizes evidence before exported API.
- Avoids premature public surface.

Cons:

- Duplicates vertex/edge/path/error shapes across #49 and #51.
- Makes later convergence harder because public examples may expose temporary
  local types.
- Does not satisfy #48 acceptance criteria for minimal Go models.

## Decision

Choose Option A.

The package will expose only model values and error categories required by #49
and #51. Repository/session/schema/index/merge/transaction/batch/algorithm
contracts are intentionally not abstracted in code. They remain follow-up
decisions that need I/O, example, and backend evidence.

## Proposed API Shape

The first implementation should use these files:

- `graph/doc.go`: package documentation and explicit scope boundaries.
- `graph/errors.go`: sentinel errors and `ValidationError`.
- `graph/types.go`: `ElementID`, `Label`, `Properties`, `Vertex`, `Edge`.
- `graph/path.go`: `PathStep`, `Path`, and path constructors.
- `graph/*_test.go`: TDD coverage for validation, defensive copies, path
  derivations, JSON validation, redacted errors, and zero-value behavior.
- `graph/example_test.go`: compile-only examples.
- `graph/README.md` and `graph/README.ko.md`: package user guidance.
- Root `README.md` and `README.ko.md`: move graph from planned family to
  active package table and package documentation list.
- `CHANGELOG.md` and `WIP.md`: record the new public package in release
  bookkeeping.

Expected public concepts:

- `type ElementID string`
- `func NewElementID(value string) (ElementID, error)`
- `func (id ElementID) Validate() error`
- `func ElementIDFromInt(value int64) (ElementID, error)`
- `func MustElementID(value string) ElementID`
- `type Label string`
- `func NewLabel(value string) (Label, error)`
- `func (label Label) Validate() error`
- `type Properties map[string]any`
- `func (p Properties) Clone() Properties`
- `type Vertex struct { ... }` with unexported fields and accessor methods
  `ID() ElementID`, `Label() Label`, `Properties() Properties`, and
  `Validate() error`.
- `type EdgeEndpoints struct { Start ElementID; End ElementID }`
- `func (endpoints EdgeEndpoints) Validate() error`
- `type RawEdgeEndpoints struct { Start string; End string }`
- `type Edge struct { ... }` with unexported fields and accessor methods
  `ID() ElementID`, `Label() Label`, `StartID() ElementID`,
  `EndID() ElementID`, `Properties() Properties`, and `Validate() error`.
- `func NewVertex(id ElementID, label Label, properties Properties) (Vertex, error)`
- `func NewEdge(id ElementID, label Label, endpoints EdgeEndpoints, properties Properties) (Edge, error)`
- Convenience parse helpers `ParseVertex(id string, label string, properties Properties) (Vertex, error)`
  and `ParseEdge(id string, label string, endpoints RawEdgeEndpoints, properties Properties) (Edge, error)`.
- `type PathStep struct { ... }` with unexported discriminant and value fields,
  plus `IsVertex()`, `IsEdge()`, `Vertex() (Vertex, bool)`, and
  `Edge() (Edge, bool)`, and `Validate() error`.
- `type Path struct { ... }` with unexported steps and total weight, plus
  `Validate() error`.
- `func NewPath(steps ...PathStep) (Path, error)`
- `func NewWeightedPath(weight float64, steps ...PathStep) (Path, error)`
- `func VertexStep(vertex Vertex) (PathStep, error)`
- `func EdgeStep(edge Edge) (PathStep, error)`
- `func EmptyPath() Path`
- Derived path methods: `Steps() []PathStep`, `Vertices() []Vertex`,
  `Edges() []Edge`, `Length() int`, `TotalWeight() float64`, `IsEmpty() bool`.

`NewElementID` accepts only strings and rejects blank values. Numeric backend
IDs must use `ElementIDFromInt`; it accepts non-negative integers and rejects
negative values. Unsupported `any` conversion is intentionally not part of the
base API. This keeps raw backend adaptation explicit and avoids accidental
stringification of structs, booleans, floats, or secret-bearing `Stringer`
values.

`NewEdge` and `ParseEdge` must use named endpoint structs instead of adjacent
same-type positional start/end parameters. `EdgeEndpoints` is constructed with
named fields and validated through `Validate`; no public helper should accept
`start, end` adjacent parameters. This makes the directed edge role visible at
the callsite and reduces accidental endpoint reversal:

```go
edge, err := graph.ParseEdge(
    "rel-1",
    "CALLS",
    graph.RawEdgeEndpoints{Start: "service-a", End: "service-b"},
    graph.Properties{"latency_ms": 12},
)
```

Path validation should keep invalid step shapes unrepresentable through public
constructors. `VertexStep` and `EdgeStep` must validate their inputs and return
errors for zero or otherwise invalid values; callers should not be able to
silently create invalid steps through exported helpers. Path order should not
require alternating order yet, because #49 may need edge-only streams and #51
may need vertex-only fixture paths. Documentation must say that stricter
traversal-path validation belongs to later algorithms or backend adapters.
`NewWeightedPath` must reject `NaN`, infinity, and negative weights.

Zero-value behavior:

- Zero `ElementID` and zero `Label` are invalid and must be detected by
  `Validate`, `NewVertex`, `NewEdge`, `ParseVertex`, and `ParseEdge`.
- Zero `Vertex`, `Edge`, and `PathStep` values are invalid implementation
  artifacts; exported constructors should be used to create valid values.
- Zero `Path` is a valid empty path: `IsEmpty() == true`, `Length() == 0`,
  `TotalWeight() == 0`, and returned slices are nil or empty.
- Methods must not panic on zero values; invalid values should return empty
  accessors or fail validation where applicable.

## Error Contract

The package should follow existing package patterns from `audit/errors.go`:

- Sentinel errors must be usable with `errors.Is`.
- Field-specific validation should avoid leaking property values in error
  strings.
- Graph validation errors should not retain raw sensitive values in exported
  fields. Store only `Kind`, `Field`, optional `Cause`, and redacted summaries
  such as a type name or length when needed.
- Public constructors and parse helpers that can observe invalid values return
  errors; `Must*` helpers may panic only where the repo already uses that
  convention for values.

Expected sentinels:

- `ErrInvalidElementID`
- `ErrInvalidLabel`
- `ErrInvalidVertex`
- `ErrInvalidEdge`
- `ErrInvalidPath`
- `ErrUnsupportedCapability`

`ErrUnsupportedCapability` exists only as a typed error category for later #49,
#50, and #51 boundaries. It must not be backed by a base capability interface
in this issue.

JSON decoding must not bypass invariants. Because core fields are unexported,
model types that support JSON must implement validating `MarshalJSON` and
`UnmarshalJSON`, and the implementation must expose `Validate` methods for
post-decode checks. Negative tests must cover blank IDs/labels, invalid path
steps, invalid weights, unsupported raw ID examples, and redacted validation
errors.

Public JSON shape is part of the first release contract:

- `ElementID` and `Label` encode as JSON strings and decode through
  `NewElementID` and `NewLabel`.
- `Vertex` encodes as `{"id": "...", "label": "...", "properties": {...}}`.
- `Edge` encodes as
  `{"id": "...", "label": "...", "start": "...", "end": "...", "properties": {...}}`.
- `PathStep` encodes with a single discriminator object:
  `{"vertex": {...}}` or `{"edge": {...}}`; both-present and neither-present
  shapes are invalid.
- `Path` encodes as `{"steps": [...], "total_weight": <number>}` and rejects
  `NaN`, infinity, and negative weights after decode.

## 검증 And Tests

TDD must cover:

- `NewElementID` accepts non-blank strings and rejects blank values.
- `ElementIDFromInt` accepts non-negative integers and rejects negative values.
- `MustElementID` panics for invalid input and is documented only for constants
  and test/static fixtures; runtime input should use `NewElementID`.
- Unsupported raw ID inputs such as nil, bool, float, struct, slice, and
  `fmt.Stringer` are not accepted by the base API and are covered by parse or
  adapter examples instead of broad `any` conversion.
- `NewLabel` trims and rejects blank labels.
- `Properties.Clone` returns a shallow defensive copy and preserves nil/empty
  behavior.
- Nested mutable property values intentionally remain caller-owned; this is
  tested and documented as not being a deep-copy, sanitization, or trust-boundary
  primitive.
- `NewVertex` and `NewEdge` accept typed `ElementID`/`Label` values, validate
  them again to catch zero values, and copy properties.
- `ParseVertex` and `ParseEdge` validate raw string inputs for examples and
  backend adaptation.
- `ParseEdge` uses `RawEdgeEndpoints`, and `NewEdge` uses `EdgeEndpoints`, so
  start/end roles are named at the callsite.
- Edge self-loops are valid.
- `NewPath` copies steps, computes default weight from edge count, and exposes
  defensive `Steps`, `Vertices`, and `Edges` slices.
- `NewWeightedPath` rejects `NaN`, infinity, and negative weights.
- `VertexStep` and `EdgeStep` return errors for invalid zero-value inputs.
- Invalid path steps are unrepresentable through successful constructors;
  zero-value `PathStep` behavior is documented and tested.
- Direct JSON decode of invalid values fails, including scalar `ElementID` and
  `Label` values, invalid vertex/edge fields, invalid path-step shape, and
  invalid path weights.
- Error strings, formatted validation errors, and exported validation-error
  fields do not retain raw property, ID, label, JSON, or `fmt.Stringer` values.
- Examples compile without network, database, or Testcontainers.
- Examples show `errors.Is` usage and raw-driver record adaptation without
  adopting a backend driver package. At least one example must use the real
  import path `github.com/bluetape4k/bluetape-go/graph` and show local raw
  node/relationship structs being converted to `graph.Vertex` and `graph.Edge`.

Stress tests are not required for this first package because it contains no
goroutines, shared readers/writers, caches, worker pools, or batched importers.
The values are shallow defensive-copy models, not a synchronization primitive:
nested mutable property values remain caller-owned. The issue's stress
requirement should become active for #49 streaming/batched I/O and any future
backend adapters. This issue still requires race validation for the package
surface.

## Documentation

Package README files should explain:

- What the package is: shared graph values for I/O and examples.
- What it is not: graph database client, query DSL, transaction manager,
  schema DSL, algorithm engine, or backend adapter.
- How to construct vertices, edges, and paths.
- How typed validation errors are intended to be used.
- How to adapt raw backend IDs explicitly without broad `any` conversion.
- A table routing unsupported capabilities to #49, #50, and #51.
- That `ErrUnsupportedCapability` is reserved for future capability boundaries
  and no #48 base API returns it yet.
- Which follow-up issues own I/O helpers, backend evaluation, and examples.
- Import, minimal vertex/edge/path usage, `errors.Is`/`errors.As` handling,
  raw backend record adaptation, unsupported capability routing, property
  ownership, and test commands.
- A release-support note: before a tag, rollback is deleting `graph` plus docs
  and release bookkeeping; after a tag, changes must preserve Go API
  compatibility or move to a breaking release.

Root README files should add `graph` as active and remove graph from the
planned-family sentence.
All exported graph identifiers require English Go doc comments suitable for
`pkg.go.dev`, with zero-value and ownership behavior documented where relevant.

## Acceptance Criteria

- `graph` package exists with documented exported API.
- `go test -count=1 ./graph` passes and demonstrates the intended API through
  examples.
- `go test -race -count=1 ./graph` passes.
- Repo-wide verification uses serial gates: `make test`, `make race`, and
  `make ci` where available.
- `make fmt-check`, `make tidy-check`, `make vet`, `make lint`, and `make ci`
  pass, or any unavailable command is reported with exact blocker evidence.
- README and README.ko remain synchronized for graph package visibility.
- `CHANGELOG.md` and `WIP.md` record the new active package.
- Review evidence records P0=0 and P1=0 before PR creation.

## 위험 And Mitigations

| Risk | Mitigation |
| --- | --- |
| Public API becomes too broad before backend proof. | Keep code to value models and typed errors only; document optional capabilities as omitted. |
| Path validation overfits traversal semantics before #49/#51. | Validate only step shape and defensive copying; defer alternating-order constraints. |
| `map[string]any` property values can alias nested mutable objects. | Copy only the map boundary, hide internal maps behind accessors, document nested value ownership, and require future I/O/backend adapters to deep-copy before trust boundaries. |
| README implies backend support that does not exist. | State explicitly that graph is not a database client and link #49/#50/#51 as follow-ups. |
| Stress-test requirement is misapplied to model values. | Record N/A rationale now, run race validation for `graph`, and require stress coverage for future goroutine/streaming/backend work. |

## Open Questions

None blocking after #38 and #48 narrowing. Future repository/session capability
questions should be answered by #49, #50, and #51 evidence rather than this
base model package.
