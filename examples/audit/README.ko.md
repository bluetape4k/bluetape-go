# Audit Example Service

[English](README.md) | [한국어](README.ko.md)

이 package는 `audit` package를 사용하는 runnable order-service example입니다.
Framework나 public helper API를 추가하지 않고 command-side aggregate 변경,
audit repository write, history query, optional outbox replay를 보여줍니다.

## Flow

- `OrderService.CreateOrder`, `AddItem`, `CompleteOrder`는 주입된
  `audit.Repository`에 `audit.Entry`를 append한 뒤 in-memory source model을
  변경합니다.
- `OrderService.History`는 repository boundary를 통해 aggregate history를
  재구성해 읽습니다.
- `ReplayHistoryToOutbox`는 한 aggregate history를 `EntrySink`로 복사합니다.
  `MemoryOutbox`는 fixture입니다. 운영 code는 application-owned transaction 안에서
  이 boundary를 `audit/sqloutbox.Store.Enqueue`로 바꿀 수 있습니다.

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
