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
- `Report.Scheduled` is the total planned task execution count. `Started`,
  `Completed`, `Failures`, `Panics`, `Skipped`, and `MaxConcurrent` make the
  run outcome deterministic even when cancellation stops queued work before it
  starts.
- `Skipped` counts scheduled executions that never started because the run
  context ended before they were queued.
- Panics, task errors, and run-level cancellation/timeout errors are reported
  as failures in the returned `Report`.
- `RunT` fails the supplied test when the run reports an error.
- Timeout and cancellation are cooperative. Tasks must observe `ctx.Done()` or
  return; Go cannot safely stop a goroutine that ignores its context forever.

Use these helpers when the test needs repeated bounded goroutine execution,
panic aggregation, context cancellation proof, timeout proof, or deterministic
failure accounting. Prefer plain table tests, `testing`, `sync.WaitGroup`, or
`errgroup` when a single small concurrency assertion is enough and no repeated
stress/reporting contract is needed.

## Test

```bash
go test -count=1 ./testing/concurrency
go test -race -count=1 ./testing/concurrency
```
