# leader/mongo

[English](README.md) | [한국어](README.ko.md)

`leader/mongo` provides a MongoDB-backed implementation of the single
`leader.Elector` contract. It uses one lease document per leader key and
owner-token predicates for acquire, renew, release, and observation.

This package does not implement `leader.GroupElector` or
`leader.StrategicElector`.

## Import

```go
import mongoleader "github.com/bluetape4k/bluetape-go/leader/mongo"
```

## Usage

```go
collection := client.Database("coordination").Collection("leader_leases")
if err := mongoleader.EnsureIndexes(ctx, collection); err != nil {
    return err
}

elector, err := mongoleader.New(collection, leader.Options{
    Group:         "billing-workers",
    MemberID:      "worker-1",
    Lease:         30 * time.Second,
    RenewInterval: 10 * time.Second,
})
if err != nil {
    return err
}
if err := elector.Campaign(ctx); err != nil {
    return err
}
defer elector.Resign(context.Background())
```

## Storage Contract

Each leader group stores one document keyed by `_id`:

| Field | Purpose |
|---|---|
| `_id` | Normalized leader key, `<keyPrefix>:<group>`. |
| `group` | Leader group name. |
| `member_id` | Current owner member ID. |
| `token` | Opaque owner token returned by `Leader`. |
| `lease_until` | Authoritative lease expiry. |
| `created_at` / `updated_at` | Diagnostic timestamps. |

`EnsureIndexes` creates a TTL index on `lease_until` for cleanup only. MongoDB's
TTL monitor is asynchronous, so leadership correctness never depends on the TTL
delete timing. `Campaign` and `Leader` use normal `lease_until` predicates.

## Operational Boundaries

- The MongoDB client, database, collection, indexes, write concern, and cleanup
  are caller-owned.
- Production deployments should configure an appropriate write concern, commonly
  majority, on the caller-owned client or collection.
- The first implementation computes `lease_until` from the process clock.
  Keep clocks synchronized across contenders and choose leases larger than
  expected clock skew plus MongoDB operation latency.
- `Campaign(ctx)` waits until leadership is acquired or the context is canceled.
- Failed renewal, token replacement, or backend renewal errors make `IsLeader`
  false.

## Test

```bash
go test -count=1 ./leader ./leader/mongo
go test -race -count=1 ./leader ./leader/mongo
go test -p 1 -count=1 ./leader/mongo ./testcontainers/mongodb
```

