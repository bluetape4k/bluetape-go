# Audit Example Service

[English](README.md) | [한국어](README.ko.md)

This package is a runnable order-service example for the `audit` package. It
shows command-side aggregate changes, audit repository writes, history queries,
and optional outbox replay without introducing a framework or public helper API.

## Flow

- `OrderService.CreateOrder`, `AddItem`, and `CompleteOrder` mutate an
  in-memory source model only after an `audit.Entry` is appended through the
  injected `audit.Repository`.
- `OrderService.History` reads reconstructed aggregate history through the
  repository boundary.
- `ReplayHistoryToOutbox` copies one aggregate history into an `EntrySink`.
  `MemoryOutbox` is a fixture; production code can adapt this boundary to
  `audit/sqloutbox.Store.Enqueue` inside an application-owned transaction.

## Non-Goals

- This is not a full event-sourcing framework.
- This is not a JaVers-style object graph diff or shadow reconstruction engine.
- This is not a replacement for a durable source-of-truth database.
- `MemoryOutbox` is not a durable outbox. Use `audit/sqloutbox` when delivery
  must survive process restarts.

## Test

```bash
go test -count=1 ./examples/audit
go test -race -count=1 ./examples/audit
```

## Related Packages

- [`audit`](../../audit/README.md)
- [`audit/sqloutbox`](../../audit/sqloutbox/README.md)
- [`testing/concurrency`](../../testing/concurrency/README.md)
