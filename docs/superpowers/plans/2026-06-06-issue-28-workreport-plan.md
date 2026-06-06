# Issue 28 Workreport Plan

Issue: #28
Milestone: 0.4.0
Spec: `docs/superpowers/specs/2026-06-06-issue-28-workreport-spec.md`
Spec review: `docs/superpowers/reviews/2026-06-06-issue-28-workreport-spec-review.md`
Parent plan: `docs/superpowers/plans/2026-06-05-issue-135-0.4.0-state-workflow-plan.md`

## Execution Boundary

Implement only the first-party `workreport` package. Do not implement workflow
runners, mutable work contexts, retry/repeat/scheduler features, observer
runtimes, or dependency additions.

Apply `$bluetape-go-patterns` to every Go API, test, example, README, and review
task. Keep the API small, explicit, value-based, and `errors.Is` compatible.

## Task Plan

| Task | Complexity | Expected files | Actions | Verification |
|---|---|---|---|---|
| T0 - Pre-implementation risk check | medium | `docs/superpowers/reviews/2026-06-06-issue-28-workreport-preimplementation-risk.md` | Record the main risks before code: aggregation semantics, unknown policy error contract, zero-value handling, child order preservation, and future #27 compatibility. | Risk artifact exists before implementation. |
| T1 - Package scaffold and exported types | medium | `workreport/doc.go`, `workreport/status.go`, `workreport/policy.go`, `workreport/report.go`, `workreport/errors.go` | Define `Status`, `FailurePolicy`, `Report`, `ErrUnknownFailurePolicy`, comments, and zero-value behavior. Keep fields ordinary Go values. | `go test -count=1 ./workreport` compiles once tests exist. |
| T2 - Constructors and predicates | medium | `workreport/report.go`, `workreport/report_test.go` | Add `Completed`, `Failed`, `Partial`, `Aborted`, `Cancelled`, status predicates, `IsSuccess`, `IsFailure`, and `IsTerminal`. Preserve errors and reason fields. | Unit tests for every constructor and predicate, including zero-value report. |
| T3 - Aggregation semantics | high | `workreport/report.go`, `workreport/report_test.go` | Implement `Aggregate(name, policy, children...) (Report, error)` with stop and continue policy behavior, child-order preservation, no-child success, and unknown policy errors. | Unit tests for stop-on-failure truncation, continue-on-failure all-child preservation, no children, all completed, partial/aborted/cancelled children, and `errors.Is`. |
| T4 - Stress and cancellation coverage | high | `workreport/report_concurrency_test.go` | Use `GoroutineStressTester` for repeated immutable child aggregation and `AsyncJobTester` for cancellation report preservation with `context.Canceled`. Keep tests race-compatible and service-free. | `go test -race -count=1 ./workreport`. |
| T5 - Examples and package docs | medium | `workreport/workreport_example_test.go`, `workreport/README.md`, `workreport/README.ko.md`, `workreport/doc.go` | Add compile-checked examples for aggregation and cancellation/failure reporting. Document statuses, policies, zero-value behavior, and #27 workflow boundary. | `go test -count=1 ./workreport -run Example`; README grep for `StopOnFailure`, `ContinueOnFailure`, `zero-value`. |
| T6 - Release/workflow notes and lesson | low | `CHANGELOG.md`, `WIP.md`, `docs/lessons/2026-06-06-workreport-failure-policy.md` | Update 0.4.0 notes and record a short lesson about keeping result models independent from runner execution. Do not update root README package tables unless needed to satisfy this PR; #132 owns root index alignment. | `rg -n "workreport|#28|0.4.0" CHANGELOG.md WIP.md docs/lessons/2026-06-06-workreport-failure-policy.md`. |
| T7 - Formatting and local validation | medium | changed files | Run `gofmt -w workreport`, targeted tests, race tests, relevant example grep, `go test -count=1 ./...`, and `git diff --check`. If full `./...` fails due unrelated Testcontainers or environment packages, record exact package/error and keep targeted evidence primary. | `go test -count=1 ./workreport`; `go test -race -count=1 ./workreport`; `go test -count=1 ./...`; `git diff --check`. |
| T8 - Verifier and Step 6-R code review | high | `docs/superpowers/reviews/2026-06-06-issue-28-workreport-verifier.md`, `docs/superpowers/reviews/2026-06-06-issue-28-workreport-code-review.md` | Read required Step 6-R references. Verify implementation against spec/plan and run local 7-tier Go review. P0/P1 findings block PR work and must be fixed and re-reviewed. | Verifier artifact says verified; code review artifact records `P0=0 P1=0`. |
| T9 - Commit, PR, PR review, CI, and DoD | medium | git/GitHub state, PR body | Commit with Lore trailers after validation/review passes. Push branch and create PR with `Fixes #28`; PR body must end with `## DoD Status`. Verify live body, run Step 7-R PR review, check CI, and do not merge without user request. | `git status --short`; `gh pr view --json body,state,url`; `gh pr checks`; final Step DoD table. |

## Acceptance Mapping

| Spec acceptance | Plan coverage |
|---|---|
| `workreport` compiles without new dependencies. | T1, T7 |
| Statuses, failure policies, reports, constructors, predicates, and aggregation helpers. | T1, T2, T3 |
| `Report` preserves errors and child reports. | T2, T3 |
| Zero-value behavior documented and tested. | T1, T2, T5 |
| Stress/cancellation helpers used where they add signal. | T4, T7 |
| Package README pair and compile-checked examples exist. | T5 |
| Step 6-R review reaches `P0=0 P1=0`. | T8 |

## Ordering And Recheck Points

1. Commit spec, spec review, plan, and plan review before implementation.
2. Run T0 before creating `workreport` source files.
3. Implement types before constructors, constructors before aggregation, and
   aggregation before examples/README snippets.
4. Add unit tests before stress/cancellation tests so concurrency tests assert
   stable semantics.
5. Run targeted `./workreport` tests before full `./...` validation.
6. Run Step 6-R only after tests, examples, docs, and release notes are present.

## Risk Controls

| Risk | Control |
|---|---|
| `Aggregate` policy errors become ambiguous | Export `ErrUnknownFailurePolicy` and test `errors.Is`. |
| Zero-value report is treated as success | Predicate tests require zero value to be unknown, non-success, non-failure, and non-terminal. |
| Child errors are flattened away | Preserve child reports and assert `errors.Is` on child `Err` values. |
| Stop/continue semantics conflict with #27 | Encode stop truncation and continue all-child preservation before workflow consumes the package. |
| Race tests give false confidence | Use immutable inputs and race run; keep workreport stateless. |

## Validation Commands

```bash
gofmt -w workreport
go test -count=1 ./workreport
go test -race -count=1 ./workreport
go test -count=1 ./...
rg -n "^func Example|GoroutineStressTester|AsyncJobTester" workreport
rg -n "workreport|#28|0.4.0" CHANGELOG.md WIP.md docs/lessons/2026-06-06-workreport-failure-policy.md
git diff --check
```

## Step 3 Checklist Completion Report

| Item | Status | Notes |
|------|--------|-------|
| Plan path confirmed inside feature worktree | Done | `docs/superpowers/plans/2026-06-06-issue-28-workreport-plan.md`. |
| All tasks have complexity labels | Done | T0-T9 include complexity. |
| `$bluetape-go-patterns` applied to code-bearing tasks | Done | Execution boundary and T1-T8 require Go API/test/docs/review checks. |
| Tests and verification tasks included | Done | T2-T4, T7, T8. |
| Stress/cancellation helpers assigned when applicable | Done | T4 uses `GoroutineStressTester` and `AsyncJobTester`. |
| Multilingual README and English contributor docs assigned | Done | T5 README pair; T6 CHANGELOG/WIP/lesson. |
| AGENTS.md impact considered | Done | No guidance or module-list change is introduced. |
| Risky ordering/dependency assumptions explicit | Done | Ordering and risk controls recorded. |
| Spec + plan committed before implementation | Pending | Commit after Step 3-R passes. |
