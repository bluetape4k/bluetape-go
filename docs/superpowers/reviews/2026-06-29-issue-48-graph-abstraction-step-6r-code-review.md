# Issue #48 Step 6-R Code Review

Date: 2026-06-29
Worktree: `.worktrees/issue-48-graph-abstraction`
Branch: `feat/issue-48-graph-abstraction`
Baseline: `4d48de3`

## Verdict

P0: 0
P1: 0

The implementation is approved for PR creation after final verification.

## Initial Review

| Perspective | P0 | P1 | P2 | P3 | Resolution |
| --- | ---: | ---: | ---: | ---: | --- |
| Performance | 0 | 0 | 1 | 1 | Removed redundant JSON serialization copies and preallocated path accessors while preserving zero-path nil accessors. |
| Stability | 0 | 1 | 1 | 0 | Rejected malformed path JSON with required-field checks and added null/missing-field tests. |
| Security | 0 | 0 | 0 | 1 | Documented that #48 JSON validation is not strict schema validation for untrusted I/O records. |
| Operator/Ops | 0 | 0 | 1 | 1 | Updated root README release status to `v0.9.0`/active `0.10.0` and added the planned lesson artifact. |
| Developer/API | 0 | 2 | 1 | 1 | Restored zero-path accessor contract, documented path continuity as out of scope, and pinned exact JSON wire shape. |
| User/Caller | 0 | 0 | 2 | 1 | Documented `Path` as a model container, explained raw parse helper scope, and aligned Korean scope wording. |

## Corrections Applied

- `Path.UnmarshalJSON` now rejects top-level `null`, missing `steps`, missing
  `total_weight`, `steps:null`, and non-empty edge paths with omitted weight.
- Exact JSON tests pin vertex, edge, path step, and path field names, including
  always-present `properties`.
- `Path` documentation states that endpoint continuity, alternating order, and
  traversal correctness belong to later algorithms or backend adapters.
- README pairs explain that JSON validation is not a strict schema validator for
  untrusted I/O records.
- Root README pairs now match the published `v0.9.0` status and active
  `0.10.0` graph milestone.
- `docs/lessons/2026-06-29-graph-model-api-boundaries.md` records the API
  boundary rationale.

## Rerun Evidence

| Perspective | P0 | P1 | Evidence |
| --- | ---: | ---: | --- |
| Stability | 0 | 0 | Rerun accepted required-field path JSON checks and malformed payload tests. |
| Developer/API | 0 | 0 | Rerun accepted model-only path scope, exact JSON shape, zero-path behavior, and reserved unsupported-capability contract. |

## Main Integration Review

No unresolved P0 or P1 findings remain. The branch keeps #48 scoped to model
values only and does not introduce repository, session, schema, query,
transaction, backend, algorithm, or capability interfaces.

Verification evidence collected before this review:

- `go test -count=1 ./graph`: PASS
- `go test -race -count=1 ./graph`: PASS
- `make fmt-check`: PASS
- `make tidy-check`: PASS
- `make vet`: PASS
- `make lint`: PASS
- `make test`: PASS
- `make race`: PASS
- `make ci`: PASS
- `go doc ./graph`: PASS
- `git diff --check`: PASS

## Follow-Up Notes

- #49 must define external I/O trust-boundary policy: unknown fields, duplicate
  fields, payload size limits, nested property copying, and malformed-record
  redaction.
- #50 must re-check backend ID conversion and avoid broad `any` or
  `fmt.Stringer` ID conversion helpers.
