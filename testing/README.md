# testing

[English](README.md) | [한국어](README.ko.md)

`testing` provides small asynchronous assertion helpers for bluetape-go tests.
Use it when a condition should become true eventually, remain true for a short
observation window, or prove a context cancellation contract.

![testing concurrency harness map](../docs/images/readme-diagrams/testing-concurrency-harness-map.png)

## Import

```go
import bttesting "github.com/bluetape4k/bluetape-go/testing"
```

## Usage

```go
var ready atomic.Bool
go func() {
    time.Sleep(10 * time.Millisecond)
    ready.Store(true)
}()

bttesting.Eventually(t, time.Second, ready.Load)
bttesting.Consistently(t, 100*time.Millisecond, ready.Load)

bttesting.RequireContextCanceled(t, func(ctx context.Context) error {
    return ctx.Err()
})

bttesting.RequireCleanupOnCancel(t, 50*time.Millisecond, func(ctx context.Context, ready func(), cleaned func()) error {
    ready()
    <-ctx.Done()
    cleaned()
    return ctx.Err()
})
```

## Behavior

- Helpers fail the supplied `*testing.T` when the condition does not satisfy the
  expected timing contract.
- `EventuallyWithPolling` and `ConsistentlyWithPolling` allow explicit polling
  intervals.
- `CheckContextCanceled` and `CheckDeadlineExceeded` return diagnostics when an
  operation masks `context.Canceled` or `context.DeadlineExceeded`.
- `CheckWaiterReleased` and `CheckCleanupOnCancel` prove cooperative waiter
  release and cleanup after cancellation. The matching `Require*` helpers fail
  the supplied `testing.TB`.
- Cancellation helpers are cooperative: the operation under test must observe
  `ctx.Done()` or return. Go cannot safely stop a goroutine that ignores its
  context forever.
- Use `testing/concurrency` when the test also needs repeated bounded
  goroutine execution, panic aggregation, or stress reporting.
- The package is intended for tests only; production retry or timeout behavior
  belongs in `resilience`.

## Test

```bash
go test -count=1 ./testing
go test -race -count=1 ./testing
```
