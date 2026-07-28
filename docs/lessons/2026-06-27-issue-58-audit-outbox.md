# Issue 58 audit outbox 교훈

## 맥락

Issue #58은 #56 audit model과 #57 repository/history contract 다음 단계다. 어떤
outbox/publisher adapter를 먼저 둘지 결정한다.

## 교훈

- Broker를 고르기 전에 durable outbox boundary를 먼저 정한다. Kafka, NATS, Redis
  Streams, RabbitMQ, Redpanda, Pulsar는 delivery/projection 선택지이지 durable audit
  outbox state를 대체하지 않는다.
- 첫 outbox 구현은 SQL이어야 한다. #100이 이미 runtime-first `database/sql`과
  PostgreSQL-first 방향을 세웠고, #41도 SQL을 유력한 durable history/outbox boundary로
  평가했다.
- At-least-once delivery와 duplicate publish attempt는 first-class contract여야 한다.
  Event ID와 idempotency key uniqueness가 durable dedupe handle이며, exactly-once
  delivery는 범위 밖이다.
- Implementation issue가 caller-supplied transaction/session hook을 명시적으로
  수용하기 전까지 transaction choreography는 application-owned로 둔다.
- Outbox relay code는 concurrency-sensitive하다. #346이 async relay loop를 추가할 때는
  cancellation, retry/dead-letter, stress, race coverage가 필요하다.
