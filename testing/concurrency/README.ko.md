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
- Panic과 task error는 returned `Report`의 failure로 보고됩니다.
- `RunT`는 run이 error를 보고하면 supplied test를 실패시킵니다.

## 테스트

```bash
go test -count=1 ./testing/concurrency
```
