# Issue #175 Step 3-R Plan Review

Issue: #175
Date: 2026-06-14
Plan: `docs/superpowers/plans/2026-06-14-issue-175-jwt-provider-cache-adapters-plan.md`
Spec: `docs/superpowers/specs/2026-06-14-issue-175-jwt-provider-cache-adapters-design.md`
Gate: 7-Tier = 6 independent lanes + main integration review
Wait SLA: subagent wait max 10 minutes; long blocking wait is forbidden.

## Initial 7-Tier Results

| Tier | Perspective | Verdict | P0 | P1 | P2 | P3 |
|---|---|---:|---:|---:|---:|---:|
| 1 | Performance | COMMENT | 0 | 0 | 0 | 1 |
| 2 | Stability | REQUEST_CHANGES | 0 | 2 | 1 | 0 |
| 3 | Security | APPROVE | 0 | 0 | 0 | 0 |
| 4 | Operator/Ops | COMMENT | 0 | 0 | 1 | 1 |
| 5 | Developer/API | COMMENT | 0 | 0 | 1 | 1 |
| 6 | User/Caller | COMMENT | 0 | 0 | 1 | 1 |
| Main | Integration | REQUEST_CHANGES | 0 | 2 | 4 | 4 |

## Plan Hardening Applied

| Review area | Plan update |
|---|---|
| Performance benchmark evidence | Task 8 now requires `-benchmem`, parse counts, key lookup/revalidation counts, warm-hit evidence, same-key cold-burst evidence, and key-builder benchmarks. |
| Stability context semantics | Task 4 now requires local `ParseContext(nil)`, canceled context, expired deadline, `ClearCache(nil/canceled/deadline)`, no cache mutation/delegation on already-done contexts, and blocked same-key singleflight waiter cancellation. |
| Stability race gate | Task 10 now requires `make race`; targeted `go test -race -count=1 ./jwt ./cache ./testing/concurrency` is allowed only as documented fallback for unrelated repository failures. |
| Cache error propagation | Task 6 now requires `errors.Is` preservation for `Set`, stale-entry `Delete`, `ClearCache`, adapter-owned rotation clear failures, and distributed delete/reset clear failures. |
| Operator runbook | Task 9 now requires EN/KO operator runbook coverage for clear scope, multi-instance behavior, diagnostics, unsupported untrusted shared/external cache, failure response, and no raw-token logging. |
| Developer examples | Task 9 now requires compile-checked examples for local and distributed cached-provider construction. |
| User documentation parity | Task 9 now requires explicit EN/KO README parity evidence for selection guide, imports, examples, trust boundary, custom clock bypass, clear scope, diagnostics, and non-auth-framework caveats. |

## Affected-Lane Rerun

| Tier | Rerun Scope | Result | P0 | P1 | Notes |
|---|---|---:|---:|---:|---|
| 2 | Stability blocking fixes | TIMEOUT | N/A | N/A | `lane timed out; main integration fallback performed`. Agent `019ec1dd-4a9f-7ec0-bd7f-72435cb2ddb4` was closed after the SLA check. |

## Main Integration Fallback

The stability rerun did not return before the enforced wait limit. The main
integration review performed a read-only fallback against the current plan and
verified that the original stability P1 findings are now covered by explicit
implementation tasks and verification commands:

- Local and distributed context rejection paths are required before cache
  access or provider delegation.
- Blocked same-key singleflight waiter cancellation is required.
- Cache failure tests must preserve `errors.Is` for non-miss failures and must
  make stale delete and clear failures caller-visible.
- `GoroutineStressTester`, cold-burst tests, targeted race tests, and
  repository-level `make race` are required.
- Rotation, distributed delete/reset, stale-key invalidation, and failure
  non-caching are included in the stress matrix.

No remaining P0/P1 issue is visible in the plan after fallback review.

## Main Integration Verdict

APPROVE.

Final gate:

- P0 = 0
- P1 = 0

The plan is ready for Step 4 implementation. P2/P3 comments were converted into
explicit tasks where they reduce implementation risk, documentation drift, or
verification ambiguity.

## Verification Evidence

- `git diff --check`
  - PASS
- `go test -count=1 ./jwt ./cache ./testing/concurrency`
  - PASS
- Plan content checks:
  - `ParseContext(nil)` coverage present
  - same-key `singleflight` waiter cancellation coverage present
  - `make race` final gate present
  - `errors.Is` failure propagation coverage present
  - compile-checked examples present
  - README parity evidence requirement present

## DoD

| Item | Status |
|---|---|
| Six independent lanes run | Done |
| Main integration review run | Done |
| Completed agents closed | Done |
| Timeout fallback recorded if needed | Done |
| P0/P1 normalized | Done |
| Blocking findings addressed in plan | Done |
| Convergence P0=0 P1=0 | Done |
