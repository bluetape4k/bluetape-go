# audit/sqloutbox/redisstreams

[English](README.md) | [한국어](README.ko.md)

`audit/sqloutbox/redisstreams`는 claimed `sqloutbox.Record` 값을 publish attempt
마다 Redis Streams에 한 번 `XADD`합니다.

## Import

```go
import redisstreams "github.com/bluetape4k/bluetape-go/audit/sqloutbox/redisstreams"
```

## Usage

이 package에는 constructor를 검증하는 compile-checked `ExampleNew`가 있습니다.
일반적인 relay wiring은 `audit/sqloutbox`와 함께 다음처럼 구성합니다.

```go
publisher, err := redisstreams.New(redisstreams.Options{
	Client: redisClient,
	Stream: "audit:sqloutbox",
})
if err != nil {
	return err
}

relay, err := sqloutbox.NewRelay(store, publisher, sqloutbox.RelayOptions{
	ClaimLimit:  10,
	MaxAttempts: 3,
	RetryDelay:  time.Second,
})
```

Redis client는 caller-owned입니다. 이 package는 Redis connection을 만들거나 닫지
않습니다.

## Stream Fields

각 Redis stream entry는 안정적인 sqloutbox envelope를 담습니다.

- `record_id`: `sqloutbox.Record.ID`의 decimal string
- `status`: publish 시점의 sqloutbox record status
- `aggregate_type`: aggregate type
- `aggregate_id`: aggregate instance ID
- `revision`: audit revision의 decimal string
- `event_id`: 안정적인 audit event ID
- `idempotency_key`: 안정적인 idempotency key
- `event_type`: audit event type
- `occurred_at`: UTC RFC3339Nano timestamp
- `recorded_at`: UTC RFC3339Nano timestamp
- `schema_version`: audit schema version의 decimal string
- `attempts`: sqloutbox claim attempt count의 decimal string
- `entry_json`: marshal된 전체 `audit.Entry`

`entry_json`은 sqloutbox가 이미 저장한 전체 audit entry JSON입니다. Redis reader나
retention이 SQL outbox와 다르면 enqueue/publish 전에 민감 payload를 redact 또는
classify해야 합니다.

Retry는 stream entry를 하나 더 append합니다. Consumer는 `event_id` 또는
`idempotency_key`로 deduplicate하고, `attempts`는 diagnostic metadata로 취급해야
합니다. Redis/client failure는 server가 `XADD`를 accept한 뒤에도 caller 관점에서는
ambiguous할 수 있으므로 operator replay와 consumer processing은 같은 안정적 event
identity를 가진 duplicate entry를 허용해야 합니다.

## Operational Boundaries

- 이 package는 Redis `XADD`만 호출합니다.
- `Stream`은 request/tenant input이 아니라 신뢰된 topology configuration입니다.
  Package는 empty/blank 값만 거부하고 caller가 제공한 Redis key를 그대로 보존합니다.
- Stream retention, trimming, consumer group, pending-entry recovery, replay,
  authentication, TLS, Redis Cluster topology는 caller-owned입니다.
- Redis Streams는 outbox record의 publish transport이지 durable audit source of
  truth가 아닙니다.
- Context cancellation은 `XADD` 전에 확인하고 Redis command error는 wrap해서
  `Relay.RunOnce`가 기존 sqloutbox contract대로 retry/dead-letter를 처리하게
  합니다. Relay failure text는 Redis error detail을 저장할 수 있으므로 Redis
  diagnostic을 민감 정보로 취급하는 배포에서는 이 adapter 앞에서 client error를 wrap
  또는 redact해야 합니다.

## Tests

Redis와 relay integration test는 Testcontainers를 사용하므로 Docker가 필요합니다.

```bash
go test -count=1 ./audit/sqloutbox/redisstreams
go test -race -count=1 ./audit/sqloutbox/redisstreams
```
