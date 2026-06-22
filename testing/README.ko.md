# testing

[English](README.md) | [한국어](README.ko.md)

`testing`은 bluetape-go test를 위한 작은 asynchronous assertion helper를
제공합니다. 조건이 eventually true가 되어야 하거나 짧은 observation window 동안
true로 유지되어야 할 때, 또는 context cancellation contract를 증명해야 할 때
사용합니다.

![testing concurrency harness map](../docs/images/readme-diagrams/testing-concurrency-harness-map.png)

## 가져오기

```go
import bttesting "github.com/bluetape4k/bluetape-go/testing"
```

## 사용 예

```go
var ready atomic.Bool
go func() {
    time.Sleep(10 * time.Millisecond)
    ready.Store(true)
}()

bttesting.Eventually(t, time.Second, ready.Load)
bttesting.Consistently(t, 100*time.Millisecond, ready.Load)

bttesting.RequireAwaitValue(context.Background(), t, time.Second, 10*time.Millisecond, func(ctx context.Context) (string, error) {
    return cache.Get(ctx, "customer:42")
}, "fresh")

bttesting.RequireAwait(context.Background(), t, time.Second, 10*time.Millisecond, func(ctx context.Context) (bool, error) {
    return lock.TryAcquire(ctx, "invoice:42")
}, func(acquired bool, err error) bttesting.AwaitStatus {
    if err != nil {
        return bttesting.AwaitFailure
    }
    if acquired {
        return bttesting.AwaitSuccess
    }
    return bttesting.AwaitContinue
})

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

## 동작

- Helper는 condition이 expected timing contract를 만족하지 않으면 supplied
  `*testing.T`를 실패시킵니다.
- `EventuallyWithPolling`과 `ConsistentlyWithPolling`은 explicit polling interval을
  허용합니다.
- `CheckAwait`와 `RequireAwait`는 context-aware probe를 polling하고 supplied
  check가 `AwaitSuccess` 또는 `AwaitFailure`를 반환할 때 멈춥니다. Diagnostic에는
  final observed value/error와 attempt count가 포함됩니다.
- `CheckAwaitValue`/`RequireAwaitValue`는 eventually expected value를 기다리고,
  `CheckAwaitError`/`RequireAwaitError`는 expected non-context error state를
  기다립니다.
- `CheckContextCanceled`와 `CheckDeadlineExceeded`는 operation이
  `context.Canceled` 또는 `context.DeadlineExceeded`를 숨기는 경우 diagnostic을
  반환합니다.
- `CheckWaiterReleased`와 `CheckCleanupOnCancel`은 cancellation 이후 cooperative
  waiter release와 cleanup을 증명합니다. 대응하는 `Require*` helper는 supplied
  `testing.TB`를 실패시킵니다.
- Await/cancellation helper는 cooperative contract입니다. 테스트 대상 operation은
  `ctx.Done()`을 관찰하거나 반환해야 합니다. Go는 context를 영원히 무시하는
  goroutine을 안전하게 중지할 수 없습니다. Helper는 caller-owned
  `context.Canceled` 또는 `context.DeadlineExceeded`를 retry하지 않습니다.
- Await helper는 eventual cache invalidation, lock acquisition, Testcontainers
  readiness, workflow status, HTTP mock verification 같은 bounded test observation에
  사용하세요.
- 반복되는 bounded goroutine execution, panic aggregation, stress reporting까지
  필요하면 `testing/concurrency`를 사용하세요.
- 이 패키지는 test 전용입니다. Production retry/timeout behavior는 `resilience`에
  둡니다.

## 테스트

```bash
go test -count=1 ./testing
go test -race -count=1 ./testing
```
