# Issue #443 concurrency stress coverage

## 결정

bluetape-go stress test의 기본 증명 표면은 ad hoc goroutine orchestration이 아니라
`testing/concurrency` helper로 둔다.

## 교훈

Stress coverage를 추가하기 전에 후보 module을 owned contract 기준으로 분류한다:
shared state, goroutine-safe public claim, retry/timeout behavior, external resource
lifecycle 중 무엇을 package가 소유하는지 먼저 판단한다. Package가 증명해야 할 concurrency
또는 cancellation behavior를 소유할 때만 `GoroutineStressTester`나 `AsyncJobTester`를
추가한다.

## 적용된 계약

- Shared-state 또는 goroutine-safe helper claim에는 bounded stress test와 대응하는
  `go test -race` 통과가 필요하다.
- Cancellation과 deadline behavior는 caller-owned `context.Canceled` 또는
  `context.DeadlineExceeded`를 retry하지 않고 보존해야 한다.
- External-resource package는 activity를 만들기 위한 stress test를 받지 않는다. 먼저
  concrete lifecycle 또는 synchronization risk가 있어야 한다.
