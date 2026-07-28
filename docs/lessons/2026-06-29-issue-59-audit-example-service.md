# Issue #59 교훈: audit example service

## 결정

Runnable Go example은 Ktor, Spring, Exposed, Kafka, Redis wiring을 복사하지 않고,
plain order service로 audit boundary를 보여 준다.

## 교훈

- 유용한 source parity는 JVM framework container가 아니라 source-write와
  audit-history의 경계다.
- Example code는 `examples/` 아래에 둔다. 조합 방법을 보여 주되 production helper API가
  되지 않게 한다.
- Service-free example에는 in-memory outbox replay면 충분하다. 다만 README가 durable
  delivery는 `audit/sqloutbox`로 이어진다고 명확히 말해야 한다.

## 후속 작업

- Workshop example은 나중에 `examples/audit` 개념을 실제 SQL outbox나 broker adapter와
  조합할 수 있다.
- 미래 HTTP example은 current read가 source model, audit history, projection 중 어디에서
  오는지 명시해야 한다.
