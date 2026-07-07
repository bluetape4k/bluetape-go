# Audit Example Service

[English](README.md) | [한국어](README.ko.md)

이 package는 `audit` package를 사용하는 runnable order-service example입니다.
Framework나 public helper API를 추가하지 않고 command-side aggregate 변경,
audit repository write, history query, optional outbox replay, durable outbox
row에서 publisher로 이어지는 운영 handoff를 보여줍니다.

![Audit Example Service Flow](../../docs/images/readme-diagrams/audit-example-service-flow.png)

## Flow

이 예제는 framework가 아니라 세 개의 경계를 보여줍니다.

1. `OrderService.CreateOrder`, `AddItem`, `CompleteOrder`가 command를 검증하고
   `audit.Entry`를 만듭니다.
2. Service는 주입된 `audit.Repository`에 entry를 append합니다. Append가 성공한
   뒤에만 in-memory `Order` source model을 바꿉니다. Repository write가 실패하면
   source model은 그대로 둡니다.
3. `OrderService.History`는 같은 repository boundary로 aggregate history를
   재구성해 읽고, `ReplayHistoryToOutbox`는 한 aggregate history를 `EntrySink`로
   복사합니다.

`MemoryOutbox`는 replay boundary를 테스트하기 위한 fixture입니다. 운영 code에서는
application-owned transaction 안에서 `EntrySink`를
`audit/sqloutbox.Store.Enqueue`로 연결하면 됩니다. Service-owned relay는
`sqloutbox.NewRelay`를 만들고 `Relay.RunOnce` 또는 `Relay.Run`으로 polling한 뒤,
claim된 `sqloutbox.Record`를 `sqloutbox.Publisher.Publish`에 넘깁니다.

## Publisher Adoption

Adoption path는 명시적으로 유지합니다.

1. Source write와 `Store.Enqueue`는 caller-owned `*sql.Tx`를 공유합니다. Package는
   transaction hook을 숨기지 않습니다.
2. `Relay.RunOnce`는 scheduler-owned polling에, `Relay.Run`은 `context.Context`로
   제어되는 worker lifecycle에 사용합니다.
3. Publisher adapter는 `Record.EventID`와 `Record.IdempotencyKey`를 보존해야
   downstream consumer가 at-least-once delivery를 deduplicate할 수 있습니다.
4. Durable transport adapter가 없을 때도 test와 workshop에서는
   `sqloutboxtest.RecordingPublisher`와 `sqloutboxtest.WithFailures`로 retry와
   duplicate-delivery behavior를 먼저 증명할 수 있습니다.

Operator notes:

- Retry는 cancellation이 아닌 `Publisher.Publish` error와 relay의 `MaxAttempts` /
  `RetryDelay` option으로 제어됩니다.
- Context cancellation과 deadline은 retry/dead-letter 상태를 만들지 않고 worker를
  멈춥니다.
- Persisted failure text는 operator-facing입니다. Publisher가 반환하는 error는
  bounded/redacted여야 합니다.
- Duplicate delivery는 contract의 일부입니다. Consumer는 `Record.EventID` 또는
  `Record.IdempotencyKey`를 stable idempotency key로 취급해야 합니다.
- Cross-repo workshop coverage는
  [bluetape-go-workshop#57](https://github.com/bluetape4k/bluetape-go-workshop/issues/57)
  에서 추적합니다.

## Non-Goals

- Full event-sourcing framework가 아닙니다.
- JaVers-style object graph diff나 shadow reconstruction engine이 아닙니다.
- Durable source-of-truth database를 대체하지 않습니다.
- `MemoryOutbox`는 durable outbox가 아닙니다. Process restart 이후 delivery가
  살아야 한다면 `audit/sqloutbox`를 사용하세요.

## Test

```bash
go test -count=1 ./examples/audit
go test -race -count=1 ./examples/audit
```

## Related Packages

- [`audit`](../../audit/README.ko.md)
- [`audit/sqloutbox`](../../audit/sqloutbox/README.ko.md)
- [`audit/sqloutbox/sqloutboxtest`](../../audit/sqloutbox/sqloutboxtest/README.ko.md)
- [`testing/concurrency`](../../testing/concurrency/README.ko.md)

Diagram source: [`audit-example-service-flow.svg`](../../docs/images/readme-diagrams/audit-example-service-flow.svg)
