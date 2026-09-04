# #520 EventBridge 감사 publisher 교훈

## 결정

`audit/sqloutbox/eventbridge`는 새로운 broker abstraction이 아니라
`sqloutbox.Publisher`에 맞춘 얇은 single-entry adapter로 유지한다. AWS SDK
client와 credential, retry, timeout, bus/rule/target provisioning은 caller가
소유한다. `Record.EventID`와 `Record.IdempotencyKey`는 JSON detail에 그대로
전달하고 EventBridge response `EventId`와 혼동하지 않는다.

## 재사용할 패턴

- SDK method subset만 interface로 주입하면 `*eventbridge.Client`와 fake가 같은
  compile contract를 갖고 live AWS 없이 failure/cancellation을 재현할 수 있다.
- HTTP-level success를 성공으로 간주하지 않고 `FailedEntryCount`와 entry
  `ErrorCode`/`ErrorMessage`를 결정론적으로 검사해야 한다.
- EventBridge entry size는 detail만이 아니라 source/type/bus metadata overhead를
  포함해 dispatch 전에 계산해야 한다.
- transport 원인은 `errors.Is`로 관찰하되 `Error()`/`%+v`와 persisted relay
  failure text에는 raw provider message를 넣지 않는다.
- cancellation은 SDK 호출 전과 response 직후 우선 검사하고, existing relay가
  caller shutdown을 retry/dead-letter로 바꾸지 않게 한다.

## 유예 범위

Kinesis #521, Step Functions #522, batching, live AWS smoke, production IAM
rollout, downstream idempotency storage는 이 adapter에서 구현하지 않는다.
다음 slice가 같은 outbox identity를 필요로 하더라도 각 issue가 독립 설계와
exact-head PR gate를 거친다.

## 검증

fake client의 request deep-copy, blocking/cancellation, output-plus-error,
redaction, concurrent isolation과 package normal/race/vet 테스트가 이 교훈을
구체화한다. Repository-wide Testcontainers 결과는 changed package 결과와
분리해 기록한다.
