# testing

[English](README.md) | [한국어](README.ko.md)

`testing`은 bluetape-go test를 위한 작은 asynchronous assertion helper를
제공합니다. 조건이 eventually true가 되어야 하거나 짧은 observation window 동안
true로 유지되어야 할 때 사용합니다.

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
```

## 동작

- Helper는 condition이 expected timing contract를 만족하지 않으면 supplied
  `*testing.T`를 실패시킵니다.
- `EventuallyWithPolling`과 `ConsistentlyWithPolling`은 explicit polling interval을
  허용합니다.
- 이 패키지는 test 전용입니다. Production retry/timeout behavior는 `resilience`에
  둡니다.

## 테스트

```bash
go test -count=1 ./testing
```
