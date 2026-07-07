# Issue #443 Concurrency Stress Coverage

## Decision

Use `testing/concurrency` helpers as the default proof surface for bluetape-go
stress tests instead of ad hoc goroutine orchestration.

## Lesson

Before adding stress coverage, classify each candidate module by owned
contract: shared state, goroutine-safe public claim, retry/timeout behavior, or
external resource lifecycle. Add `GoroutineStressTester` or `AsyncJobTester`
only where the package owns the concurrency or cancellation behavior being
proved.

## Applied Contract

- Shared-state or goroutine-safe helper claims need bounded stress tests and a
  matching `go test -race` pass.
- Cancellation and deadline behavior should preserve caller-owned
  `context.Canceled` or `context.DeadlineExceeded` without retrying it.
- External-resource packages should not get stress tests only to create
  activity; they need a concrete lifecycle or synchronization risk first.
