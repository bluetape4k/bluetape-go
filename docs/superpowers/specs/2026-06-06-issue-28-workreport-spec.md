# Issue 28 Workreport Spec

Issue: #28
Milestone: 0.4.0
Parent spec: `docs/superpowers/specs/2026-06-05-issue-135-0.4.0-state-workflow-spec.md`

## Problem

0.4.0 needs a shared result model before `workflow` runners are implemented.
The model must describe completed, failed, partial, aborted, and cancelled work
without coupling callers to a runner implementation.

`workreport` is the dependency boundary for #27. It should be small enough that
ordinary Go work functions can return reports directly, and explicit enough that
future sequential and parallel runners can aggregate child outcomes without
inventing runner-local status semantics.

## Current Evidence

- #135 defines the 0.4.0 package split as `state`, `workreport`, and
  `workflow`, with `workflow` importing `workreport`.
- #135 requires statuses, failure policy, report tree, timestamps, error
  storage, helper predicates, deterministic child ordering, and documented
  zero-value behavior.
- `state` already follows the package shape expected for 0.4.0: `doc.go`,
  narrow public API files, typed errors, unit tests, concurrency tests, examples,
  and package READMEs.
- Existing Go packages keep APIs first-party, small, and framework-free. The
  implementation must not copy Kotlin DSL or coroutine surfaces.

## Goals

- Add a new `workreport` package.
- Define stable `Status` values:
  - `completed`
  - `failed`
  - `partial`
  - `aborted`
  - `cancelled`
- Define explicit `FailurePolicy` values:
  - stop on first failure
  - continue after ordinary failures
- Define a `Report` value with name, status, start/end timestamps, error,
  reason, and child reports.
- Provide helper constructors for common terminal outcomes.
- Provide helper predicates for terminal, success, failure, partial, aborted,
  and cancelled checks.
- Provide aggregation helpers that preserve deterministic child order and make
  sequential/parallel runner behavior straightforward for #27.
- Document zero-value behavior.
- Add deterministic unit tests, race-compatible tests, and compile-checked
  examples.

## Non-Goals

- Do not implement `workflow` runners in #28.
- Do not add a durable workflow engine.
- Do not add a mutable shared work context.
- Do not add retry, repeat, scheduler, observer, coroutine, reactive, or virtual
  thread concepts.
- Do not add external dependencies.
- Do not hide causal errors behind strings.

## API Direction

`Report` should be an ordinary value type. Mutating builder APIs are not needed
for 0.4.0; callers can use constructors or struct literals.

Expected shape:

```go
type Status string

const (
    StatusCompleted Status = "completed"
    StatusFailed    Status = "failed"
    StatusPartial   Status = "partial"
    StatusAborted   Status = "aborted"
    StatusCancelled Status = "cancelled"
)

type FailurePolicy int

const (
    StopOnFailure FailurePolicy = iota
    ContinueOnFailure
)

type Report struct {
    Name      string
    Status    Status
    StartedAt time.Time
    EndedAt   time.Time
    Err       error
    Reason    string
    Children  []Report
}
```

Constructor direction:

- `Completed(name string) Report`
- `Failed(name string, err error) Report`
- `Partial(name string, children ...Report) Report`
- `Aborted(name, reason string) Report`
- `Cancelled(name string, err error) Report`
- `Aggregate(name string, policy FailurePolicy, children ...Report) (Report, error)`

The exact helper names can be refined during implementation if tests show a
clearer Go shape, but the exported API must stay narrow and predictable.

## Behavior Contract

- A zero-value `Report` has an unknown status and is not considered successful,
  failed, partial, aborted, cancelled, or terminal.
- `completed` means the work finished without an error or failed child.
- `failed` means the work failed with a caller-visible error.
- `partial` means the parent completed enough work to report children, but at
  least one child failed while other children may have completed.
- `aborted` means execution stopped for a policy or caller-defined reason.
- `cancelled` means caller cancellation or deadline stopped the work.
- `failed`, `partial`, `aborted`, and `cancelled` are failure outcomes for
  parent aggregation.
- `failed`, `aborted`, and `cancelled` are terminal single-work outcomes.
- Aggregation preserves the input child order.
- Under `StopOnFailure`, aggregation returns the first non-completed child as
  the parent status and keeps children through the first stopping child.
- Under `ContinueOnFailure`, aggregation includes all children and returns
  `partial` when any child is failed, partial, aborted, or cancelled.
- If all children are completed, aggregation returns `completed`.
- If no children are supplied, aggregation returns `completed`.
- Failure policy validation must reject unknown policy values through
  `Aggregate` with an `errors.Is`-compatible sentinel.

## Error Contract

- `Failed` and `Cancelled` preserve caller-visible errors in `Report.Err`.
- Aggregation helpers do not convert child errors into strings.
- Unknown failure policies return an error that callers can check with
  `errors.Is`.
- `context.Canceled` and `context.DeadlineExceeded` remain caller-owned errors
  when they appear in cancellation reports; the package does not retry or wrap
  them away.

## Test Requirements

- Unit tests cover every status predicate.
- Unit tests cover constructors, timestamps, error preservation, reason fields,
  and child order.
- Unit tests cover `StopOnFailure`, `ContinueOnFailure`, no-child aggregation,
  and unknown policy validation.
- A race-compatible test repeatedly aggregates shared immutable child input with
  `GoroutineStressTester`.
- A cancellation-shaped test uses `AsyncJobTester` to prove cancellation reports
  preserve `context.Canceled`.
- `go test -count=1 ./workreport` and `go test -race -count=1 ./workreport`
  must pass.

## Documentation Requirements

- Add `workreport/doc.go`.
- Add `workreport/README.md` and `workreport/README.ko.md`.
- Add at least one compile-checked `Example*`.
- Root README links are owned by #132 unless this PR chooses to include them as
  part of package README completeness.

## Risks And Mitigations

| Risk | Mitigation |
|---|---|
| API becomes Kotlin-shaped or runner-specific | Keep `workreport` as value types plus narrow helpers; defer runner behavior to #27. |
| Failure aggregation hides child errors | Preserve children and `Err`; tests assert causal errors survive. |
| Stop/continue policy semantics drift before #27 | Encode policy behavior in `Aggregate` tests before `workflow` consumes it. |
| Zero-value report is accidentally treated as success | Predicate tests require zero value to be unknown and non-terminal. |
| Concurrency gate is treated as optional | Use `GoroutineStressTester`, `AsyncJobTester`, and race validation in #28. |

## Acceptance Criteria

- `workreport` compiles without new dependencies.
- Statuses, failure policies, reports, constructors, predicates, and aggregation
  helpers are implemented and tested.
- `Report` preserves errors and child reports.
- Zero-value behavior is documented and tested.
- Stress/cancellation helpers are used where they add signal.
- Package README pair and compile-checked examples exist.
- Step 6-R review reaches `P0=0 P1=0`.
