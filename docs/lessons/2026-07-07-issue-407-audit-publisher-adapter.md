# Issue #407 audit publisher adapter 교훈

일자: 2026-07-07
범위: `audit/sqloutbox/sqloutboxtest`

## 교훈

첫 audit publisher adapter는 test/example helper package로 두는 것이 가장 안전하다.
Kafka, NATS, Redis Streams, RabbitMQ, Redpanda, Pulsar topology를 성급히 선택하지 않고도
`sqloutbox.Publisher` contract를 실행해 볼 수 있다.

## 패턴

- `audit`는 storage-neutral하게 유지한다.
- `audit/sqloutbox`는 PostgreSQL outbox state와 relay semantics를 책임진다.
- Deterministic publisher helper는 `audit/sqloutbox/sqloutboxtest` 아래에 둔다.
- Concurrent helper state에는 `GoroutineStressTester`를 사용하고, retry/dead-letter
  handoff에는 relay-backed Testcontainers test를 둔다.
- 새 diagram이 필요하지 않은 조건을 문서화한다. Package가 기존 participant만 구현하고 새
  runtime sequence/topology를 추가하지 않는다면 중복 그림을 만들지 말고 existing class/sequence
  diagram을 연결한다.

## 후속 작업

Durable broker adapter는 topology, authentication/TLS, redaction, replay, idempotency,
operator runbook contract를 각각 가진 별도 package여야 한다.
