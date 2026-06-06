# Issue #26 Pre-Implementation Risk Review

Issue: #26
Milestone: 0.4.0
Gate: Step 3-P
Date: 2026-06-05
Spec: `docs/superpowers/specs/2026-06-05-issue-26-state-spec.md`
Plan: `docs/superpowers/plans/2026-06-05-issue-26-state-plan.md`

Reference loaded:
`/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-4p-perf-scan.md`.

## Trigger

Step 3-P is required because #26 adds high-complexity concurrency behavior:

- mutable current state protected by synchronization.
- caller-supplied guard execution.
- context cancellation before and during transitions.
- deterministic behavior under concurrent transition attempts.

## Risk Register

| Risk | Priority | Required implementation control | Required verification |
|---|---|---|---|
| Guard deadlock if a guard calls back into the machine while a lock is held. | P1 | Copy current state and target under lock, release lock, run guard, then re-lock to commit. | Unit test with a guard that calls `State`; race test. |
| State changes while a slow guard runs. | P1 | Re-check current state after guard and before mutation; return `ErrConcurrentTransition` for losers. | `GoroutineStressTester` test asserting exactly one success and deterministic loser errors. |
| Context cancellation after guard but before commit could mutate state after caller cancellation. | P1 | Check `ctx.Err()` before lookup and again before commit. | Cancellation test where guard cancels/unblocks before commit and state remains unchanged when context is canceled. |
| `CanTransition` can execute side-effecting guard code. | P2 | Keep the API contract but document inquiry-safe guard expectation in Go doc and README. | Unit/doc test showing `CanTransition` does not mutate machine state; README grep. |
| `AllowedEvents` can be mistaken for guard-approved events. | P2 | Implement it as registered-event lookup only and avoid guard evaluation. | Test with rejecting guard where event still appears in `AllowedEvents` but `CanTransition` returns false/error. |
| `TransitionError` can lose either sentinel or guard/context cause. | P1 | Store sentinel `Kind`, optional `Cause`, implement `Is` for `Kind`, `Unwrap` for `Cause`. | `errors.Is` tests for `ErrGuardRejected`, `ErrInvalidTransition`, `ErrFinalState`, `ErrConcurrentTransition`, guard error, and context error. |

## Step 3-P Checklist Completion Report

| Item | Status | Notes |
|------|--------|-------|
| 3-P ran or skip reason recorded | Done | Required by high-complexity concurrency behavior. |
| Top risks/mitigations added to Step 4 task context | Done | Risk controls map directly to T1-T4 and T8 in the plan. |
