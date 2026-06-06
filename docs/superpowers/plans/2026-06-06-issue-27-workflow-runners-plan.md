# Issue 27 Workflow Runners Plan

Issue: #27
Milestone: 0.4.0
Spec: `docs/superpowers/specs/2026-06-06-issue-27-workflow-runners-spec.md`
Spec review: `docs/superpowers/reviews/2026-06-06-issue-27-workflow-runners-spec-review.md`
Parent plan: `docs/superpowers/plans/2026-06-05-issue-135-0.4.0-state-workflow-plan.md`

## Execution Boundary

Implement only the first-party `workflow` package and directly related docs,
release notes, lessons, tests, reviews, and PR evidence. Do not implement retry,
repeat, scheduler, durable workflow, mutable `WorkContext`, any-success
parallel semantics, or diagram work.

Apply `$bluetape-go-patterns` to every Go API, test, example, README, and
review task.

## Task Plan

| Task | Complexity | Expected files | Actions | Verification |
|---|---|---|---|---|
| T0 - Pre-implementation risk check | medium | `docs/superpowers/reviews/2026-06-06-issue-27-workflow-runners-preimplementation-risk.md` | Record risks before code: cancellation propagation, sibling cancellation, invalid policies, nil work/predicate, order preservation, and mutable context creep. | Risk artifact exists before implementation. |
| T1 - Package scaffold and core API | medium | `workflow/doc.go`, `workflow/workflow.go`, `workflow/errors.go` | Define `Work`, `Runner`, `Predicate`, constructors, unexported runner structs, nil context normalization, and checkable errors. | `go test -count=1 ./workflow` compiles once tests exist. |
| T2 - Sequential runner | high | `workflow/workflow.go`, `workflow/sequential_test.go` | Run works in order; honor stop/continue policy; stop immediately on aborted/cancelled children; preserve child order through `workreport.Aggregate`. | Unit tests for stop, continue, aborted, cancelled, nil work, and unknown policy. |
| T3 - Conditional runner | medium | `workflow/workflow.go`, `workflow/conditional_test.go` | Evaluate predicate once; run exactly one branch; completed no-op on false with no false branch; predicate error and nil predicate handling. | Branch-count tests, predicate error tests, cancellation-shaped tests. |
| T4 - Parallel runner | high | `workflow/workflow.go`, `workflow/parallel_test.go` | Run all branches with derived cancellable context; cancel siblings for stop policy and terminal abort/cancel; aggregate ordered child reports; wait for all goroutines. | Unit tests for ordering, aggregation, cancellation propagation, sibling cancel, and no-leak shape. |
| T5 - Stress/cancellation coverage | high | `workflow/workflow_concurrency_test.go` | Use `GoroutineStressTester` for repeated parallel fan-out and `AsyncJobTester` for cancellation/deadline propagation. | `go test -race -count=1 ./workflow ./workreport`. |
| T6 - Examples and package docs | medium | `workflow/workflow_example_test.go`, `workflow/README.md`, `workflow/README.ko.md`, `workflow/doc.go` | Add compile-checked examples for sequential, conditional, and parallel runners. Document failure policy, cancellation, and no mutable context. | `go test -count=1 ./workflow -run Example`; README grep for key runner names. |
| T7 - Release/workflow notes and lesson | low | `CHANGELOG.md`, `WIP.md`, `docs/lessons/2026-06-06-workflow-runners.md` | Update 0.4.0 notes and record the runner-context lesson. Do not take over #132 root README indexing or #133 diagrams. | `rg -n "workflow|#27|0.4.0" CHANGELOG.md WIP.md docs/lessons/2026-06-06-workflow-runners.md`. |
| T8 - Formatting and local validation | medium | changed files | Run `gofmt`, targeted tests, race tests, full tests, grep checks, and `git diff --check`. If full `./...` fails from unrelated environment-bound packages, record the exact failure. | `go test -count=1 ./workflow ./workreport`; `go test -race -count=1 ./workflow ./workreport`; `go test -count=1 ./...`; `git diff --check`. |
| T9 - Verifier and Step 6-R code review | high | `docs/superpowers/reviews/2026-06-06-issue-27-workflow-runners-verifier.md`, `docs/superpowers/reviews/2026-06-06-issue-27-workflow-runners-code-review.md` | Verify implementation against issue/spec/plan and run local 7-tier review. P0/P1 findings block PR work and must be fixed and re-reviewed. | Verifier artifact says verified; code review artifact records `P0=0 P1=0`. |
| T10 - Commit, PR, PR review, CI, and DoD | medium | git/GitHub state, PR body | Commit with Lore trailers after validation/review passes. Push branch and create PR with `Fixes #27`; PR body must end with `## DoD Status`. Verify live PR body, run Step 7-R PR review, check CI, and do not merge without user request. | `git status --short`; `gh pr view --json body,state,url`; `gh pr checks`; final DoD table. |

## Acceptance Mapping

| Spec acceptance | Plan coverage |
|---|---|
| `workflow` compiles without new dependencies. | T1, T8 |
| Sequential runner stops or continues according to policy. | T2 |
| Parallel runner propagates cancellation and aggregates child reports. | T4, T5 |
| Conditional runner has branch tests. | T3 |
| Stress/cancellation helpers are used where they add signal. | T5 |
| README pair and compile-checked examples exist. | T6 |
| Local 7-tier review reaches `P0=0 P1=0`. | T9 |

## Ordering And Recheck Points

1. Commit spec, spec review, plan, and plan review before implementation.
2. Run T0 before creating `workflow` source files.
3. Implement shared API and validation before individual runner behavior.
4. Implement sequential and conditional before parallel.
5. Add unit tests before stress tests so stress runs assert stable semantics.
6. Run targeted tests before full `./...` validation.
7. Run Step 6-R only after tests, examples, docs, release notes, and lessons are present.

## Risk Controls

| Risk | Control |
|---|---|
| Parallel runner leaks goroutines | Use derived context cancellation, collect every started goroutine with `sync.WaitGroup`, and run stress/race tests. |
| Child report order becomes nondeterministic | Preallocate result slots by input index and copy only after goroutines finish. |
| Invalid policy silently succeeds | Convert `workreport.Aggregate` policy errors into failed reports and test `errors.Is`. |
| Nil work panics | Validate each work before call and return failed reports. |
| Mutable context map becomes attractive | Keep examples closure-based and document the non-goal. |

## Validation Commands

```bash
gofmt -w workflow
go test -count=1 ./workflow ./workreport
go test -race -count=1 ./workflow ./workreport
go test -count=1 ./...
go test -count=1 ./workflow -run Example
rg -n "^func Example|GoroutineStressTester|AsyncJobTester" workflow
rg -n "workflow|#27|0.4.0" CHANGELOG.md WIP.md docs/lessons/2026-06-06-workflow-runners.md
git diff --check
```

## Step 3 Checklist Completion Report

| Item | Status | Notes |
|------|--------|-------|
| Plan path confirmed inside feature worktree | Done | `docs/superpowers/plans/2026-06-06-issue-27-workflow-runners-plan.md`. |
| All tasks have complexity labels | Done | T0-T10 include complexity. |
| `$bluetape-go-patterns` applied to code-bearing tasks | Done | Execution boundary and T1-T9 require Go API/test/docs/review checks. |
| Tests and verification tasks included | Done | T2-T5 and T8-T9. |
| Stress/cancellation helpers assigned when applicable | Done | T5 uses `GoroutineStressTester` and `AsyncJobTester`. |
| Multilingual README and English contributor docs assigned | Done | T6 README pair; T7 CHANGELOG/WIP/lesson. |
| AGENTS.md impact considered | Done | No guidance or module-list change is introduced. |
| Risky ordering/dependency assumptions explicit | Done | Ordering and risk controls recorded. |
| Spec + plan committed before implementation | Pending | Commit after Step 3-R passes. |
