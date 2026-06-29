# Issue #48 Graph Abstraction Step 2-R Spec Review

Date: 2026-06-29
Scope: `docs/superpowers/specs/2026-06-29-issue-48-graph-abstraction-design.md`

## Verdict

P0: 0
P1: 0

Step 2-R required two convergence iterations. Iteration 1 found blocking API
and stability issues around exported mutable fields, zero-value contracts,
`any` ID conversion, path-step invariants, and stress-test rationale. The spec
was revised to use unexported value fields, defensive accessors, explicit
`Validate` methods, typed edge endpoint structs, validating path-step helpers,
redacted errors, JSON validation, and a package-level race gate.

## Evidence

- Live issue: #48 `Design minimal Go graph abstraction`, milestone `0.10.0`,
  priority `p0`.
- Prior research: #38 and PR #307 narrowed graph work to model-first API and
  rejected Kotlin repository/session DSL porting.
- Source parity checked against `bluetape4k-graph/graph/graph-core` model and
  repository files.
- Worktree baseline: `go test ./...` passed before edits.
- Step 2-R reference:
  `/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-2r-spec-review.md`.
- Go guidance: `/Users/debop/.codex/skills/bluetape-go-patterns/SKILL.md`.

## Iteration 1 Findings

| Perspective | P0 | P1 | P2 | P3 | Resolution |
| --- | ---: | ---: | ---: | ---: | --- |
| Performance | 0 | 1 | 1 | 1 | Replaced false immutability/stress rationale with shallow defensive-copy contract and `go test -race -count=1 ./graph` gate. |
| Stability | 0 | 2 | 2 | 0 | Added zero-value behavior, unexported fields, accessors, validation, invalid weight checks, and race verification. |
| Security | 0 | 0 | 3 | 1 | Added validating JSON contract, redacted validation errors, explicit property ownership, and unsupported raw ID rejection. |
| Operator/Ops | 0 | 0 | 2 | 0 | Added `CHANGELOG.md`, `WIP.md`, `make lint`, and `make ci` requirements. |
| Developer/API | 0 | 1 | 2 | 1 | Removed exported mutable fields, narrowed constructors, specified weighted path constructor. |
| User/Caller | 0 | 2 | 1 | 0 | Added named edge endpoints, validating path-step constructors, raw-driver migration example requirement, and unsupported capability docs. |

## Iteration 2 Findings

| Perspective | P0 | P1 | P2 | P3 | Evidence |
| --- | ---: | ---: | ---: | ---: | --- |
| Performance | 0 | 0 | 0 | 1 | Rerun accepted shallow defensive-copy wording and race gate. |
| Stability | 0 | 0 | 0 | 0 | Rerun accepted zero-value, copy, path, weight, and stress/race contracts. |
| Security | 0 | 0 | 0 | 0 | Rerun accepted JSON validation, redaction, property ownership, and raw ID boundaries. |
| Developer/API + User/Caller | 0 | 0 | 0 | 0 | Second rerun accepted `EdgeEndpoints`/`RawEdgeEndpoints`, validating `VertexStep`/`EdgeStep`, real import-path migration example, and unsupported capability routing. |

## Integrated Review

| Priority | Area | Finding | Status |
| --- | --- | --- | --- |
| P1 | API stability | Public mutable map/slice/pointer fields contradicted defensive model goals. | Resolved by unexported fields and accessors. |
| P1 | User/caller safety | Adjacent same-type edge endpoints allowed silent direction reversal. | Resolved by `EdgeEndpoints` and `RawEdgeEndpoints`. |
| P1 | Stability | Zero values and invalid path steps were under-specified. | Resolved by explicit zero-value contract and validating step constructors. |
| P1 | Test evidence | Stress exemption claimed immutability while mutable state remained exposed. | Resolved by shallow defensive-copy wording and race gate. |

## Required Follow-Up In Plan

- Implement TDD tasks for validation, defensive copies, JSON validation,
  redacted errors, edge endpoint roles, path-step validation, and weighted path
  validation.
- Add package and root README locale updates.
- Add `CHANGELOG.md` and `WIP.md` release bookkeeping.
- Verify with `go test ./graph`, `go test -race -count=1 ./graph`,
  `go test ./...`, `make fmt-check`, `make tidy-check`, `make vet`,
  `make lint`, and `make ci` or record exact blockers.
