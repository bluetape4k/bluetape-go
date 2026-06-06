# Issue 27 Workflow Runners Spec Review

Spec: `docs/superpowers/specs/2026-06-06-issue-27-workflow-runners-spec.md`
Issue: #27
Gate: Step 2-R
Status: PASS

## Scope

Reviewed the #27 issue-local spec against the #27 issue body, #135 parent
spec/plan, #28 `workreport` implementation, existing concurrency helpers, and
`bluetape-go-patterns`.

## Findings

| Severity | Finding | Resolution |
|---|---|---|
| P2 | Parallel early-stop behavior could be ambiguous if `StopOnFailure` only described aggregation and not sibling cancellation. | Spec now states that `StopOnFailure`, aborted, and cancelled child reports cancel sibling work and wait for all started goroutines. |
| P2 | Conditional false branch arity could grow into a mini pipeline. | Spec limits the false branch to at most one work item and keeps multiple steps in `Sequential`. |

## Perspective Review

| Perspective | P0 | P1 | P2 | P3 | Evidence |
|---|---:|---:|---:|---:|---|
| Developer / Go implementer | 0 | 0 | 0 | 0 | API is `Work`, `Runner`, and three constructor functions; concrete types can stay unexported. |
| Test engineer | 0 | 0 | 0 | 0 | Sequential, conditional, parallel, cancellation, stress, and race coverage are explicit. |
| Architect | 0 | 0 | 0 | 0 | `workflow` imports `workreport`; no import cycle or mutable context framework is introduced. |
| Library user | 0 | 0 | 0 | 0 | Error and cancellation contracts are caller-checkable and context-driven. |

## Local 7-Tier Review

| Tier | P0 | P1 | P2 | P3 | Evidence |
|---|---:|---:|---:|---:|---|
| 1 Security | 0 | 0 | 0 | 0 | No auth, secrets, deserialization, or trust boundary. |
| 2 Ops/SRE reliability | 0 | 0 | 0 | 0 | Goroutine lifecycle, cancellation, and no-leak tests are required. |
| 3 Structural impact | 0 | 0 | 0 | 0 | New package follows the #135 split and consumes #28 only. |
| 4 Go API quality | 0 | 0 | 0 | 0 | Small first-party API; ordinary closures instead of DSL or mutable maps. |
| 5 Tests/types/silent failure | 0 | 0 | 0 | 0 | Nil work, nil predicate, predicate error, unknown policy, and context cancellation are specified. |
| 6 Performance/stability | 0 | 0 | 0 | 0 | Parallel runner must wait for started goroutines and preserve order without shared slice races. |
| 7 Docs/release/evidence | 0 | 0 | 0 | 0 | `doc.go`, README pair, examples, and Step 6-R are required. |

## Gate Verdict

P0=0 P1=0. Step 2-R is closed.
