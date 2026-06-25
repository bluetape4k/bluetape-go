# Issue #213 Cancellation Assertion Helpers Spec

Issue: #213
Milestone: 0.6.4
Worktree: `issue-213-cancellation-assertions`
Date: 2026-06-22

## Classification

Type B Fast Track. This extends the existing `testing` package with focused
assertion helpers and examples. It does not add a new module or dependency.

## Evidence

- Baseline `go test -count=1 ./testing/...` passed at `c4fdb41`.
- GNO GitHub search found #213, related #211, prior cancellation gate PR #143,
  and bluetape-rs cancellation work.
- GNO docs search had no direct #213 design notes.
- Current `testing` package has `Eventually` and `Consistently` helpers.
- Current `testing/concurrency` package already provides `AsyncJobTester`,
  timeout-aware runs, cancellation reporting, `Report.Scheduled`, and
  `Report.Skipped`.
- Kotlin source concepts reviewed:
  - `StressTester` / `WorkerStressTester` for workers/rounds bounds.
  - `MultithreadingTester` for fixed worker failure aggregation.
  - `StructuredTaskScopeTester` for structured timeout and cleanup.
  - `SuspendedJobTester` for cancellation as a structural async signal.

## Selected API

Implement in package `testing` (`github.com/bluetape4k/bluetape-go/testing`),
because these are assertion helpers alongside `Eventually` and `Consistently`.

```go
type ContextOperation func(context.Context) error
type WaiterProbe func(context.Context, func()) error
type CleanupProbe func(context.Context, func(), func()) error

func CheckContextCanceled(operation ContextOperation) error
func RequireContextCanceled(tb testing.TB, operation ContextOperation)

func CheckDeadlineExceeded(timeout time.Duration, operation ContextOperation) error
func RequireDeadlineExceeded(tb testing.TB, timeout time.Duration, operation ContextOperation)

func CheckWaiterReleased(timeout time.Duration, waiter WaiterProbe) error
func RequireWaiterReleased(tb testing.TB, timeout time.Duration, waiter WaiterProbe)

func CheckCleanupOnCancel(timeout time.Duration, probe CleanupProbe) error
func RequireCleanupOnCancel(tb testing.TB, timeout time.Duration, probe CleanupProbe)
```

## Contracts

- `CheckContextCanceled` creates an already canceled context, calls the
  operation, and requires an error matching `context.Canceled`.
- `CheckDeadlineExceeded` creates a timed context, calls the operation, and
  requires an error matching `context.DeadlineExceeded`.
- `CheckWaiterReleased` starts the waiter in a goroutine, waits for the waiter
  to call `ready`, cancels the context, and requires the waiter to return with
  `context.Canceled` before `timeout`.
- `CheckCleanupOnCancel` starts the probe in a goroutine, waits for `ready`,
  cancels the context, requires `cleaned` to be called, and requires the probe
  to return with `context.Canceled` before `timeout`.
- `Require*` wrappers call `tb.Helper()` and `tb.Fatalf` with the `Check*`
  diagnostic.
- Invalid input (`nil` operation/probe, non-positive timeout) returns a clear
  error from `Check*` and fails from `Require*`.
- Helpers distinguish cancellation errors from ordinary nil/operational errors.
- Helpers do not retry `context.Canceled` or `context.DeadlineExceeded`.
- Helpers are cooperative: they cannot forcibly stop a goroutine that ignores
  `ctx.Done()`. Timeout diagnostics identify probes that never call `ready`,
  never return, or never call `cleaned`.

## Tests

Use TDD:

- Add failing tests for each success contract.
- Add failing-diagnostic tests for nil operation, wrong error, nil return,
  waiter not ready, waiter not released, cleanup not called, and non-positive
  timeout.
- Add example tests for a callback wrapper and resource cleanup probe.
- Run race tests for the changed package.

## Documentation

Update `testing/README.md` and `testing/README.ko.md`:

- Show cancellation assertion usage.
- Explain cooperative cancellation limits.
- Mention `testing/concurrency` when repeated stress execution is needed.

## Validation

- `go test -count=1 ./testing`
- `go test -race -count=1 ./testing`
- `go test -count=1 ./testing/...`
- `make fmt-check`
- `make vet`
- `make lint`
- `make race`
- `git diff --check`

## Lightweight 7-Tier Review

Native subagent lanes are not used because prior lane cleanup in this session
has been unreliable; main-session local 7-tier fallback performed.

| Tier | Perspective | P0 | P1 | Notes |
|---|---:|---:|---:|---|
| 1 | Performance | 0 | 0 | Helpers use bounded goroutines/channels only in tests. |
| 2 | Stability | 0 | 0 | Timeout, release, cleanup, and wrong-error diagnostics are required. |
| 3 | Security | 0 | 0 | Test-only helpers; no IO/auth/secret surface. |
| 4 | Operator/Ops | 0 | 0 | Diagnostics must name the broken cancellation contract. |
| 5 | Developer/API | 0 | 0 | `Check*` functions make failure diagnostics testable; `Require*` matches assertion style. |
| 6 | User/Caller | 0 | 0 | README must explain when to use root assertions vs stress helpers. |
| 7 | Integration | 0 | 0 | Scope stays inside `testing` plus docs. |

P0 = 0, P1 = 0. Proceed with TDD implementation.
