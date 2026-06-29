# Audit Example Service

[English](README.md) | [한국어](README.ko.md)

This package is a runnable order-service example for the `audit` package. It
shows command-side aggregate changes, audit repository writes, history queries,
and optional outbox replay without introducing a framework or public helper API.

![Audit Example Service Flow](../../docs/images/readme-diagrams/audit-example-service-flow.png)

## Flow

Think of the example as three small boundaries, not one framework:

1. `OrderService.CreateOrder`, `AddItem`, and `CompleteOrder` validate a
   command and build an `audit.Entry`.
2. The service appends that entry through the injected `audit.Repository`.
   Only after the append succeeds does it mutate the in-memory `Order` source
   model. If the repository fails, the source model is left unchanged.
3. `OrderService.History` reads reconstructed aggregate history through the
   same repository boundary, while `ReplayHistoryToOutbox` copies one
   aggregate history into an `EntrySink`.

`MemoryOutbox` is only a fixture for the replay boundary. Production code can
adapt `EntrySink` to `audit/sqloutbox.Store.Enqueue` inside an
application-owned transaction.

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

Diagram source: [`audit-example-service-flow.svg`](../../docs/images/readme-diagrams/audit-example-service-flow.svg)
