# testing/concurrency

[English](README.md) | [한국어](README.ko.md)

`testing/concurrency` provides deterministic helpers for stress and async job
tests. It is useful when a package needs repeated bounded goroutine execution,
panic capture, cancellation checks, or timeout reports.

![testing concurrency harness map](../../docs/images/readme-diagrams/testing-concurrency-harness-map.png)

## Import

```go
import concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
```

## Usage

```go
tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
    Workers:       4,
    RoundsPerTask: 10,
})

report, err := tester.Run(ctx, func(context.Context) error {
    return exerciseConcurrentPath()
})
if err != nil {
    return err
}
_ = report.Completed
```

For context-aware background jobs:

```go
tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
    Workers: 2,
    Timeout: time.Second,
})

report := tester.RunT(t, func(ctx context.Context) error {
    return runJob(ctx)
})
```

## Behavior

- `GoroutineStressTester` runs each task for `RoundsPerTask` rounds across a
  bounded worker count.
- `AsyncJobTester` runs jobs with a timeout-aware context.
- Panics and task errors are reported as failures in the returned `Report`.
- `RunT` fails the supplied test when the run reports an error.

## Test

```bash
go test -count=1 ./testing/concurrency
```
