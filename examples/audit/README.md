# Audit Example Service

[English](README.md) | [한국어](README.ko.md)

This package is a runnable order-service example for the `audit` package. It
shows command-side aggregate changes, audit repository writes, history queries,
optional outbox replay, and the production handoff from durable outbox rows to a
publisher without introducing a framework or public helper API.

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
application-owned transaction. A service-owned relay then calls
`sqloutbox.NewRelay`, polls with `Relay.RunOnce` or `Relay.Run`, and hands each
claimed `sqloutbox.Record` to `sqloutbox.Publisher.Publish`.

## Publisher Adoption

Keep the adoption path explicit:

1. Source writes and `Store.Enqueue` share the caller-owned `*sql.Tx`; the
   package does not hide transaction hooks.
2. `Relay.RunOnce` is for scheduler-owned polling, while `Relay.Run` is for a
   worker lifecycle controlled by `context.Context`.
3. Publisher adapters must preserve `Record.EventID` and
   `Record.IdempotencyKey` so downstream consumers can deduplicate at-least-once
   delivery.
4. Tests and workshops can use `sqloutboxtest.RecordingPublisher` with
   `sqloutboxtest.WithFailures` to prove retry and duplicate-delivery behavior
   before a durable transport adapter exists.

Operator notes:

- Retry is driven by non-cancellation `Publisher.Publish` errors and the
  relay's `MaxAttempts` / `RetryDelay` options.
- Context cancellation and deadlines stop the worker without creating retry or
  dead-letter state.
- Persisted failure text is operator-facing; returned publisher errors must be
  bounded and redacted.
- Duplicate delivery is part of the contract. Consumers should treat
  `Record.EventID` or `Record.IdempotencyKey` as the stable idempotency key.
- Cross-repo workshop coverage is tracked in
  [bluetape-go-workshop#57](https://github.com/bluetape4k/bluetape-go-workshop/issues/57).

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
- [`audit/sqloutbox/sqloutboxtest`](../../audit/sqloutbox/sqloutboxtest/README.md)
- [`testing/concurrency`](../../testing/concurrency/README.md)

Diagram source: [`audit-example-service-flow.svg`](../../docs/images/readme-diagrams/audit-example-service-flow.svg)
