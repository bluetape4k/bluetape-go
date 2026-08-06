# Issue #405 교훈: audit publisher target

## 결정

첫 audit publisher adapter는 Kafka, NATS, Redis Streams가 아니라 deterministic
standard-library test/discard publisher여야 한다.

## 교훈

- 첫 publisher 범위는 `audit/sqloutbox.Publisher` contract에 집중한다: cancellation,
  duplicate attempt, retry handoff, shutdown.
- Broker adapter는 contract와 example이 안정된 뒤 사용한다.
- Workshop example에는 broker emulation보다 먼저 deterministic outbox behavior가 필요하다.
- Transport package는 audit history store가 되거나 topology, retention, replay,
  idempotency ownership을 숨기면 안 된다.

## 후속 작업

- #407은 선택한 test/discard publisher helper를 구현해야 한다.
- Transport-specific issue는 Kafka, NATS, Redis Streams를 선택하는 구체적인 workshop 또는
  application scenario가 생겼을 때 따로 만든다.
