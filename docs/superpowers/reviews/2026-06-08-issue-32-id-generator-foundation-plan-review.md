# Issue 32 ID Generator Foundation Plan Review

Plan: `docs/superpowers/plans/2026-06-08-issue-32-id-generator-foundation-plan.md`
Spec: `docs/superpowers/specs/2026-06-08-issue-32-id-generator-foundation-spec.md`
Review gate: Step 3-R
Baseline: `origin/develop` at `fc2ec24`
Method: subagent-based 7-Tier plan review

## Lane Budget

- Lanes: 7 independent read-only subagents.
- Write scope: none for reviewers.
- Forbidden work: implementation, test/build mutation, PR/issue mutation.
- Stop condition: each tier returns P0/P1/P2/P3 findings with file:line
  evidence and explicit `P0=<n> P1=<n>`.

## Initial Subagent Results

| Tier | Reviewer | P0 | P1 | P2 | P3 | Summary |
|---|---|---:|---:|---:|---:|---|
| 1 Security | subagent | 0 | 1 | 2 | 0 | Entropy defaults, failing-reader tests, Snowflake machine ID auto-discovery, and auth/secret docs needed stronger plan requirements. |
| 2 Ops/SRE reliability | subagent | 0 | 1 | 1 | 0 | Snowflake machine ID uniqueness scope and UUID v7/ULID wall-clock rollback behavior were under-specified. |
| 3 Structural impact | subagent | 0 | 0 | 2 | 0 | T0 dependency risk artifact was too narrow; T5 depended on later T10 verifier evidence for `AsyncJobTester` N/A. |
| 4 Go API quality | subagent | 0 | 1 | 0 | 0 | Common error-contract tests were incorrectly referenced from T5 and did not cover UUID/ULID/common wrapping thoroughly enough. |
| 5 Tests/types/silent failure | subagent | 0 | 1 | 2 | 0 | UUID/ULID failing entropy tests, Snowflake invalid parse/decode cases, and exact cancellation N/A branch evidence were missing. |
| 6 Performance/stability | subagent | 0 | 0 | 1 | 0 | Stateful hot paths needed `b.RunParallel` benchmark coverage. |
| 7 Docs/release/evidence | subagent | 0 | 0 | 3 | 0 | `make ci`, PR-vs-issue metadata comparison, and `KSUID (#166)` deferred verification were missing. |

Initial blocker result: `P0=0 P1=4`.

## Plan Repairs Applied

| Finding | Repair |
|---|---|
| Tier 1 P1: entropy defaults and failing-reader tests under-specified | T0 now requires `crypto/rand` or dependency-proven crypto entropy defaults; T2/T3 require test-only deterministic readers, failing-reader `%w` tests, and no zero ID with nil error. |
| Tier 1/Tier 2: Snowflake machine ID trust boundary and duplicate-ID risk | T4/T7/Risk Controls now require caller-provided unique machine IDs per live generator/process/deployment, no allocator, no MAC/env/host auto-discovery, and duplicate-machine-ID collision docs. |
| Tier 1 docs caveat gap | T7 and validation grep now require authentication, authorization, secret, and standalone security boundary wording. |
| Tier 2 P2: UUID v7/ULID clock rollback behavior | T2/T3/T7/Risk Controls now require rollback no-hang behavior, ordering caveats, and uniqueness not depending on monotonic clocks. |
| Tier 3 P2: T0 dependency risk artifact too narrow | T0 now records ULID maintenance signal, parse/format API shape, monotonic entropy behavior, deterministic testability hooks, public type exposure, and fallback rationale. |
| Tier 3 P2/Tier 5 P2: `AsyncJobTester` N/A evidence depended on later verifier | T5 now owns its concurrency notes artifact and exact branch A/B verification text. |
| Tier 4 P1: common error-contract tests misplaced | T1 now includes `id/errors_test.go` and `errors.Is`/`errors.As` tests for invalid options, invalid encoded IDs, unsupported algorithm/version, and zero-value behavior. T2/T3/T4 add parse/randomness/rollback/exhaustion wrapping checks. |
| Tier 5 P2: Snowflake invalid parse/decode gaps | T4 now requires invalid parse/decode tests for negative/non-63-bit IDs and malformed string/base36 input if string rendering is implemented. |
| Tier 6 P2: no parallel benchmark coverage | T6 now requires `b.RunParallel` benchmarks for Snowflake `NextInt64` and monotonic ULID generation. |
| Tier 7 P2: local CI and PR metadata evidence gaps | T9 now requires `make ci`; T11 now requires `gh issue view 32` plus `gh pr view` metadata comparison in DoD/PR review evidence. |
| Tier 7 P2: deferred #166 not verified in durable docs | T7/T11 and validation grep now require `KSUID (#166)` deferred notes. |

Post-repair document checks:

- `git diff --check`
- `rg -n "crypto/rand|failing-reader|entropy|errors.Is|errors.As|unique.*machine ID|duplicate.*machine ID|wall-clock rollback|AsyncJobTester N/A|b.RunParallel|make ci|gh issue view 32|KSUID.*#166" docs/superpowers/plans/2026-06-08-issue-32-id-generator-foundation-plan.md`

## Affected-Tier Re-Review

| Tier | Reviewer | P0 | P1 | P2 | P3 | Evidence |
|---|---|---:|---:|---:|---:|---|
| 1 Security | subagent | 0 | 0 | 0 | 0 | Plan lines 46, 48-50, 53, 97-101, and 115 cover crypto entropy, test-only deterministic readers, failing-reader tests, no zero-ID nil success, Snowflake machine ID trust boundaries, and auth/secret docs. |
| 2 Ops/SRE reliability | subagent | 0 | 0 | 0 | 0 | Plan lines 48-51, 53, 96-100, and 110 cover Snowflake uniqueness scope, duplicate-machine-ID risk, no allocator, no host/MAC/env auto-discovery, UUID v7/ULID rollback behavior, and race validation. |
| 3 Structural impact | subagent | 0 | 0 | 0 | 0 | Plan lines 46, 51, 72-76, and 7-22 keep T0 before implementation, put `AsyncJobTester` N/A in T5, and preserve #166/deferred scope. |
| 4 Go API quality | subagent | 0 | 0 | 0 | 0 | Plan lines 47-51 place common error tests in T1 and map dependency parse/randomness wrapping plus Snowflake rollback/exhaustion to family tasks. |
| 5 Tests/types/silent failure | subagent | 0 | 0 | 0 | 0 | Plan lines 48-53, 110-115 cover deterministic/failing readers, no zero nil IDs, Snowflake invalid parse/decode, exact cancellation branch evidence, race/stress, examples, and benchmarks. |
| 6 Performance/stability | subagent | 0 | 0 | 0 | 0 | Plan lines 50-52, 55, 96, and 111-113 cover bounded/exhaustion behavior, parallel hot-path benchmarks, race, benchmark smoke, and `make ci`. |
| 7 Docs/release/evidence | subagent | 0 | 0 | 0 | 0 | Plan lines 53-57, 83-86, 102-103, 113-116, and 130 cover README pairs, CHANGELOG/WIP, `make ci`, PR body DoD, PR-vs-issue metadata comparison, and `KSUID (#166)`. |

## Evidence-Integrity Repair

Tier 7 found two P3 stale line references in the Step 2-R spec review artifact.
The references were corrected:

- Structural deferral evidence now cites spec lines 4-6, 18-26, 71-73,
  280-288, 342, and 353-354.
- Performance benchmark evidence now cites spec lines 324-326 and 340.

Narrow Tier 7 recheck returned `P0=0 P1=0 P2=0 P3=0`.

## Integrated Verdict

PASS. Subagent-based Step 3-R convergence reached `P0=0 P1=0`.

All P0/P1 findings were fixed in the plan and affected tiers were rerun to clean
results. P2/P3 findings were also repaired rather than deferred.

## Step 3-R Checklist Completion Report

| Item | Status | Notes |
|------|--------|-------|
| Step 3-R references read | Done | `step-3r-plan-review-perspectives.md` and `step-3r-plan-review.md`. |
| Plan reviewed against spec | Done | 7-Tier subagent review used the plan, spec, spec review, and `$bluetape-go-patterns`. |
| P0/P1 findings normalized | Done | Initial `P0=0 P1=4`; final `P0=0 P1=0`. |
| Required plan edits applied | Done | Entropy, errors, Snowflake machine ID, rollback, cancellation N/A, benchmarks, CI, metadata, and #166 docs. |
| Affected lanes rerun | Done | All seven tiers reran or narrowly rechecked to clean results. |
| Next step unblocked | Done | Spec, spec review, plan, and plan review can be committed before implementation. |
