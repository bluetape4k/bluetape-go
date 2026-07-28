# Issue #406 교훈: audit publisher contract

## 결정

`audit/sqloutbox.Publisher`는 작은 `context.Context` plus `Record` contract로 남긴다.
Generic message-bus abstraction이 아니다.

## 교훈

- Caller context cancellation은 publisher failure가 아니라 shutdown으로 취급한다. Relay는
  cancellation error를 반환하고 claimed row를 lease-based recovery에 맡겨야 한다.
- 다른 publish error는 모두 `MarkFailed`를 통해 retry/dead-letter state로 반영한다.
- Helper나 broker adapter를 추가하기 전에 stable event ID와 idempotency key로 duplicate
  publish attempt를 증명한다.
- Cancellation lifecycle coverage에는 `AsyncJobTester`를, concurrent relay/store path처럼
  package가 goroutine-sensitive behavior를 소유하는 경우에는 `GoroutineStressTester`를
  사용한다.
- Public participant 또는 sequence shape가 바뀌지 않으면 diagram을 추가하거나 다시 그리지
  않는다. Existing class와 sequence diagram이 시각적으로 유효하다면 contract refinement에는
  README prose로 충분하다.

## 후속 작업

- #407은 이 contract에 맞춰 deterministic test/discard publisher helper를 추가할 수 있다.
- #408은 example에 contract language를 반영하고 transport adapter를 audit history store처럼
  보이지 않게 해야 한다.
