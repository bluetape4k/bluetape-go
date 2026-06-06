# Issue 26 State Package Plan

Spec: `docs/superpowers/specs/2026-06-05-issue-26-state-spec.md`
Spec review: `docs/superpowers/reviews/2026-06-05-issue-26-state-spec-review.md`
Research: `docs/superpowers/research/2026-06-05-issue-26-state-inventory.md`
Issue: #26
Milestone: 0.4.0

## Execution Boundary

Implement only the first-party `state` package for finite state machine
primitives. Do not implement `workflow`, `workreport`, Kotlin-style DSLs,
callbacks, observers, reactive/event-effect layers, or dependency additions.

The Step 2-R spec review passed with `P0=0 P1=0`; this plan is authored from
that reviewed spec. Commit the research, spec, spec review, and reviewed plan
before Step 4 implementation.

## Task Plan

| Task | Complexity | Expected files | Actions | Verification |
|---|---|---|---|---|
| T0 - Pre-implementation prediction | high | `docs/superpowers/reviews/2026-06-05-issue-26-state-preimplementation-risk.md` | Because the package has high-complexity concurrency behavior, run Step 3-P before implementation. Record top risks for guard execution, state recheck, context cancellation, and error inspection, then feed them into T1-T4. | Risk artifact exists before Step 4 implementation. |
| T1 - Package API and errors | high | `state/doc.go`, `state/types.go`, `state/errors.go`, `state/machine.go` | Define `Guard`, `Transition`, `Result`, `Machine`, `Option`, `WithFinalStates`, sentinel errors, and `TransitionError`. Implement `TransitionError.Is` for sentinel matching and `Unwrap` for guard/context cause inspection. Keep comments in Go doc style and package independent from `workflow`/`workreport`. | `go test -count=1 ./state` compiles once tests exist. |
| T2 - Machine construction and registry | high | `state/machine.go` | Build transition map keyed by `(from,event)`, preserve allowed event registration order, validate duplicate transitions, validate initial state membership, and normalize nil context at public entry points. | Unit tests for duplicate transitions, unknown initial state, and allowed event order. |
| T3 - Transition behavior | high | `state/machine.go`, `state/state_test.go` | Implement `State`, `Transition`, `CanTransition`, and `AllowedEvents`. Run guards outside the lock, check context before lookup and before commit, re-check current state before commit, and return `ErrConcurrentTransition` for concurrent losers. | Unit tests for valid transition, invalid transition, guard success/rejection, final state, `CanTransition`, `AllowedEvents`, context cancellation, and `TransitionError` `errors.Is`. |
| T4 - Stress and cancellation coverage | high | `state/state_concurrency_test.go` | Use `GoroutineStressTester` to prove concurrent guarded transitions commit exactly once and return deterministic errors for the rest. Use `AsyncJobTester` to prove guard cancellation is propagated. Keep Testcontainers out of scope. | `go test -race -count=1 ./state`. |
| T5 - User-facing docs and examples | medium | `state/README.md`, `state/state_example_test.go`, `state/doc.go` | Add compile-checked examples and package README. Document that `CanTransition` may execute guard code, guards used for inquiry should be side-effect safe, and `AllowedEvents` is a structural registry query that does not evaluate guards. | `go test -count=1 ./state -run Example`; README grep for `CanTransition`, `AllowedEvents`, and `errors.Is`. |
| T6 - Release/workflow docs | low | `CHANGELOG.md`, `WIP.md`, `docs/lessons/2026-06-05-state-machine-primitives.md` | Update Unreleased/0.4.0 notes and current work status. Add a short lesson for the Go FSM guard/inspection contract. Do not update root README links; #132 owns that follow-up. | `rg -n "state|#26|0.4.0" CHANGELOG.md WIP.md docs/lessons/2026-06-05-state-machine-primitives.md`. |
| T7 - Formatting, cleanup, and local validation | medium | changed files | Run `gofmt -w state`, then targeted test, race test, full test, and whitespace check. Make Step 4-S cleanup run/skip decision after implementation. If generated code exceeds 200 lines or the concurrency path shows avoidable complexity, run a focused cleanup pass and rerun tests. If full `go test ./...` fails in unrelated Testcontainers packages, record the exact package/error and keep targeted state evidence primary. | `go test -count=1 ./state`; `go test -race -count=1 ./state`; `go test -count=1 ./...`; `git diff --check`; cleanup run/skip decision recorded. |
| T8 - Step 4-P/5/6/6-R verification | high | `docs/superpowers/reviews/2026-06-05-issue-26-state-verifier.md`, `docs/superpowers/reviews/2026-06-05-issue-26-state-code-review.md` | Read `references/step-4p-perf-scan.md` and run/record the Step 4-P performance/stability scan because this is a concurrency package. Read `references/step-5-verifier-checklist.md`, then verify implementation against spec and plan. Read required Step 6-R references before local 7-Tier code review on the diff. P0/P1 findings block PR work and must be fixed and rerun. | Step 4-P run/skip evidence; verifier verdict `VERIFIED`; Step 6-R artifact records `P0=0 P1=0`. |
| T9 - Commit, PR, CI, and issue update | medium | git/GitHub state | Commit with Lore trailers after validation and review gates pass. Push branch, create PR with final section `## DoD Status`, verify PR body is non-empty, run Step 7-R PR review, then check CI. Do not merge without user request. | `git status --short`; `gh pr view --json body,state,url`; `gh pr checks`; PR review artifact/comment. |

## Acceptance Mapping

| Spec acceptance | Plan coverage |
|---|---|
| Define states, events, transitions, guards, and transition errors. | T1, T2, T3 |
| Valid transition, invalid transition, and guarded transition tests. | T3 |
| Framework-free API. | T1, T2, T9 review |
| Concurrent transition stress coverage. | T4, T7 |
| Guard cancellation coverage. | T3, T4, T7 |
| Compile-checked examples and README. | T5 |
| CHANGELOG/WIP/lesson updated. | T6 |
| Local 7-Tier review with P0/P1=0. | T8 |

## Ordering And Recheck Points

1. Commit research, spec, spec review, and plan after Step 3-R passes and before
   implementation.
2. Run T0 Step 3-P before creating implementation files.
3. Implement API/errors before machine logic so tests can target stable names.
4. Add unit tests before stress tests; stress tests should assert deterministic
   outcome categories, not scheduler timing.
5. Add docs/examples after behavior stabilizes so examples compile against the
   final API.
6. Run targeted `./state` validation before full `./...` validation.
7. Run Step 4-P and Step 6-R only after tests pass and all planned docs are
   present.

## Risk Controls

| Risk | Control |
|---|---|
| Guard deadlock | Never call guard while holding the internal lock; add a guard test that calls `State`. |
| Guard side effects during `CanTransition` | Document inquiry-safe guard expectation and include README/example wording. |
| Ambiguous `AllowedEvents` semantics | Document and test structural registry semantics without guard evaluation. |
| Race-prone transition commit | Re-check state after guard and before mutation; race-test `./state`. |
| Error inspection loses guard/context cause | Test `errors.Is` against both package sentinel and wrapped guard/context errors. |
| Full repo tests fail outside #26 scope | Record exact failure and keep targeted `./state` plus race evidence; do not hide unrelated failures. |

## Validation Commands

```bash
gofmt -w state
go test -count=1 ./state
go test -race -count=1 ./state
go test -count=1 ./...
git diff --check
```

## Step 3 Checklist Completion Report

| Item | Status | Notes |
|------|--------|-------|
| Plan path confirmed inside feature worktree | Done | `docs/superpowers/plans/2026-06-05-issue-26-state-plan.md` in `.worktrees/issue-26-state`. |
| All tasks have complexity labels | Done | T0-T9 include complexity. |
| Code-bearing tasks include applicable code pattern expectations | Done | Go package tasks require Go doc, first-party API, context-first behavior, `errors.Is`, and repo test helpers; Kotlin-specific `$bluetape4k-code-patterns` is N/A for Go implementation. |
| Thread/cancellation safety tests use existing helpers when applicable | Done | T4 requires `GoroutineStressTester` and `AsyncJobTester`. |
| Tests and verification tasks included | Done | T3, T4, T7, T8. |
| Multilingual README, English contributor-doc, and AGENTS.md tasks included when scope requires | Done | Package `README.md`, CHANGELOG, WIP, and lesson included; root README deferred to #132 by spec. AGENTS.md not touched because no durable guidance or module-list rule changes are introduced. |
| Risky ordering/dependency assumptions are explicit | Done | Ordering and recheck points recorded. |
| Spec + plan committed to feature branch before implementation | Done | This plan is included in the required pre-implementation commit before Step 4. |
