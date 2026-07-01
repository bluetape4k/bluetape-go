# Issue #49 Step 2-R Spec Review

Date: 2026-06-30
Worktree: `.worktrees/issue-49-graph-io`
Branch: `feat/issue-49-graph-io`
Baseline: `da6005c`

## Verdict

P0: 0
P1: 0

The Step 2 graph I/O specification is approved for Step 3 implementation
planning after targeted corrections.

## Initial Review

| Perspective | P0 | P1 | P2 | Resolution |
| --- | ---: | ---: | ---: | --- |
| Performance | 0 | 1 | 2 | Removed unsafe unbounded defaults by adding a default record cap and explicit `UnlimitedRecords` trust-boundary opt-in. |
| Stability | 0 | 1 | 2 | Made zero-value reads fail closed, documented caller-owned reader/writer ownership, and made writer post-close behavior explicit. |
| Security | 0 | 1 | 1 | Added default record cap for endpoint-validation state and redacted/bounded `RecordID` guidance. |
| Operator/Ops | 0 | 1 | 1 | Added bounded import defaults and explicit stream ownership/close responsibility. |
| Developer/API | 0 | 0 | 4 | Added `ErrInvalidOptions`, fixed the `errors.As` caller shape, removed stale validation wording, and named the raw JSON property default. |
| User/Caller | 0 | 1 | 2 | Split CSV formula safety from lossless interchange and clarified unsupported capability routing. |

## Corrections Applied

- #49 is scoped to standard-library NDJSON and CSV stream codecs under
  `graph/graphio`; GraphML, compression, encryption, path helpers, and atomic
  file replacement remain deferred.
- Normal NDJSON/CSV paths must not return `graph.ErrUnsupportedCapability`.
- Zero-value `ReadOptions` are bounded and fail closed: duplicate vertices and
  missing endpoints fail by default, and `MaxRecords == 0` caps reads at
  1,000,000 records.
- `UnlimitedRecords == -1` is the only no-record-count-cap opt-in and is
  reserved for trusted, externally bounded inputs.
- Invalid options return `ErrInvalidOptions`, separate from wire/format
  failures.
- `Failure.Location` and `Error.Location` use a structured `Location` value.
- `errors.As` examples use the compilable Go shape:
  `var ge *graphio.Error; errors.As(err, &ge)`.
- `RecordID` is optional redacted correlation data and must not retain
  secret-bearing IDs verbatim.
- `CSVFormulaEscape` is the default safe spreadsheet export policy.
  `CSVFormulaRaw` is an explicit lossless graph-interchange opt-in and is not
  spreadsheet-safe by default.
- CSV raw JSON mode defaults `RawPropertiesColumn` to `properties`.
- CSV record byte limits must be enforced before `encoding/csv.Reader` can
  allocate arbitrarily large quoted fields or logical records.
- GraphML remains documentation-only in #49; no `FormatGraphML` constant or
  GraphML public helper is added.
- CSV and NDJSON reader/writer constructors accept caller-owned streams; graphio
  `Close` finalizes graphio state but does not close the underlying stream.
- `NDJSONWriter.WriteRecord` after `Close` returns `ErrStreamClosed` without
  writing or mutating counts.

## Step 3-R Spec Corrections

Step 3-R exposed three additional P1 issues in the spec/plan boundary. The spec
was corrected before implementation planning passed:

- CSV writes default to `CSVFormulaEscape`; lossless `CSVFormulaRaw` is explicit.
- CSV reads require a pre-allocation logical-record byte guard before
  `encoding/csv.Reader`.
- CSV exports `CSVReader`/`CSVWriter` streaming types, and GraphML has no public
  constant or helper in #49.

## Rerun Evidence

| Perspective | P0 | P1 | Evidence |
| --- | ---: | ---: | --- |
| Performance | 0 | 0 | Accepted bounded defaults, `UnlimitedRecords`, streaming memory boundaries, CSV caps, and no benchmark-ranking claim. |
| Security | 0 | 0 | Accepted redaction, bounded malformed input, fail-closed defaults, safe formula default, explicit raw opt-in, and no public deferred helpers. |
| Stability/Ops | 0 | 0 | Accepted ownership, close/idempotency, partial reports, cancellation caveat, bounded imports, and failure reporting. |
| Developer/API | 0 | 0 | Accepted exported API shape, `ErrInvalidOptions`, `errors.As`, CSV defaults, and unsupported capability consistency. |
| User/Caller | 0 | 0 | Accepted explicit round-trip/formula mode split, NDJSON ordering, CSV paired streams, flush semantics, and docs expectations. |
| Main Integration | 0 | 0 | Current on-disk spec has no remaining P0/P1 contradictions across review lanes. |

## Follow-Up Gates

- Step 3-R must verify that the implementation plan preserves the bounded
  defaults, redaction contract, CSV round-trip policy, and deferred-capability
  boundary.
- Step 4 must write failing tests before implementation code.
- Step 6-R must review the actual implementation, docs, and examples against
  this spec and require P0=0/P1=0 before PR finalization.
