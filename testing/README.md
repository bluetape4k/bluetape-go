# testing

[English](README.md) | [한국어](README.ko.md)

`testing` provides small asynchronous assertion helpers for bluetape-go tests.
Use it when a condition should become true eventually or should remain true for
a short observation window.

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
```

## Behavior

- Helpers fail the supplied `*testing.T` when the condition does not satisfy the
  expected timing contract.
- `EventuallyWithPolling` and `ConsistentlyWithPolling` allow explicit polling
  intervals.
- The package is intended for tests only; production retry or timeout behavior
  belongs in `resilience`.

## Test

```bash
go test -count=1 ./testing
```
