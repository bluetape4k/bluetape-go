# Issue #49 Graph I/O Design

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

## 맥락

Issue #49 is the first graph I/O implementation task for milestone `0.10.0`.
It follows #48, which added only model values in package `graph`: `ElementID`,
`Label`, `Properties`, `Vertex`, `Edge`, `Path`, JSON round trips, and redacted
validation errors. The #48 lesson deliberately left repository, session,
schema, backend, algorithm, and traversal contracts out of the base API.

Current live issue evidence:

- #44 is the `0.10.0` graph epic and keeps the graph track focused on useful
  Go packages rather than a mechanical Kotlin port.
- #38 and the #49 research update set the I/O order as NDJSON first, CSV
  second, and GraphML deferred.
- #49 asks for CSV, NDJSON, GraphML, and streaming APIs to be either
  implemented or explicitly deferred with source-parity rationale.
- #51 examples should use concrete I/O and domain value before backend work
  broadens the API.
- #50 backend adapter research remains separate and must not be pulled into
  this package.

Current repository evidence:

- `graph` is active and has model values plus package README files.
- There is no graph I/O package yet.
- `testing/concurrency` provides `GoroutineStressTester` and
  `AsyncJobTester`; #49 explicitly requires both when adding new functionality.
- The feature worktree baseline passed `go mod download` and
  `go test -count=1 ./graph`.

Source parity evidence from `bluetape4k-graph/graph-io`:

- `graph-io-core` defines shared records, options, reports, import/export
  contracts, duplicate vertex policy, missing endpoint policy, sources/sinks,
  failure reports, and elapsed timing.
- `graph-io-jackson2` and `graph-io-jackson3` use the same NDJSON envelope:
  one JSON object per line, with `type` equal to `vertex` or `edge`.
- NDJSON vertex lines use `id`, `label`, and `properties`.
- NDJSON edge lines use `id`, `label`, `from`, `to`, and `properties`.
- Kotlin importers create or read vertices first, buffer edges, then resolve
  edge endpoints after vertices are known.
- `graph-io-csv` uses two files: `vertices.csv` and `edges.csv`.
- CSV vertex files use `id`, `label`, and property columns.
- CSV edge files use `id`, `label`, `from`, `to`, and property columns.
- CSV supports property modes, including prefixed columns and a raw JSON
  property payload column.
- GraphML is a streaming XML/StAX property-graph subset with additional
  unsupported-element policy and key/type handling.
- OkIO adds path/source/sink ownership, segment streaming, compression chaining,
  encryption-aware stream concepts, and atomic writes.
- The source benchmark module measures CSV, NDJSON, GraphML, and OkIO paths,
  but there is no Go graph I/O measurement yet.

## Problem

The Go graph package needs reusable streaming I/O helpers so examples and later
backend adapters can exchange graph values without each package inventing its
own wire shape, report type, duplicate handling, and cancellation behavior. The
first implementation must still be small enough to review, test, and evolve
before a backend contract exists.

The dangerous failure modes are silent graph corruption, unbounded memory growth
on large streams, context cancellation that keeps reading or writing, raw value
leakage in errors, and broad compatibility claims for GraphML, compression, or
encrypted streams without Go evidence.

## 목표s

- Add a narrow graph I/O package under `graph` that works directly with #48
  model values.
- Implement streaming NDJSON read/write as the primary format.
- Implement CSV read/write as a paired stream/file contract.
- Preserve Go-shaped API boundaries: explicit `context.Context`,
  `io.Reader`/`io.Writer`, standard library only, and typed options/reports.
- Provide duplicate vertex and missing endpoint policies for import
  validation.
- Keep endpoint resolution local to the stream parser and fail closed by
  default: edges reference vertex IDs through `from`/`to` and are checked
  against vertices already seen in the same import. The first Go streaming
  contract requires vertex records before edge records when endpoint validation
  is enabled.
- Keep all error messages redacted and `errors.Is`/`errors.As` compatible.
- Support cancellation checks during long reads and writes.
- Cover malformed input, large streaming graphs, property typing, missing
  endpoints, duplicate IDs, round trips, logical EOF, truncated final input,
  post-terminal reader/writer reuse, and double terminal calls.
- Document format selection, memory behavior, streaming caveats, unsupported
  capabilities, and the benchmark/measurement boundary.

## Non-Goals

- No graph repository, session, transaction, traversal, schema, merge, batch
  insert, or backend adapter abstraction.
- No Neo4j, Memgraph, AGE, FalkorDB, TinkerPop, or Testcontainers dependency.
- No GraphML implementation in #49. GraphML remains deferred because its XML
  key/type/unsupported-element policy is much broader than the first Go stream
  API and needs separate design.
- No OkIO-style adapter, compression chaining, encryption chaining, or atomic
  file-write helper in #49. Go can add narrow gzip/path helpers later only after
  the base stream contract is stable.
- No benchmark performance claims. #49 may record a measurement plan and simple
  benchmark skeleton only if it does not imply production throughput ranking.
- No deep property sanitization. `graph.Properties` remains a shallow map; I/O
  must validate JSON-compatible property values at the wire boundary and avoid
  including raw values in errors.

## 설계 Options

### Option A: `graph/graphio` with records, reports, and stream codecs

Create `graph/graphio` as a small subpackage that imports `graph`, exposes
format-neutral stream records/reports/options, and format-specific NDJSON/CSV
helpers.

Pros:

- Keeps model values in `graph` and I/O concerns in a dedicated package.
- Avoids the confusing package name `io`, which collides with the standard
  library in examples.
- Gives #51 a stable import path without backend abstractions.
- Makes later GraphML or compression packages additive.

Cons:

- Adds one subpackage instead of keeping all graph APIs in `graph`.
- Callers must import both `graph` and `graph/graphio` for construction plus
  streaming.

### Option B: Put I/O helpers directly in package `graph`

Add all NDJSON/CSV types and functions to `graph`.

Pros:

- One import path for callers.
- Simple discovery in package docs.

Cons:

- Mixes model invariants with stream policy, reporting, and parsing.
- Makes future GraphML/compression additions more likely to bloat the base
  package.
- Weakens #48's model-only boundary.

### Option C: Create independent `graphio` top-level package

Create `graphio` at the repository root.

Pros:

- Short import path and clean separation from model code.

Cons:

- Disconnects graph I/O from the graph package family in README navigation.
- Makes later graph subpackages less discoverable.
- Does not match the current milestone organization.

## Decision

Choose Option A: create `graph/graphio`.

The package will implement NDJSON and CSV stream codecs using the standard
library only. It will expose format-neutral records, options, reports, and
typed errors. GraphML, compression, encryption, and path ownership are
documented as deferred follow-up capabilities. #49 will not add public GraphML,
compression, encryption, path, or atomic-file helpers, and normal NDJSON/CSV
code paths must not return `graph.ErrUnsupportedCapability`.

## Proposed API Shape

### Core types

Files:

- `graph/graphio/doc.go`
- `graph/graphio/errors.go`
- `graph/graphio/options.go`
- `graph/graphio/records.go`
- `graph/graphio/report.go`
- `graph/graphio/ndjson.go`
- `graph/graphio/csv.go`
- `graph/graphio/example_test.go`
- `graph/graphio/*_test.go`
- `graph/graphio/README.md`
- `graph/graphio/README.ko.md`

Expected public concepts:

- `type Format string`
  - `FormatNDJSON`
  - `FormatCSV`
- `type RecordKind string`
  - `RecordVertex`
  - `RecordEdge`
- `type Record struct`
  - `Kind RecordKind`
  - `Vertex graph.Vertex`
  - `Edge graph.Edge`
- `func VertexRecord(graph.Vertex) (Record, error)`
- `func EdgeRecord(graph.Edge) (Record, error)`
- `type DuplicateVertexPolicy string`
  - `DuplicateVertexFail`
  - `DuplicateVertexSkip`
- `type MissingEndpointPolicy string`
  - `MissingEndpointFail`
  - `MissingEndpointSkipEdge`
- `type ReadOptions struct`
  - `DuplicateVertexPolicy DuplicateVertexPolicy`
  - `MissingEndpointPolicy MissingEndpointPolicy`
  - `MaxLineBytes int`
  - `MaxRecordBytes int`
  - `MaxFieldBytes int`
  - `MaxColumns int`
  - `MaxRecords int64`
  - `MaxFailures int`
- `const UnlimitedRecords int64 = -1`
- `type WriteOptions struct`
  - `IncludeEmptyProperties bool`
  - `MaxFailures int`
- `type Report struct`
  - `Format Format`
  - `VerticesRead`, `EdgesRead`, `VerticesWritten`, `EdgesWritten int64`
  - `SkippedVertices`, `SkippedEdges int64`
  - `Failures []Failure`
  - `OmittedFailures int64`
  - `Elapsed time.Duration`
- `type Failure struct`
  - `Phase Phase`
  - `Severity Severity`
  - `Location Location`
  - `Field string`
  - `RecordID string`
  - `Summary string`
- `type Phase string`
  - `PhaseReadVertex`
  - `PhaseReadEdge`
  - `PhaseWriteVertex`
  - `PhaseWriteEdge`
  - `PhaseValidate`
- `type Severity string`
  - `SeverityError`
  - `SeverityWarning`

`Record` is a one-of value. `Validate` must reject records with missing values,
both values, or a mismatch between `Kind` and the populated graph value.

The package should expose redacted sentinel errors:

- `ErrInvalidRecord`
- `ErrInvalidFormat`
- `ErrInvalidOptions`
- `ErrDuplicateVertex`
- `ErrMissingEndpoint`
- `ErrMalformedInput`
- `ErrStreamClosed`

I/O errors must wrap causes with `%w`. Validation and parse errors must support
`errors.Is` against both graph sentinels and graphio sentinels where useful.
No error type may retain raw input lines, raw property values, or secret-bearing
stringer values.

The package must also expose a concrete redacted error type:

- `type Error struct`
  - `Kind error`
  - `Format Format`
  - `Phase Phase`
  - `Location Location`
  - `Field string`
  - `RecordID string`
  - `Summary string`
  - `Cause error`
- `func (e *Error) Error() string`
- `func (e *Error) Unwrap() []error`

The public caller shape must compile:

```go
var ge *graphio.Error
if errors.As(err, &ge) {
    // inspect ge.Kind, ge.Format, ge.Phase, ge.Location
}
```

`errors.As` must work for parser, validation, option, and stream state failures.
`RecordID` is optional redacted correlation data, not a log-safety guarantee.
It must never retain secret-bearing IDs verbatim in tests, and it must be
bounded by default: truncate or hash long IDs and use a redacted placeholder
when the implementation cannot prove the value is safe. Raw line content and
property values remain forbidden.
`Location` must be structured:

- `type Location struct`
  - `Line int64`
  - `Row int64`
  - `Column string`
  - `FileRole FileRole`
- `type FileRole string`
  - `FileRoleVertices`
  - `FileRoleEdges`
  - `FileRoleStream`

`Report.Failures` is bounded. `MaxFailures == 0` means the default cap of 100
retained failures. Negative values are invalid. When more failures occur, the
report increments `OmittedFailures` and continues only if the active policy
allows skipping; fail policies still return immediately.

Read option zero values are safe:

- duplicate vertices fail by default;
- missing endpoints fail by default;
- endpoint validation is on by default and can only be relaxed by setting
  `MissingEndpointPolicy` to `MissingEndpointSkipEdge`;
- `MaxLineBytes == 0` means 1 MiB for NDJSON;
- `MaxRecordBytes == 0` means 1 MiB for CSV logical records before CSV field
  allocation;
- `MaxFieldBytes == 0` means 256 KiB for individual CSV fields;
- `MaxColumns == 0` means 1024 CSV columns;
- `MaxRecords == 0` means the default record cap of 1,000,000 records;
- `MaxRecords == UnlimitedRecords` is the explicit opt-in for no record-count
  cap and is only appropriate for trusted, externally bounded inputs.

All negative limits are invalid configuration and return `ErrInvalidOptions`
before reading or writing, except `MaxRecords == UnlimitedRecords`.

### NDJSON API

NDJSON uses one JSON object per line. Each object is an envelope:

```json
{"type":"vertex","id":"v1","label":"Person","properties":{"name":"Alice","age":30}}
{"type":"edge","id":"e1","label":"KNOWS","from":"v1","to":"v2","properties":{"since":2020}}
```

Expected helpers:

- `func WriteNDJSON(ctx context.Context, writer io.Writer, records []Record, options WriteOptions) (Report, error)`
- `func ReadNDJSON(ctx context.Context, reader io.Reader, options ReadOptions) ([]Record, Report, error)`
- `type NDJSONWriter struct`
  - `func NewNDJSONWriter(ctx context.Context, writer io.Writer, options WriteOptions) *NDJSONWriter`
  - `func (w *NDJSONWriter) WriteRecord(record Record) error`
  - `func (w *NDJSONWriter) Close() (Report, error)`
- `type NDJSONReader struct`
  - `func NewNDJSONReader(ctx context.Context, reader io.Reader, options ReadOptions) *NDJSONReader`
  - `func (r *NDJSONReader) ReadRecord() (Record, error)`
  - `func (r *NDJSONReader) Close() (Report, error)`

Convenience slice helpers are allowed, but streaming reader/writer types own
the main contract and tests. The `ReadNDJSON` convenience helper is the
validated whole-import path and may return the complete `[]Record` only after
all records have been validated.

All reader and writer constructors accept caller-owned `io.Reader`/`io.Writer`
values. `Close` finalizes graphio state and reports; it does not close the
underlying reader or writer.

Streaming endpoint contract:

- `NDJSONWriter` writes records in caller order, but `WriteNDJSON` must emit all
  vertices before edges to create a validated stream.
- `NDJSONReader.ReadRecord` is a validated streaming reader. With zero-value
  options it requires every edge endpoint to reference a vertex already returned
  by the same reader.
- An edge before its vertex is `ErrMissingEndpoint` under the default fail
  policy. Under `MissingEndpointSkipEdge`, it is skipped and not returned.
- Full unordered NDJSON import that buffers edges until EOF is explicitly
  deferred. This keeps memory bounded and makes the first Go stream contract
  simple.

Reader EOF behavior must be explicit:

- After logical EOF, `ReadRecord` returns `io.EOF`.
- Calling `ReadRecord` again after EOF returns `io.EOF` and must not advance
  state or mutate counts.
- Calling `ReadRecord` after `Close` returns `ErrStreamClosed`.
- Calling `Close` twice returns the same final report and no panic.
- Calling `WriteRecord` after `Close` returns `ErrStreamClosed`, does not write
  bytes, and does not mutate counts.
- Calling writer `Close` twice returns the same final report and no panic.

Truncated final input must be tested separately from clean EOF. A final line
without a trailing newline is accepted if it is complete JSON. A final line that
contains incomplete JSON returns `ErrMalformedInput`.

`MaxLineBytes` is a safety guard for NDJSON. A zero value uses the 1 MiB
default. Exceeding the limit returns `ErrMalformedInput` without including the
raw line in the error.

### CSV API

CSV uses a paired stream contract:

- Vertices: `id,label,<properties...>`
- Edges: `id,label,from,to,<properties...>`

Expected helpers:

- `func WriteCSV(ctx context.Context, streams CSVWriterStreams, records []Record, options CSVWriteOptions) (Report, error)`
- `func ReadCSV(ctx context.Context, streams CSVReaderStreams, options CSVReadOptions) ([]Record, Report, error)`
- `type CSVWriter struct`
  - `func NewCSVWriter(ctx context.Context, streams CSVWriterStreams, options CSVWriteOptions) *CSVWriter`
  - `func (w *CSVWriter) WriteVertex(vertex graph.Vertex) error`
  - `func (w *CSVWriter) WriteEdge(edge graph.Edge) error`
  - `func (w *CSVWriter) Close() (Report, error)`
- `type CSVReader struct`
  - `func NewCSVReader(ctx context.Context, streams CSVReaderStreams, options CSVReadOptions) *CSVReader`
  - `func (r *CSVReader) ReadVertex() (graph.Vertex, error)`
  - `func (r *CSVReader) ReadEdge() (graph.Edge, error)`
  - `func (r *CSVReader) Close() (Report, error)`
- `type CSVWriterStreams struct`
  - `Vertices io.Writer`
  - `Edges io.Writer`
- `type CSVReaderStreams struct`
  - `Vertices io.Reader`
  - `Edges io.Reader`
- `type CSVWriteOptions struct`
  - `WriteOptions`
  - `PropertyMode CSVPropertyMode`
  - `PropertyPrefix string`
  - `RawPropertiesColumn string`
  - `FormulaPolicy CSVFormulaPolicy`
  - `PropertyColumns []string`
- `type CSVReadOptions struct`
  - `ReadOptions`
  - `PropertyMode CSVPropertyMode`
  - `PropertyPrefix string`
  - `RawPropertiesColumn string`
- `type CSVPropertyMode string`
  - `CSVPropertiesPrefixedColumns`
  - `CSVPropertiesRawJSONColumn`
  - `CSVPropertiesNone`
- `type CSVFormulaPolicy string`
  - `CSVFormulaEscape`
  - `CSVFormulaRaw`

Default CSV mode should be prefixed property columns with prefix `prop.`.
`RawPropertiesColumn == ""` means the default raw JSON column name
`properties`. Raw JSON property column is included because it is useful for
round trips and maps cleanly to Go's `encoding/json`. CSV property typing is
limited:

- Prefixed columns read values as strings.
- Raw JSON property column preserves JSON numbers, booleans, arrays, objects,
  and null as the standard `encoding/json` decoded Go types.
- Writer must reject non-JSON-serializable property values in raw JSON mode.
- CSV docs must warn that prefixed columns are better for spreadsheet
  inspection, while raw JSON is better for type-preserving round trips.

`CSVFormulaEscape` is the default safe export policy. It prefixes a single quote
to string cells that begin with `=`, `+`, `-`, `@`, tab, carriage return, or
newline. `CSVFormulaRaw` is the explicit lossless graph-interchange opt-in for
callers that do not target spreadsheet tools and have their own export
sanitization boundary. Reads do not automatically unescape the formula prefix;
docs and examples must state that escaped exports are intended for spreadsheet
inspection and are not guaranteed to be lossless round-trip input. Raw JSON
property mode still escapes scalar CSV cells such as IDs and labels by default,
while the JSON payload itself is treated as a JSON string field.

CSV import reads vertices before edges. Endpoint validation is fail-closed by
default: edges whose `from` or `to` vertex ID was not seen in the vertex stream
trigger `ErrMissingEndpoint` or are skipped according to
`MissingEndpointPolicy`. Duplicate vertex IDs fail or skip according to
`DuplicateVertexPolicy`.

The slice helpers are convenience APIs. Stateful `CSVReader` and `CSVWriter`
own the streaming API contract. `CSVWriter` requires either `PropertyColumns`
to be set before the first row or a caller-provided finite helper input so the
header is known before rows are written. True unbounded CSV writing with
late-discovered property columns is deferred. `CSVReader` reads vertices and
edges by file role without requiring all records in memory; the convenience
helper returns a deterministic vertices-then-edges slice after validation.

CSV read limits must be enforced before unbounded `encoding/csv.Reader`
allocation. The implementation must wrap each underlying reader in a bounded
logical-record reader that stops after `MaxRecordBytes` while respecting quoted
newlines, then pass only the bounded record bytes to `encoding/csv.Reader`.
`MaxFieldBytes` is checked after parsing the bounded record. Oversized quoted
fields and records must return typed redacted errors without retaining the raw
record.

CSV writer success requires flushing both underlying `encoding/csv.Writer`
instances and checking their `Error()` values before returning a successful
report. Counts become final only after the relevant writer is flushed
successfully. Atomic paired-file replacement is out of scope; if the edge writer
fails after vertices have been flushed, the function returns a partial report
and a wrapped error, and docs must state that caller-owned path/rollback policy
is required.

### Unsupported capabilities

GraphML is documentation-only in #49. Do not add `FormatGraphML`, public GraphML
helpers, compression helpers, encryption helpers, path ownership helpers, or
atomic file replacement helpers. Examples must route unsupported requests by
application-level format/capability selection before calling NDJSON/CSV helpers.
Do not add stubbed partial XML parsing.

Compression, encryption, path/file ownership, and atomic writes are documented
as future wrappers around stream APIs, not implemented in #49.

## Data Flow

NDJSON write:

1. Caller creates `graph.Vertex` and `graph.Edge` values.
2. Caller wraps them in `Record` values.
3. `NDJSONWriter.WriteRecord` checks context cancellation.
4. The writer validates the record and graph value.
5. The writer writes exactly one JSON envelope plus `\n`.
6. Counts are updated only after a successful write.
7. `Close` freezes the report.

NDJSON read:

1. `NDJSONReader.ReadRecord` checks context cancellation.
2. The reader reads one logical line with the configured line limit.
3. Blank lines are malformed input rather than silently ignored.
4. The reader decodes the envelope and creates `graph.Vertex` or `graph.Edge`.
5. Duplicate vertex and missing endpoint policies fail closed by default.
   `MissingEndpointSkipEdge` skips only edges whose endpoints have not already
   been seen in the same stream.
6. The returned record is valid and owns shallow-copied graph properties through
   the underlying `graph` constructors.

CSV write:

1. Caller passes named `CSVWriterStreams` so vertex and edge writers are not
   adjacent same-type parameters.
2. Writer discovers headers from finite records or uses configured
   `PropertyColumns` for the selected property mode.
3. Writer writes vertex header and rows, then edge header and rows.
4. Context is checked between records and before each writer phase.
5. Writers are flushed and `Error()` is checked before success is reported.
6. `CSVWriter.Close` is idempotent, and post-close writes return
   `ErrStreamClosed` without mutating counts.

CSV read:

1. Read and validate the vertex header.
2. Read each logical CSV record through the pre-allocation record byte guard.
3. Decode vertex rows, applying duplicate policy.
4. Read and validate the edge header.
5. Read each edge record through the pre-allocation record byte guard.
6. Decode edge rows, applying the fail-closed missing endpoint policy by
   default.
7. Return vertices followed by edges to keep deterministic round trips in the
   convenience helper.

## Error Handling

- `context.Canceled` and `context.DeadlineExceeded` must be returned unchanged
  or wrapped so `errors.Is` works.
- Cancellation must not be retried or converted into partial success.
- Reader and writer close methods must be idempotent.
- Malformed JSON, invalid CSV shape, invalid record kind, duplicate vertex, and
  missing endpoint must be typed.
- Failure reports must include phase, field, record ID when safe, and redacted
  summary. They must not include raw lines or property values.
- Failure reports must respect `MaxFailures` and increment `OmittedFailures`
  after the cap is reached.
- Slice convenience helpers should return both the partial report and the error
  when they fail after reading or writing some records.
- Context cancellation is cooperative for arbitrary `io.Reader` and `io.Writer`
  values. The package checks context before each logical read/write and between
  records, but it cannot preempt a blocking reader or writer that ignores
  context. Tests must cover cooperative blocking fakes and docs must tell
  callers to use deadline-aware readers/writers for hard interruption.

## Security And Trust Boundaries

- NDJSON and CSV are untrusted input boundaries.
- No raw input line is retained in an error or failure report.
- Property values decoded from JSON are caller-owned data, not sanitized.
- CSV formula-injection protection is automatic for writes by default through
  `CSVFormulaEscape`. Docs must warn callers who opt into `CSVFormulaRaw` to
  sanitize values at their application boundary before opening exports in
  spreadsheet tools.
- `MaxLineBytes` exists to avoid unbounded NDJSON line allocation.
- CSV record, field, column, and retained-failure caps exist to avoid unbounded
  CSV memory growth. Record caps must be enforced before `encoding/csv.Reader`
  can allocate an arbitrarily large quoted field or logical record.
- `MaxRecords` defaults to 1,000,000 to bound endpoint-validation state. The
  `UnlimitedRecords` sentinel is reserved for trusted, externally bounded input
  sources.
- CSV header union is computed from in-memory records for the convenience
  writer unless `PropertyColumns` is supplied; true late-header streaming CSV
  writing without precomputed headers is a future design.

## Performance And Memory Boundaries

- NDJSON reader/writer are streaming and should not require loading the full
  input except for the convenience slice helpers.
- NDJSON endpoint validation stores seen vertex IDs and therefore grows with the
  vertex count. This is the explicit cost of fail-closed validation.
- CSV convenience writer must inspect finite records to compute property headers
  unless `PropertyColumns` is supplied. This is acceptable for #49 but must be
  documented.
- CSV reader stores seen vertex IDs for fail-closed endpoint validation. This
  grows with vertex count and must be documented.
- `Report.Failures` is bounded by `MaxFailures`; omitted failures are counted
  instead of retained.
- No benchmark ranking is claimed in #49. Add a measurement plan for a future
  benchmark issue if docs mention large graph performance.

## Test Requirements

Unit tests must cover:

- NDJSON vertex and edge round trips.
- NDJSON malformed JSON, unknown type, blank line, over-limit line, missing
  scalar fields, invalid graph values, duplicate vertices, missing endpoints,
  clean EOF, final complete line without newline, truncated final JSON,
  post-EOF reuse, post-close reuse, double close, and cancellation.
- CSV prefixed-column round trip with explicit `CSVFormulaRaw`.
- CSV raw JSON property round trip preserving bool, number, object, array, and
  null values.
- CSV stateful `CSVReader`/`CSVWriter` direct usage, including explicit
  `PropertyColumns` for streaming writes, missing-header setup errors, vertex
  EOF before edge reads, repeated EOF, post-close `ErrStreamClosed`, double
  close idempotency, stable final reports, and no count mutation after terminal
  state.
- CSV missing required headers, malformed rows, duplicate vertices, missing
  endpoints, unknown property mode, oversized records, oversized fields,
  excessive columns, default excessive-record cap, explicit `UnlimitedRecords`
  opt-in, formula escaping as the safe default and non-round-trip spreadsheet
  export, oversized quoted record pre-allocation guard, writer flush failure,
  partial paired-export failure, and cancellation.
- Redaction checks for NDJSON/CSV malformed input and secret-bearing property
  values and secret-bearing IDs.
- Large streaming graph test using many records without relying on benchmark
  claims.
- Bounded failure accumulation tests for skip policies.
- Zero-value `ReadOptions` tests proving duplicate vertices and missing
  endpoints fail closed.
- `GoroutineStressTester` coverage for repeated independent NDJSON/CSV
  round trips.
- `AsyncJobTester` coverage for cancellation on long read/write paths.
- `go test -race -count=1 ./graph/graphio`.

Documentation tests/examples must show:

- NDJSON stream usage.
- CSV paired stream usage, including direct `CSVReader`/`CSVWriter` usage.
- `errors.Is` and `errors.As` handling.
- Documentation-only GraphML/compression/encryption routing.
- Format selection guidance.

## Documentation And Bookkeeping

Update:

- `graph/graphio/README.md`
- `graph/graphio/README.ko.md`
- `graph/README.md`
- `graph/README.ko.md`
- root `README.md`
- root `README.ko.md`
- `CHANGELOG.md`
- `WIP.md`
- `docs/lessons/2026-06-30-graph-io-boundaries.md`

The README pair must keep English and Korean content aligned. The root README
must list `graph/graphio` as active only after implementation exists.

## 검증 Plan

Targeted validation:

- `go test -count=1 ./graph ./graph/graphio`
- `go test -race -count=1 ./graph/graphio`
- `go test -run 'Test.*Stress|Test.*Cancellation' -count=10 ./graph/graphio`
- `go vet ./graph/graphio`
- `golangci-lint run ./graph/graphio`
- `go doc ./graph/graphio`
- `git diff --check`

Repo validation before PR:

- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make test`
- `make race`
- `make ci`

## Acceptance Mapping

| #49 criterion | Design mapping |
|---|---|
| Implement or defer CSV, NDJSON, GraphML, streaming APIs | NDJSON and CSV stream APIs implemented; GraphML, compression, encryption explicitly deferred without public constants or helpers. |
| Malformed input tests | NDJSON/CSV malformed, truncated, invalid header, invalid row, and over-limit line tests required. |
| Large streaming graph tests | Large NDJSON/CSV tests required without benchmark claims. |
| Property typing | NDJSON preserves JSON types; CSV prefixed columns read strings; CSV raw JSON preserves JSON types. |
| Missing endpoints | `MissingEndpointPolicy` defaults to fail and endpoint validation is on by default. |
| Duplicate IDs | `DuplicateVertexPolicy` defaults to fail. |
| Round trips | NDJSON, CSV prefixed with explicit `CSVFormulaRaw`, and CSV raw JSON round trips required. |
| Context cancellation | Reader/writer methods and convenience helpers accept context and cancellation tests are required. |
| Format guidance | README pair must include format selection and caveats. |
| Measurement plan | No performance claims; docs record future benchmark requirement. |

## Open Questions Resolved By This Spec

- Package name: `graph/graphio`.
- First formats: NDJSON and CSV.
- Deferred formats: GraphML, compression, encryption, path ownership, atomic
  writes, without public constants or helpers in #49.
- Property typing: JSON-preserving for NDJSON and CSV raw JSON, string-only for
  CSV prefixed columns.
- Endpoint validation: fail-closed, vertices-before-edges for validated streams.
- CSV API: named stream structs, bounded untrusted input, bounded failures, and
  explicit flush/partial-export semantics.
- Backend integration: out of scope.
