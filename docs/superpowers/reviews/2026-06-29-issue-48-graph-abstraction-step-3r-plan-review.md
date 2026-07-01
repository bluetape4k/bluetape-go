# Issue #48 Step 3-R Plan Review

Date: 2026-06-29
Worktree: `.worktrees/issue-48-graph-abstraction`
Branch: `feat/issue-48-graph-abstraction`
Baseline: `20f9fbc`

## Verdict

P0: 0
P1: 0

The Step 3 implementation plan is approved for TDD implementation after
targeted spec and plan corrections.

## Initial Review

| Perspective | P0 | P1 | P2 | P3 | Resolution |
| --- | ---: | ---: | ---: | ---: | --- |
| Performance | 0 | 0 | 1 | 2 | Added nil/empty `Properties.Clone` expectations and stress-test N/A rationale. |
| Stability | 0 | 1 | 2 | 0 | Added scalar `ElementID`/`Label` JSON validation, zero-value no-panic tests, and secret-bearing redaction tests. |
| Security | 0 | 0 | 1 | 2 | Added secret-bearing redaction tests, property ownership tests/docs, and unsupported raw-ID fixtures. |
| Operator/Ops | 0 | 1 | 2 | 0 | Replaced raw repo-wide `go test ./...` completion gate with serial `make test`/`make race`/`make ci` gates and added PR evidence/release-support notes. |
| Developer/API | 0 | 2 | 2 | 1 | Removed public `NewEdgeEndpoints(start,end)` helper, fixed JSON wire shape, pinned reserved `ErrUnsupportedCapability`, and required exported Go docs. |
| User/Caller | 0 | 1 | 3 | 1 | Strengthened redaction proof, `MustElementID` guidance, README beginner sections, and unsupported capability routing. |

## Corrections Applied

- `EdgeEndpoints` is now constructed with named fields and validated through
  `EdgeEndpoints.Validate`; no public adjacent `start, end` helper is planned.
- JSON shape is part of the release contract for `ElementID`, `Label`, `Vertex`,
  `Edge`, `PathStep`, and `Path`.
- Scalar JSON decoding must validate through `NewElementID` and `NewLabel`.
- Redaction tests use actual secret-bearing invalid inputs and inspect
  `err.Error()`, formatted output, and exported `ValidationError` fields.
- `ValidationError` must not include a raw `Value any` field.
- `ErrUnsupportedCapability` is reserved for #49/#50/#51 and no #48 constructor
  returns it.
- Repo verification uses serial Makefile gates to respect Testcontainers-backed
  packages.
- README pairs must include beginner sections and be checked for content parity.
- Exported graph identifiers require English Go doc comments suitable for
  `pkg.go.dev`.
- Stress tests are N/A for #48 model-only values, with race validation retained.

## Rerun Evidence

| Perspective | P0 | P1 | Evidence |
| --- | ---: | ---: | --- |
| Stability | 0 | 0 | Rerun accepted scalar JSON validation, zero-value no-panic tests, and secret-bearing redaction tests. |
| Operator/Ops | 0 | 0 | Rerun accepted serial `make test`/`make race` gates, PR evidence, and release-support notes. |
| Developer/API | 0 | 0 | Rerun accepted removal of adjacent endpoint helper, fixed JSON shape, reserved unsupported-capability contract, and Go doc requirements. |
| User/Caller | 0 | 0 | Rerun accepted redaction proof, `MustElementID` guidance, README beginner sections, and unsupported capability routing. |

## Follow-Up Gates

- Step 4 must implement tests before code and prove the expected red failures.
- Step 6-R must verify the actual implementation still has P0=0 and P1=0.
- PR evidence must include the Step DoD table, verification outputs or blockers,
  and GitHub check status.
