# Issue 27 Workflow Runners Spec

Issue: #27
Milestone: 0.4.0
Parent spec: `docs/superpowers/specs/2026-06-05-issue-135-0.4.0-state-workflow-spec.md`
Dependency: `workreport` from #28

## Problem

0.4.0 needs lightweight workflow runners that compose ordinary Go work
functions and return structured `workreport.Report` values. The package must
port only the useful parts of Kotlin `utils/workflow`: sequential execution,
parallel fan-out, and conditional branching with caller-owned cancellation.

The package must not become a workflow engine, DSL, scheduler, retry runtime,
or mutable shared-context framework.

## Current Evidence

- #135 defines the package split as `state`, `workreport`, and `workflow`.
- #28 added `workreport.Report`, `Status`, `FailurePolicy`, and `Aggregate`.
- The #27 issue requires sequential, parallel, and conditional runners with
  context cancellation.
- The parent plan orders #27 after #28 because runners should return
  `workreport` results.
- Existing concurrency helpers use caller contexts, bounded goroutines, and
  race-compatible tests.

## Goals

- Add a first-party `workflow` package.
- Define:
  - `type Work func(context.Context) workreport.Report`
  - `type Runner interface { Run(context.Context) workreport.Report }`
- Implement sequential, conditional, and parallel runners.
- Keep runner construction explicit and small.
- Preserve child report order for deterministic inputs.
- Propagate caller cancellation through `context.Context`.
- Stop immediately on aborted or cancelled child reports.
- Add unit, cancellation, stress, race, README, and example coverage.

## Non-Goals

- Do not add a durable workflow engine.
- Do not add a Kotlin-style DSL, reflection builder, or mutable `WorkContext`
  map.
- Do not add retry, repeat, scheduler, observer, event, coroutine, reactive, or
  virtual-thread concepts.
- Do not add new dependencies.
- Do not make any-success parallel semantics part of #27 unless the API remains
  smaller than the required all-branches surface. The default for #27 is
  all-branches semantics.

## API Direction

The package should expose ordinary Go constructors that return `Runner` values:

```go
type Work func(context.Context) workreport.Report

type Runner interface {
    Run(context.Context) workreport.Report
}

func Sequential(name string, policy workreport.FailurePolicy, works ...Work) Runner
func Conditional(name string, predicate Predicate, trueWork Work, falseWork ...Work) Runner
func Parallel(name string, policy workreport.FailurePolicy, works ...Work) Runner

type Predicate func(context.Context) (bool, error)
```

The exact concrete runner types may remain unexported. Public behavior should
be through `Run`.

## Error And Validation Contract

- Nil caller contexts are normalized to `context.Background()`.
- Running a nil `Work` returns a failed report with a caller-checkable error.
- Running a conditional runner with a nil predicate returns a failed report.
- Predicate errors return a failed report and do not run either branch.
- Unknown failure policies return a failed report whose error matches
  `workreport.ErrUnknownFailurePolicy`.
- Caller cancellation returns a cancelled report with the caller context error.

## Sequential Behavior

- Runs works in input order.
- `StopOnFailure` stops after the first failed or partial child.
- `ContinueOnFailure` continues after failed or partial child reports and
  returns a partial aggregate.
- Aborted and cancelled child reports stop immediately regardless of failure
  policy.
- Child report order matches execution order.

## Conditional Behavior

- Evaluates exactly one predicate.
- Runs only the selected branch.
- If the predicate is false and no false branch exists, returns completed.
- False branch accepts at most one work item.
- Selected branch receives the same caller context semantics as other runners.
- Predicate cancellation is reported as cancelled when it returns a context
  cancellation error.

## Parallel Behavior

- Uses all-branches semantics.
- Starts each work item with a shared cancellable context derived from the
  caller context.
- Preserves child reports in input order.
- `StopOnFailure` cancels sibling work after the first failed or partial child.
- `ContinueOnFailure` aggregates every child unless caller cancellation or an
  aborted/cancelled child requires early cancellation.
- Aborted and cancelled child reports cancel siblings immediately.
- The runner waits for all started goroutines before returning.
- The runner does not leak goroutines after cancellation.

## Test Requirements

- Unit tests cover sequential stop and continue policies.
- Unit tests cover aborted/cancelled stopping semantics.
- Unit tests cover conditional true branch, false branch, missing false branch,
  predicate error, and nil predicate.
- Unit tests cover parallel aggregation, input-order preservation, cancellation
  propagation, and unknown policy handling.
- `GoroutineStressTester` covers repeated parallel runner fan-out.
- `AsyncJobTester` covers cancellation/deadline propagation.
- `go test -race -count=1 ./workflow ./workreport` must pass.

## Documentation Requirements

- Add `workflow/doc.go`.
- Add `workflow/README.md` and `workflow/README.ko.md`.
- Add compile-checked examples for sequential, conditional, and parallel
  runners.
- Root README links are owned by #132 unless this PR updates them explicitly.
- Diagrams are owned by #133 after the API stabilizes.

## Risks And Mitigations

| Risk | Mitigation |
|---|---|
| API grows into a workflow DSL | Keep constructors small and concrete; use ordinary closures for input/output. |
| Parallel cancellation leaks goroutines | Use a derived context, cancel on early-stop events, wait for all goroutines, and stress/race test. |
| Failure policy semantics drift from `workreport` | Use `workreport.Aggregate` for final parent reports and return checkable failures for invalid policies. |
| Conditional runner accidentally runs both branches | Unit tests assert exact branch execution counts. |
| Mutable context map sneaks into examples | Examples use closures and explicit variables only. |

## Acceptance Criteria

- `workflow` compiles without new dependencies.
- Sequential runner stops or continues according to policy.
- Parallel runner propagates cancellation, aggregates child reports, and avoids
  goroutine leaks.
- Conditional runner has branch tests.
- Stress/cancellation helpers are used where they add signal.
- README pair and compile-checked examples exist.
- Local 7-tier review reaches `P0=0 P1=0`.
