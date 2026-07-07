# audit/sqloutbox/sqloutboxtest

[English](README.md) | [한국어](README.ko.md)

Deterministic `sqloutbox.Publisher` implementations for tests, local examples,
and workshop adoption.

This package is not a durable transport adapter. It implements the existing
`audit/sqloutbox` publisher contract without adding broker lifecycle, topology,
retention, authentication, replay, or consumer idempotency policy.

## Import

```go
import "github.com/bluetape4k/bluetape-go/audit/sqloutbox/sqloutboxtest"
```

## Publishers

`DiscardPublisher` accepts records without retaining or transporting them. Use
it when a test needs to exercise relay/store paths without asserting published
content.

```go
publisher := sqloutboxtest.DiscardPublisher{}
err := publisher.Publish(ctx, record)
```

`PublisherFunc` adapts a function to `sqloutbox.Publisher`. A nil
`PublisherFunc` returns `ErrNilPublisherFunc`.

```go
publisher := sqloutboxtest.PublisherFunc(func(ctx context.Context, record sqloutbox.Record) error {
	return send(ctx, record)
})
```

`RecordingPublisher` records every publish attempt, returns defensive snapshots,
and can inject deterministic per-event failures. The zero value is ready to use
and concurrent-safe.

```go
publisher := sqloutboxtest.NewRecordingPublisher(
	sqloutboxtest.WithFailures(map[audit.EventID]int{
		"evt-retry": 1,
	}, errors.New("temporary sink failure")),
)
```

## Delivery Semantics

The helpers preserve the `audit/sqloutbox` relay contract:

- Context cancellation or deadline errors are returned without becoming
  retry/dead-letter state.
- Non-cancellation publish errors can be used to drive retry and dead-letter
  tests through `sqloutbox.Relay`.
- `RecordingPublisher` records duplicate attempts in order, so tests can assert
  at-least-once delivery and stable `Record.EventID` /
  `Record.IdempotencyKey` handoff.
- `Reset` clears recorded attempts and configured failure counters.

## Diagrams

No package-local diagram is required. The package adds no new runtime topology
or call sequence beyond the existing `sqloutbox.Publisher` participant. Use the
class contract and relay sequence diagrams in
[`audit/sqloutbox`](../README.md) for the source-backed contract.

## Tests

```bash
go test -count=1 ./audit/sqloutbox/sqloutboxtest
go test -race -count=1 ./audit/sqloutbox/sqloutboxtest
go test -count=1 ./audit/sqloutbox ./audit/sqloutbox/sqloutboxtest
```

The package uses `GoroutineStressTester` for concurrent recording coverage and a
PostgreSQL Testcontainers-backed relay test for retry/dead-letter handoff.
