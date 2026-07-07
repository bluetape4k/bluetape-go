# audit/sqloutbox

[English](README.md) | [한국어](README.ko.md)

`audit.Entry` 값을 위한 PostgreSQL-backed audit outbox store와 relay입니다.

이 package는 transaction ownership을 숨기지 않습니다. 각 operation마다 호출자가
`database/sql` session을 넘기므로, source write와 `Store.Enqueue`를 같은
`*sql.Tx` 안에서 명시적으로 묶을 수 있습니다.

## Diagrams

![audit sqloutbox class contract map](../../docs/images/readme-diagrams/audit-sqloutbox-class-contract-map.png)

![audit sqloutbox relay sequence](../../docs/images/readme-diagrams/audit-sqloutbox-relay-sequence.png)

## Import

```go
import "github.com/bluetape4k/bluetape-go/audit/sqloutbox"
```

## Store

```go
store, err := sqloutbox.NewStore(sqloutbox.Options{})
if err != nil {
	return err
}

// 운영 환경에서는 명시적 migration을 권장합니다. CreateSchema는 local setup,
// test, package-owned DDL을 선택한 application을 위해 제공합니다.
if err := store.CreateSchema(ctx, db); err != nil {
	return err
}

err = sqlkit.WithTx(ctx, db, nil, func(ctx context.Context, tx *sql.Tx) error {
	if err := writeSource(ctx, tx, aggregate, command); err != nil {
		return err
	}
	return store.Enqueue(ctx, tx, auditEntry)
})
```

`Claim`은 PostgreSQL `FOR UPDATE SKIP LOCKED`를 사용하고 bounded claim lease를
설정하며, 만료된 claimed row를 다시 claim할 수 있습니다. 같은 aggregate에서 더
낮은 revision이 pending 또는 claimed 상태이면 이후 revision을 claim하지 않습니다.
이 방식은 global ordering을 약속하지 않으면서 store가 강제할 수 있는
per-aggregate ordering을 보존합니다.

## Relay

```go
relay, err := sqloutbox.NewRelay(store, publisher, sqloutbox.RelayOptions{
	ClaimLimit:  10,
	MaxAttempts: 3,
	RetryDelay:  time.Second,
})
if err != nil {
	return err
}

result, err := relay.RunOnce(ctx, db)
```

`RunOnce`는 scheduler가 polling을 소유할 때 사용하기 좋습니다. `Run`은 context가
취소될 때까지 반복하며 service-owned worker lifecycle에 맞춥니다.

Delivery는 at-least-once입니다. Publisher와 consumer는 duplicate publish attempt가
가능하다고 보고, 안정적인 audit event ID 또는 idempotency key로 deduplication해야
합니다.

Publisher error는 retry contract의 일부입니다.

- `Publisher.Publish`가 caller-owned context cancellation 또는 deadline error를
  반환하면 `RunOnce`는 그 error를 반환하고 claimed row를 건드리지 않습니다. Row는
  lease 기반 recovery에 맡기며, shutdown이 retry/dead-letter 상태를 만들면 안
  됩니다.
- Cancellation이 아닌 publish error는 bounded failure text와 함께 `MarkFailed`로
  저장합니다. Row는 `MaxAttempts`에 따라 retryable 또는 dead-letter 상태가
  됩니다.
- Retry 또는 expired claim은 같은 audit envelope를 다시 publish할 수 있습니다.
  Publisher adapter는 안정적인 `Record.EventID`와 `Record.IdempotencyKey` handoff를
  유지하고 downstream idempotency에 의존해야 합니다.
- Adapter는 caller-owned logging, metric, hook을 사용할 수 있지만 이 package는
  global logger를 제공하지 않습니다. Failure text가 operator를 위해 저장되므로
  반환 error는 bounded/redacted여야 합니다.

## Boundaries

- PostgreSQL이 첫 concrete SQL target입니다.
- `Store`는 검증된 audit entry JSON과 event ID, idempotency key, aggregate
  identity, revision, event type, timestamp, schema version, attempt state,
  bounded failure text를 저장합니다.
- Claimed row는 `available_at`을 lease deadline으로 사용하며
  `ClaimOptions.LeaseDuration` 이후 다시 claim될 수 있습니다.
- Publish/failure mark는 반환된 `Record`의 현재 claim attempt가 맞을 때만
  성공하므로 stale worker가 재획득된 row를 덮어쓰지 못합니다.
- 저장된 byte를 package가 제한한 뒤에만 `audit.DecodeEntryJSON`을 사용합니다.
- `CreateSchema`는 명시적이고 선택적입니다. 이 package는 migration을 숨기지
  않습니다.
- Source transaction choreography, PII 정책, redaction, schema migration rollout,
  publisher idempotency, operator replay tooling은 caller 책임입니다.
- Publisher는 external transport topology, authentication, TLS, retention,
  consumer replay, idempotent duplicate handling을 책임집니다.
- Kafka, NATS, Redis Streams, RabbitMQ, Redpanda, Pulsar, direct Redis audit
  storage는 이후 adapter 범위입니다.

## Tests

```bash
go test -count=1 ./audit/sqloutbox
go test -race -count=1 ./audit/sqloutbox
go test -count=1 ./audit/sqloutbox -run 'RelayRunOnce(PublisherContextCancellationDoesNotRetry|RetriesDuplicatePublishWithStableEnvelope|ConcurrentStressPublishesEachRecordOnce)'
```

Relay test는 cancellation-driven worker lifecycle coverage에 `AsyncJobTester`를
사용하고, concurrent `RunOnce` claim/publish coverage에 `GoroutineStressTester`를
사용합니다.
