# Issue 533 - Redis Streams sqloutbox publisher 교훈

## Context

Issue #533은 첫 broker-backed `audit/sqloutbox.Publisher` provider를 추가한다. 이전
audit research는 SQL outbox를 durable source of truth로 선택했고 `Publisher`
contract가 안정될 때까지 Redis Streams를 미뤘다.

## Decision

- `audit/sqloutbox/redisstreams`를 좁은 provider package로 추가한다.
- generic message-bus abstraction 대신 작은 `XAdd` interface로 caller-owned Redis
  client를 받는다.
- sqloutbox publish attempt마다 Redis stream entry 하나를 발행한다.
- `Record.EventID`, `Record.IdempotencyKey`, aggregate identity, event type,
  schema version, attempt count, 전체 `entry_json` payload를 보존한다.
- stream trimming, retention, consumer group, replay, auth, TLS, topology는
  caller에게 맡긴다.

## Validation Notes

- Unit test는 fake `XAdd` client로 field preservation, cancellation
  short-circuiting, Redis error propagation을 증명한다.
- Testcontainers Redis test는 실제 stream append와 duplicate attempt를 증명한다.
- relay integration test는 PostgreSQL sqloutbox와 Redis Streams를 결합해
  non-cancellation publish error가 relay retry behavior로 표면화되고 성공한 retry가
  `attempts=2`를 전달함을 증명한다.

## Follow-up Guardrails

- 향후 Kafka, NATS, RabbitMQ, Redpanda, Pulsar provider는 repeated call-site
  evidence가 생기기 전 shared broker abstraction을 도입하지 말고 같은
  provider-local shape를 유지한다.
- Redis Streams는 audit storage backend가 아니라 transport provider로 남긴다.
- caller-managed stream retention이 반복 문제로 드러나면 이 publisher에 trimming
  flag를 추가하기 전에 명시적 options/operations issue로 조사한다.
