# redis/stream

[English](README.md) | [한국어](README.ko.md)

`github.com/bluetape4k/bluetape-go/redis/stream` provides direct, typed Redis
Streams command helpers. Its package name is `redisstream`.

![Redis Streams consumer lifecycle](../../docs/images/readme-diagrams/redis-streams-consumer-lifecycle.png)

## Scope

This package is not a message broker, consumer framework, or `go-redis` facade.
It owns no connection, retry, logger, metric, background goroutine, payload
encoding, consumer topology, or retention policy. The caller owns the Redis
client and every deadline.

The helpers map directly to explicit Redis commands:

- `Append` (`XADD`)
- `Read` (`XREAD`)
- `CreateGroup` (`XGROUP CREATE ... MKSTREAM`)
- `ReadGroup` (`XREADGROUP`)
- `Acknowledge` (`XACK`)
- `Pending` (`XPENDING` detail)
- `AutoClaim` (`XAUTOCLAIM`)
- `TrimMaxLen`, `TrimMinID`, and `Delete`

Names, IDs, field keys, values, and `go-redis` command options remain
caller-owned. Blank structural names are rejected, but valid names are passed
to Redis verbatim. `Read` and `ReadGroup` use the go-redis stream layout: all
stream keys first, then one ID per stream.

## Append

Use a caller-owned timeout and choose payload encoding at the call site:

```go
ctx, cancel := context.WithTimeout(parent, time.Second)
defer cancel()

id, err := redisstream.Append(ctx, client, redis.XAddArgs{
	Stream: "orders",
	Values: map[string]any{
		"event_id": "evt-42",
		"kind":     "created",
	},
})
```

`XAddArgs.MaxLen`, `MinID`, and related trim settings remain exactly as the
caller supplies them. The package never applies retention implicitly.

## Consumer Groups And Recovery

Redis Streams consumer groups are at-least-once. A process can finish an effect
and fail before `Acknowledge`, or a command can time out after Redis accepted
it. Consumers must make effects idempotent using a stable application identity
such as `event_id`.

```go
if err := redisstream.CreateGroup(ctx, client, "orders", "billing", "0"); err != nil {
	return err // callers decide whether an existing group is acceptable
}

streams, err := redisstream.ReadGroup(ctx, client, redis.XReadGroupArgs{
	Group:    "billing",
	Consumer: "billing-1",
	Streams:  []string{"orders", ">"},
	Count:    10,
})
```

`ReadGroup` creates pending entries unless `NoAck` is set. Acknowledge only
after the idempotent effect is complete. `Acknowledge` removes a pending record
for the group; it does not delete the stream entry.

Use `Pending` to inspect recovery candidates. `AutoClaim` returns messages and
the next Redis cursor; persist or continue that cursor according to your own
recovery policy. The package does not select idle thresholds, steal work, or
retry effects automatically.

When a consumer stops, stop issuing reads, finish or record in-flight
idempotent work, acknowledge only completed effects, and leave unresolved work
pending for controlled recovery.

## Retention And Replay

`TrimMaxLen`, `TrimMinID`, and `Delete` are explicit destructive commands.
They can make history unavailable to slow consumers or incident replay. Choose
retention only after accounting for consumer lag, pending entries, and replay
requirements. `Delete` does not replace acknowledgement; it merely removes the
selected stream entries.

## Errors And Runbook

An already-canceled or expired context is returned directly before dispatch. A
dispatched Redis failure returns `*btredis.OpError`, whose formatted text has a
low-cardinality operation and redacted stream-key correlation ID. It does not
include raw stream names or provider error text. Use `errors.Is` and `errors.As`
to inspect the causal error.

If a context expires after dispatch, command commit state is indeterminate:
inspect stream/group state or retry through an idempotent workflow instead of
assuming no effect occurred.

Operational checks:

- `context.Canceled` / `context.DeadlineExceeded`: caller timeout path; inspect
  Redis state if the command may have reached Redis.
- `*btredis.OpError`: inspect the unwrapped cause and redacted key ID; do not
  reconstruct or log raw keys from error strings.
- Growing pending entries: investigate unhealthy consumers, idempotency, and
  `AutoClaim` policy before increasing retention or deleting history.
- Replay: use explicit `Read`/`ReadGroup` IDs and a bounded, service-owned
  procedure. This package never starts replay by itself.

## Verification

Redis integration tests require Docker:

```bash
go test -p 1 -count=1 ./redis/stream
go test -p 1 -race -count=1 ./redis/stream
go test -count=1 ./redis/stream -run Example
```

No benchmark is run in this issue. Provider benchmark tables, charts, and
written analysis are owned by issue #560.
