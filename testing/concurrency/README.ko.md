# testing/concurrency

[English](README.md) | [한국어](README.ko.md)

`testing/concurrency`는 stress test와 async job test를 위한 deterministic helper를
제공합니다. Package가 repeated bounded goroutine execution, panic capture,
cancellation check, timeout report를 필요로 할 때 유용합니다.

![testing concurrency harness map](../../docs/images/readme-diagrams/testing-concurrency-harness-map.png)

## 가져오기

```go
import concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
```

## 사용 예

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

Context-aware background job에는 다음 helper를 사용합니다.

```go
tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
    Workers: 2,
    Timeout: time.Second,
})

report := tester.RunT(t, func(ctx context.Context) error {
    return runJob(ctx)
})
```

## 동작

- `GoroutineStressTester`는 bounded worker count로 각 task를 `RoundsPerTask` round
  동안 실행합니다.
- `AsyncJobTester`는 timeout-aware context로 job을 실행합니다.
- `Report.Scheduled`는 예정된 전체 task 실행 수입니다. `Started`,
  `Completed`, `Failures`, `Panics`, `Skipped`, `MaxConcurrent`로 cancellation이
  queued work를 시작 전에 멈춘 경우에도 결과를 deterministic하게 설명할 수 있습니다.
- `Skipped`는 run context가 끝나 queue에 들어가기 전에 시작하지 못한 scheduled
  execution 수입니다.
- Panic, task error, run-level cancellation/timeout error는 returned `Report`의
  failure로 보고됩니다.
- `RunT`는 run이 error를 보고하면 supplied test를 실패시킵니다.
- Timeout과 cancellation은 cooperative contract입니다. Task는 `ctx.Done()`을
  관찰하거나 반환해야 합니다. Go는 context를 영원히 무시하는 goroutine을 안전하게
  중지할 수 없습니다.

반복되는 bounded goroutine execution, panic aggregation, context cancellation
proof, timeout proof, deterministic failure accounting이 필요할 때 이 helper를
사용하세요. 단일의 작은 concurrency assertion이면 plain table test, `testing`,
`sync.WaitGroup`, `errgroup`를 우선하세요.

## 테스트

```bash
go test -count=1 ./testing/concurrency
go test -race -count=1 ./testing/concurrency
```
