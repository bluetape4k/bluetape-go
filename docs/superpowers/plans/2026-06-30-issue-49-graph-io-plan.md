# Issue #49 Graph I/O Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use test-driven-development
> before implementation edits. Steps use checkbox (`- [ ]`) syntax for
> tracking.

**Goal:** Add `graph/graphio` with safe, streaming NDJSON and CSV graph I/O
helpers while explicitly deferring GraphML, compression, encryption, path
ownership, atomic file replacement, and any public constants or helpers for
those deferred capabilities.

**Architecture:** Keep graph model values in `graph`. Add I/O as a sibling
subpackage that owns wire formats, options, reports, redacted typed errors,
streaming readers/writers, and documentation. Use the standard library only.

**Tech Stack:** Go 1.26.3, standard library `encoding/json`, `encoding/csv`,
`bufio`, `io`, `context`, existing `graph` values, existing
`testing/concurrency` helpers.

---

## File Structure

- Create `graph/graphio/doc.go`: package overview, supported formats, deferred
  capabilities, and caller-owned stream contract.
- Create `graph/graphio/errors.go`: graphio sentinels, structured `Error`,
  `Location`, `FileRole`, and redacted ID handling.
- Create `graph/graphio/options.go`: formats, options, defaults, validation,
  policy enums, and limit helpers.
- Create `graph/graphio/records.go`: `Record`, record constructors,
  validation, internal envelope conversions, and property helpers.
- Create `graph/graphio/report.go`: `Report`, `Failure`, bounded failure
  accumulation, elapsed timing.
- Create `graph/graphio/ndjson.go`: NDJSON reader, writer, and slice helpers.
- Create `graph/graphio/csv.go`: CSV paired-stream reader, writer, and helpers.
- Create `graph/graphio/example_test.go`: compileable examples for streams,
  errors, and format selection.
- Create `graph/graphio/*_test.go`: focused unit tests for common, NDJSON,
  CSV, limits, redaction, cancellation, and stress paths.
- Create `graph/graphio/README.md` and `graph/graphio/README.ko.md`.
- Modify `graph/README.md` and `graph/README.ko.md`.
- Modify root `README.md` and `README.ko.md`.
- Modify `CHANGELOG.md` and `WIP.md`.
- Add `docs/lessons/2026-06-30-graph-io-boundaries.md`.

## Task 1: Common Types, Options, Errors, And Reports

**complexity:** high

**Files:**
- Create: `graph/graphio/errors.go`
- Create: `graph/graphio/options.go`
- Create: `graph/graphio/records.go`
- Create: `graph/graphio/report.go`
- Test: `graph/graphio/errors_test.go`
- Test: `graph/graphio/options_test.go`
- Test: `graph/graphio/records_test.go`
- Test: `graph/graphio/report_test.go`

- [ ] **Step 1: Write failing tests for public contracts**

Tests must prove:

- `VertexRecord` and `EdgeRecord` accept valid `graph.Vertex`/`graph.Edge`
  values and reject zero or mismatched records with `ErrInvalidRecord`.
- `Record.Validate` rejects missing values, both values, kind/value mismatch,
  and invalid graph values.
- `ReadOptions` zero values normalize to fail-closed policies and limits:
  `MaxLineBytes=1MiB`, `MaxRecordBytes=1MiB`, `MaxFieldBytes=256KiB`,
  `MaxColumns=1024`, `MaxRecords=1_000_000`, `MaxFailures=100`.
- `UnlimitedRecords == -1` is accepted only for `MaxRecords`.
- Other negative limits return `ErrInvalidOptions`.
- `Report` retains at most the configured failure cap and increments
  `OmittedFailures` after the cap.
- `Error` supports `errors.Is` for graphio sentinels and graph sentinels and
  supports `errors.As` with:

```go
var ge *graphio.Error
if !errors.As(err, &ge) {
    t.Fatalf("expected graphio.Error")
}
```

- Redaction tests use secret-bearing raw input, property values, IDs, and
  `fmt.Stringer` values and assert that `err.Error()`, `%+v`, `Error`, and
  `Failure` fields do not retain the secret verbatim.

- [ ] **Step 2: Run red tests**

Run: `go test -count=1 ./graph/graphio`

Expected: FAIL because package `graph/graphio` does not exist.

- [ ] **Step 3: Implement common files**

Implement:

- `FormatNDJSON`, `FormatCSV`.
- `RecordKind`, `Record`, `VertexRecord`, `EdgeRecord`.
- `DuplicateVertexPolicy`, `MissingEndpointPolicy`.
- `ReadOptions`, `WriteOptions`, CSV option enums, `UnlimitedRecords`.
- `ErrInvalidRecord`, `ErrInvalidFormat`, `ErrInvalidOptions`,
  `ErrDuplicateVertex`, `ErrMissingEndpoint`, `ErrMalformedInput`,
  `ErrStreamClosed`.
- `Location`, `FileRole`, `Error`, `Failure`, `Report`.
- Internal option normalization and bounded failure helpers.
- Redacted record-ID helper. Prefer short bounded IDs when they are already
  ordinary graph IDs; truncate/hash or replace secret-like/long IDs.

- [ ] **Step 4: Run green tests**

Run: `go test -count=1 ./graph ./graph/graphio`

Expected: PASS for common graphio tests and existing graph tests.

## Task 2: NDJSON Streaming Codec

**complexity:** high

**Files:**
- Create: `graph/graphio/ndjson.go`
- Test: `graph/graphio/ndjson_test.go`

- [ ] **Step 1: Write failing NDJSON tests**

Tests must cover:

- `WriteNDJSON` emits vertices before edges even if input records are mixed.
- `NDJSONWriter.WriteRecord` writes caller order, validates each record, checks
  context, updates counts only after successful writes, and returns
  `ErrStreamClosed` after `Close`.
- `NDJSONReader.ReadRecord` reads one line at a time, returns `io.EOF`
  repeatedly after EOF, returns `ErrStreamClosed` after close, and makes double
  close idempotent.
- Valid vertex and edge envelopes round trip.
- Unknown type, blank line, malformed JSON, truncated final JSON, over-limit
  line, missing scalar fields, invalid graph values, duplicate vertices, missing
  endpoints, and excessive records return typed redacted errors.
- Final complete JSON without trailing newline is accepted.
- `MissingEndpointSkipEdge` skips only edges whose endpoints were not already
  seen in the same stream.
- `DuplicateVertexSkip` skips duplicate vertices and does not emit them.
- `ReadNDJSON` returns partial report plus error on failure.
- Context cancellation is cooperative and uses fakes that unblock on context.

- [ ] **Step 2: Run red tests**

Run: `go test -run 'TestNDJSON' -count=1 ./graph/graphio`

Expected: FAIL because NDJSON code is not implemented.

- [ ] **Step 3: Implement NDJSON**

Implement `NDJSONReader`, `NDJSONWriter`, `ReadNDJSON`, and `WriteNDJSON`.
Use bounded line reads, explicit envelope structs, graph constructors, seen
vertex ID state, bounded failures, and redacted typed errors. Do not implement
unordered edge buffering.

- [ ] **Step 4: Run green tests**

Run:

```bash
go test -run 'TestNDJSON|TestGraphIOCommon|TestRecord|TestReport|TestOptions' -count=1 ./graph/graphio
go test -count=1 ./graph ./graph/graphio
```

Expected: PASS.

## Task 3: CSV Paired-Stream Codec

**complexity:** high

**Files:**
- Create: `graph/graphio/csv.go`
- Test: `graph/graphio/csv_test.go`

- [ ] **Step 1: Write failing CSV tests**

Tests must cover:

- `WriteCSV` and `ReadCSV` use `CSVWriterStreams`/`CSVReaderStreams`.
- Direct `CSVWriter` usage with explicit `PropertyColumns` writes headers,
  vertices, edges, flushes on `Close`, and reports final counts.
- Direct prefixed-column `CSVWriter` without `PropertyColumns` fails before
  writing data unless the finite helper path can discover headers.
- Direct `CSVReader` usage reads vertices before edges, returns repeated
  `io.EOF` after vertex EOF, then allows edge reads, and reports missing
  endpoints fail-closed by default.
- `CSVReader.Close` and `CSVWriter.Close` are idempotent; post-close
  `ReadVertex`, `ReadEdge`, `WriteVertex`, and `WriteEdge` return
  `ErrStreamClosed`, reuse the stable final report, and do not mutate counts.
- Default mode is prefixed property columns with `prop.` and default
  `CSVFormulaEscape`, protecting spreadsheet exports by default.
- Lossless prefixed-column round trips require explicit `CSVFormulaRaw`.
- Raw JSON property column defaults to `properties` and preserves bool, number,
  object, array, and null through `encoding/json` decoded Go types.
- `CSVFormulaEscape` prefixes formula-like scalar cells and is documented/tested
  as the safe default, non-lossless spreadsheet export.
- Missing required headers, duplicate headers, malformed rows, unknown property
  mode, oversized record, oversized quoted record before CSV allocation,
  oversized field, excessive columns, default excessive record cap, and invalid
  options return typed redacted errors.
- Duplicate vertices and missing endpoints fail by default and skip only under
  their explicit skip policies.
- Writer flush failures on either stream are checked through
  `encoding/csv.Writer.Error`.
- Edge-writer failure after vertex flush returns a partial report and wrapped
  cause.
- Cancellation is checked between records and before each writer phase.

- [ ] **Step 2: Run red tests**

Run: `go test -run 'TestCSV' -count=1 ./graph/graphio`

Expected: FAIL because CSV code is not implemented.

- [ ] **Step 3: Implement CSV**

Implement `CSVReader`, `CSVWriter`, `WriteCSV`, and `ReadCSV` with named stream
structs, property-mode normalization, pre-allocation logical-record byte
guards, finite header discovery from `records` or `PropertyColumns`, JSON raw
property support, formula escaping, seen vertex ID validation, bounded failures,
and partial report handling.

CSV reads must not rely on post-parse size checks alone. Build a bounded
logical-record reader that stops at `MaxRecordBytes` while respecting quoted
newlines, then parse only that bounded byte slice with `encoding/csv.Reader`.
Check `MaxFieldBytes` after parsing. Add adversarial tests for oversized quoted
fields/records proving typed redacted errors and bounded retained memory.

- [ ] **Step 4: Run green tests**

Run:

```bash
go test -run 'TestCSV|TestNDJSON|TestGraphIOCommon|TestRecord|TestReport|TestOptions' -count=1 ./graph/graphio
go test -count=1 ./graph ./graph/graphio
```

Expected: PASS.

## Task 4: Examples, Stress, Cancellation, And Race Coverage

**complexity:** medium

**Files:**
- Create: `graph/graphio/example_test.go`
- Test: `graph/graphio/stress_test.go`
- Test: `graph/graphio/cancellation_test.go`

- [ ] **Step 1: Write failing examples and stress tests**

Examples must compile and show:

- NDJSON stream writer/reader usage.
- CSV paired stream usage with direct `CSVReader`/`CSVWriter` usage.
- `errors.Is` and `errors.As` handling.
- Format selection where GraphML/compression/encryption route to deferred
  application-level handling, not public graphio constants or stubs.

Stress and cancellation tests must cover:

- `testing/concurrency.GoroutineStressTester` repeated independent NDJSON/CSV
  round trips.
- `testing/concurrency.AsyncJobTester` long read/write cancellation paths.
- Large streaming graph test using many records without benchmark claims.

- [ ] **Step 2: Run red tests**

Run:

```bash
go test -run 'Example|Test.*Stress|Test.*Cancellation|Test.*Large' -count=1 ./graph/graphio
```

Expected: FAIL until examples and helper paths are complete.

- [ ] **Step 3: Implement examples and any missing cancellation hooks**

Keep examples concise and stable. Avoid performance claims.

- [ ] **Step 4: Run green and race tests**

Run:

```bash
go test -run 'Example|Test.*Stress|Test.*Cancellation|Test.*Large' -count=10 ./graph/graphio
go test -race -count=1 ./graph/graphio
```

Expected: PASS.

## Task 5: Documentation And Release Bookkeeping

**complexity:** medium

**Files:**
- Create: `graph/graphio/README.md`
- Create: `graph/graphio/README.ko.md`
- Modify: `graph/README.md`
- Modify: `graph/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`
- Modify: `WIP.md`
- Add: `docs/lessons/2026-06-30-graph-io-boundaries.md`

- [ ] **Step 1: Update docs from real implementation**

Document:

- Supported formats: NDJSON and CSV.
- Deferred capabilities: GraphML, compression, encryption, path ownership,
  atomic file replacement, backend integration.
- Zero-value fail-closed defaults and bounded import defaults.
- Property typing differences between NDJSON, CSV prefixed columns, and CSV raw
  JSON.
- CSV formula policy: safe escape default for spreadsheet exports, explicit raw
  opt-in for lossless graph interchange.
- Context cancellation caveat for blocking readers/writers.
- Format selection guidance for beginners.

Keep English and Korean README pairs aligned.

- [ ] **Step 2: Update changelog and WIP**

Record the new package and public behavior under the active unreleased section.

- [ ] **Step 3: Add a lesson**

Capture the boundary decision: graphio owns format/trust concerns; graph values
remain model-only; deferred capabilities must remain additive.

- [ ] **Step 4: Verify docs**

Run:

```bash
go test -run Example -count=1 ./graph/graphio
git diff --check
```

Expected: PASS.

## Task 6: Final Verification

**complexity:** high

- [ ] **Step 1: Targeted gates**

Run:

```bash
go test -count=1 ./graph ./graph/graphio
go test -race -count=1 ./graph/graphio
go test -run 'Test.*Stress|Test.*Cancellation' -count=10 ./graph/graphio
go vet ./graph/graphio
golangci-lint run ./graph/graphio
go doc ./graph/graphio
git diff --check
```

- [ ] **Step 2: Repo gates**

Run serially:

```bash
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
```

- [ ] **Step 3: Step 6-R code review**

Run the 7-Tier implementation review:

- Performance: streaming, limits, failure caps, race behavior.
- Stability: EOF/close/cancellation/partial reports.
- Security: redaction, formula mode, untrusted input bounds.
- Operator/Ops: docs, validation evidence, failure observability.
- Developer/API: Go docs, exported API shape, error contracts.
- User/Caller: beginner docs, round-trip behavior, deferred capability clarity.
- Main integration: final P0/P1 verdict.

Do not create the PR until Step 6-R has P0=0/P1=0.

## Acceptance Mapping

| #49 criterion | Plan mapping |
|---|---|
| CSV, NDJSON, GraphML, streaming APIs | Implement NDJSON/CSV streaming readers/writers and slice helpers; explicitly defer GraphML and non-stream wrappers without public constants or helpers. |
| Malformed input | NDJSON and CSV malformed/over-limit/header/row/truncation tests. |
| Large streaming graphs | Large NDJSON/CSV streaming and stress tests. |
| Property typing | NDJSON JSON types, CSV string prefixed columns, CSV raw JSON typed values. |
| Missing endpoints | Fail-closed default and explicit skip policy tests. |
| Duplicate IDs | Fail-closed default and explicit skip policy tests. |
| Round trips | NDJSON, CSV prefixed with explicit `CSVFormulaRaw`, and CSV raw JSON round trips. |
| Context cancellation | Context-aware reader/writer helpers and cooperative cancellation tests. |
| Docs | `graph/graphio` README pair, graph README pair, root README pair, examples. |
| Measurement plan | No performance claim; docs/lesson point to future benchmark work. |

## PR Readiness Checklist

- [ ] Spec and Step 2-R review are committed or included in the PR.
- [ ] Plan and Step 3-R review are committed or included in the PR.
- [ ] Implementation and tests pass targeted gates.
- [ ] Docs and bookkeeping are updated.
- [ ] Step 6-R review records P0=0/P1=0.
- [ ] GitHub PR links #49 and carries issue milestone/labels/assignee.
