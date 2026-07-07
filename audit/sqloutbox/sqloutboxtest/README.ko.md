# audit/sqloutbox/sqloutboxtest

[English](README.md) | [한국어](README.ko.md)

Test, local example, workshop adoption을 위한 deterministic
`sqloutbox.Publisher` 구현체입니다.

이 package는 durable transport adapter가 아닙니다. Broker lifecycle, topology,
retention, authentication, replay, consumer idempotency 정책을 추가하지 않고
기존 `audit/sqloutbox` publisher contract만 구현합니다.

## Import

```go
import "github.com/bluetape4k/bluetape-go/audit/sqloutbox/sqloutboxtest"
```

## Publishers

`DiscardPublisher`는 record를 보관하거나 전송하지 않고 accept합니다. Published
content를 assert하지 않고 relay/store 경로만 exercise하는 test에 사용합니다.

```go
publisher := sqloutboxtest.DiscardPublisher{}
err := publisher.Publish(ctx, record)
```

`PublisherFunc`는 function을 `sqloutbox.Publisher`로 adapter합니다. Nil
`PublisherFunc`는 `ErrNilPublisherFunc`를 반환합니다.

```go
publisher := sqloutboxtest.PublisherFunc(func(ctx context.Context, record sqloutbox.Record) error {
	return send(ctx, record)
})
```

`RecordingPublisher`는 모든 publish attempt를 기록하고 defensive snapshot을
반환하며, event별 deterministic failure를 주입할 수 있습니다. Zero value는 바로
사용 가능하고 concurrent-safe입니다.

```go
publisher := sqloutboxtest.NewRecordingPublisher(
	sqloutboxtest.WithFailures(map[audit.EventID]int{
		"evt-retry": 1,
	}, errors.New("temporary sink failure")),
)
```

## Delivery Semantics

Helper는 `audit/sqloutbox` relay contract를 보존합니다.

- Context cancellation 또는 deadline error는 retry/dead-letter 상태가 되지 않고
  그대로 반환됩니다.
- Cancellation이 아닌 publish error는 `sqloutbox.Relay`를 통한 retry와
  dead-letter test에 사용할 수 있습니다.
- `RecordingPublisher`는 duplicate attempt를 순서대로 기록하므로 at-least-once
  delivery와 안정적인 `Record.EventID` / `Record.IdempotencyKey` handoff를 assert할
  수 있습니다.
- `Reset`은 기록된 attempt와 configured failure counter를 지웁니다.

## Diagrams

Package-local diagram은 추가하지 않습니다. 이 package는 기존
`sqloutbox.Publisher` participant 밖의 runtime topology나 call sequence를 새로
추가하지 않습니다. Source-backed contract는
[`audit/sqloutbox`](../README.ko.md)의 class contract와 relay sequence diagram을
사용합니다.

## Tests

```bash
go test -count=1 ./audit/sqloutbox/sqloutboxtest
go test -race -count=1 ./audit/sqloutbox/sqloutboxtest
go test -count=1 ./audit/sqloutbox ./audit/sqloutbox/sqloutboxtest
```

이 package는 concurrent recording coverage에 `GoroutineStressTester`를 사용하고,
retry/dead-letter handoff에는 PostgreSQL Testcontainers-backed relay test를
사용합니다.
