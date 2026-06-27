# audit

[English](README.md) | [한국어](README.ko.md)

Storage-neutral aggregate event and audit model values for service packages.

The package defines validated aggregate IDs, positive revisions, caller-owned
event IDs, idempotency keys, audit entries, snapshot/change metadata, a
goroutine-safe pending-event recorder, and deterministic history reconstruction.
It does not persist records, publish outbox messages, or compute JaVers-style
object diffs.

## Import

```go
import "github.com/bluetape4k/bluetape-go/audit"
```

## Minimal Usage

```go
aggregate, err := audit.NewAggregateID("account", "42")
if err != nil {
	return err
}

recorder, err := audit.NewAggregateRecorder(aggregate)
if err != nil {
	return err
}

event, err := recorder.Record(audit.EventRecord{
	EventID:        audit.EventID("evt-001"),
	EventType:      audit.EventType("AccountDebited"),
	OccurredAt:     time.Now().UTC(),
	IdempotencyKey: "command-001",
	Payload:        json.RawMessage(`{"amount":"10.00"}`),
})
if err != nil {
	return err
}

entry, err := audit.NewEntry(audit.EntryOptions{
	Author: "billing-service",
	Event:  event,
})
if err != nil {
	return err
}
```

## Validated Decode

Use `DecodeEntryJSON` for already-bounded JSON bytes from trusted storage
or transport layers. Repository and outbox adapters must enforce byte/depth
limits before reading untrusted input.

```go
entry, err := audit.DecodeEntryJSON(data)
if err != nil {
	return err
}
```

Direct `json.Unmarshal` into `Entry` also validates the entry, nested
event, aggregate ID, snapshot metadata, and change metadata. Unsupported schema
versions are rejected.

## Recorder Handoff

`PendingEvents` returns a defensive snapshot and does not clear events. Call
`AckThrough` only after the source write and durable audit commit both succeed.

```go
pending := recorder.PendingEvents()
if len(pending) == 0 {
	return nil
}
if err := writeSource(ctx, aggregate, pending); err != nil {
	return err
}
if err := writeAudit(ctx, pending); err != nil {
	// The events remain pending and can be read again for retry.
	return err
}
return recorder.AckThrough(pending[len(pending)-1].Revision)
```

This keeps a failure-then-success retry path explicit: failed persistence leaves
events pending; successful durable audit commit advances the acknowledgment
boundary.

This is only an in-process retry pattern. Source writes and audit commits must
be transactionally coupled, rolled back on audit failure, or recovered through a
durable outbox/reconciliation mechanism. In-memory pending events are not crash
recovery.

## JaVers Migration Notes

| JaVers/Kotlin concept | Go package shape | Boundary |
|---|---|---|
| Aggregate root identity | `AggregateID` | Caller domain objects do not implement a package-owned root interface. |
| Domain event | `DomainEvent` | Event IDs and idempotency keys are caller supplied. |
| Audit entry/snapshot | `Entry`, `SnapshotMetadata`, `ChangeMetadata` | JSON validation is local; storage schema is external. |
| Event recording | `AggregateRecorder` | Pending events clear only after explicit ack. |
| Repository event publishing | Later repository/outbox issues | #56 does not own SQL, Redis, Kafka, NATS, or transaction choreography. |
| Object diffing | Out of scope | Callers may store change metadata, but this package does not diff objects. |

## Boundaries

- Revisions are positive and start at `InitialRevision()`.
- `NewHistory` rejects empty, mixed-aggregate, duplicated, non-contiguous, or
  non-initial histories.
- Constructors and JSON decode paths copy metadata and payloads before
  returning values.
- Callers own redaction, PII policy, payload size limits, and persistence
  transaction boundaries.
- Repository interfaces, history query APIs, outbox publishers, SQL DDL, Redis,
  Kafka, NATS, and examples are tracked by later `0.9.0` issues.

## Tests

```bash
go test -count=1 ./audit
go test -race -count=1 ./audit
go test -run '^$' -bench 'BenchmarkAggregateRecorder(Record|PendingEvents|AckThrough)' -benchmem ./audit
```
