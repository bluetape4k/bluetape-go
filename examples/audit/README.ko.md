# Audit Example Service

[English](README.md) | [한국어](README.ko.md)

이 package는 `audit` package를 사용하는 runnable order-service example입니다.
Framework나 public helper API를 추가하지 않고 command-side aggregate 변경,
audit repository write, history query, optional outbox replay를 보여줍니다.

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
`audit/sqloutbox.Store.Enqueue`로 연결하면 됩니다.

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
- [`testing/concurrency`](../../testing/concurrency/README.ko.md)

Diagram source: [`audit-example-service-flow.svg`](../../docs/images/readme-diagrams/audit-example-service-flow.svg)
