# audit/sqloutbox/redisstreams

[English](README.md) | [한국어](README.ko.md)

`audit/sqloutbox/redisstreams` publishes claimed `sqloutbox.Record` values to
Redis Streams with one `XADD` per publish attempt.

## Import

```go
import redisstreams "github.com/bluetape4k/bluetape-go/audit/sqloutbox/redisstreams"
```

## Usage

The package includes a compile-checked `ExampleNew` for the constructor. A relay
usually wires the publisher into `audit/sqloutbox` like this:

```go
publisher, err := redisstreams.New(redisstreams.Options{
	Client: redisClient,
	Stream: "audit:sqloutbox",
})
if err != nil {
	return err
}

relay, err := sqloutbox.NewRelay(store, publisher, sqloutbox.RelayOptions{
	ClaimLimit:  10,
	MaxAttempts: 3,
	RetryDelay:  time.Second,
})
```

The Redis client is caller-owned. This package does not create or close Redis
connections.

## Stream Fields

Each Redis stream entry contains the stable sqloutbox envelope:

- `record_id`: decimal string form of `sqloutbox.Record.ID`
- `status`: sqloutbox record status at publish time
- `aggregate_type`: aggregate type
- `aggregate_id`: aggregate instance ID
- `revision`: decimal string form of the audit revision
- `event_id`: stable audit event ID
- `idempotency_key`: stable idempotency key
- `event_type`: audit event type
- `occurred_at`: UTC RFC3339Nano timestamp
- `recorded_at`: UTC RFC3339Nano timestamp
- `schema_version`: decimal string form of the audit schema version
- `attempts`: decimal string form of the sqloutbox claim attempt count
- `entry_json`: full marshaled `audit.Entry`

`entry_json` is the full audit entry JSON already stored by sqloutbox. Redact or
classify sensitive payloads before enqueue/publish when Redis readers or
retention differ from the SQL outbox.

Retries append another stream entry. Consumers should deduplicate with
`event_id` or `idempotency_key` and treat `attempts` as diagnostic metadata.
Redis/client failures can be ambiguous after the server accepted `XADD`, so
operator replay and consumer processing must tolerate duplicate entries with
the same stable event identity.

## Operational Boundaries

- The package only calls Redis `XADD`.
- `Stream` is trusted topology configuration, not request or tenant input. The
  package preserves the exact caller-provided Redis key except for rejecting
  empty/blank values.
- Stream retention, trimming, consumer groups, pending-entry recovery, replay,
  authentication, TLS, and Redis Cluster topology remain caller-owned.
- Redis Streams are a publish transport for outbox records, not the durable
  audit source of truth.
- Context cancellation is checked before `XADD` and Redis command errors are
  wrapped so `Relay.RunOnce` can retry or dead-letter through the normal
  sqloutbox contract. Relay failure text can persist Redis error details, so
  wrap or redact client errors before this adapter when your deployment treats
  Redis diagnostics as sensitive.

## Tests

Redis and relay integration tests use Testcontainers and require Docker:

```bash
go test -count=1 ./audit/sqloutbox/redisstreams
go test -race -count=1 ./audit/sqloutbox/redisstreams
```
