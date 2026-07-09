# leader/mongo

[English](README.md) | [한국어](README.ko.md)

`leader/mongo` provides MongoDB-backed implementations of `leader.Elector`,
`leader.GroupElector`, and `leader.StrategicElector`. The single elector uses
one lease document per leader key. The group elector uses one lease document per
bounded slot, so MongoDB single-document atomicity enforces the exact
`MaxLeaders` cap under concurrent acquisition. The strategic elector stores one
leased candidate document per node and lets the caller choose FIFO, random, or
scored strategy execution.

## Diagram

![MongoDB leader election runtime map](../../docs/images/readme-diagrams/mongo-leader-election-lifecycle.png)

![MongoDB leader election sequence](../../docs/images/readme-diagrams/mongo-leader-election-sequence.png)

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

Use `NewGroup` when up to `MaxLeaders` workers may run concurrently:

```go
group, err := mongoleader.NewGroup(collection, leader.GroupOptions{
    Options: leader.Options{
        Group:         "billing-workers",
        MemberID:      "worker-1",
        Lease:         30 * time.Second,
        RenewInterval: 10 * time.Second,
    },
    MaxLeaders: 3,
})
if err != nil {
    return err
}
if err := group.Campaign(ctx); err != nil {
    return err
}
defer group.Resign(context.Background())
```

Use `NewStrategic` when candidates should be ranked by metadata instead of
competing for a lock:

```go
strategic, err := mongoleader.NewStrategic[string](collection, leader.Options{
    Group:    "nightly-jobs",
    MemberID: "worker-1",
})
if err != nil {
    return err
}

err = strategic.RegisterCandidate(ctx, "nightly-jobs", leader.CandidateInfo{
    NodeID:   "worker-1",
    Weight:   10,
    Metadata: map[string]string{"zone": "a"},
}, 30*time.Second)
if err != nil {
    return err
}

strategy := leader.ScoredStrategy{Scorer: leader.WeightScorer{}}
result, ran, err := strategic.RunIfLeader(ctx, "nightly-jobs", strategy, func(context.Context) (string, error) {
    return "report-created", nil
})
_ = result
_ = ran
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

Each group elector stores one document per slot:

| Field | Purpose |
|---|---|
| `_id` | Normalized slot key, `<keyPrefix>:<group>:slot:<slot>`. |
| `group_key` | Shared group key for counting active slots. |
| `group` | Leader group name. |
| `slot` | Zero-based slot number in `[0, MaxLeaders)`. |
| `member_id` | Current slot owner member ID. |
| `token` | Opaque owner token for this slot. |
| `lease_until` | Authoritative slot lease expiry. |
| `created_at` / `updated_at` | Diagnostic timestamps. |

`NewGroup` only acquires from the bounded slot set and renews/deletes by owner
token. `ActiveCount` counts documents with the same `group_key` and
`lease_until > now`; `AvailableSlots` clamps negative values to zero so lowering
`MaxLeaders` does not over-admit new owners while older active slots drain.

Each strategic elector stores one document per candidate:

| Field | Purpose |
|---|---|
| `_id` | Normalized candidate key, `<keyPrefix>:<group>:candidate:<nodeID>`. |
| `group_key` | Shared group key for live-candidate scans. |
| `group` | Strategy group name. |
| `node_id` | Candidate node ID. |
| `registered_at` | Strategy ordering timestamp supplied by the caller or defaulted at registration. |
| `last_started_at` / `last_completed_at` | Diagnostic action timestamps. |
| `success_count` / `failure_count` | Atomic action outcome counters. |
| `weight` / `metadata` | Strategy input copied from `leader.CandidateInfo`. |
| `lease_until` | Authoritative candidate expiry. |
| `created_at` / `updated_at` | Diagnostic timestamps. |

`ListCandidates` prunes expired candidate documents before returning live
documents sorted by `node_id`. `UpdateResult` updates counters only while the
candidate lease is live and returns `leader.ErrNotLeader` for missing or expired
candidates.

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
- `GroupElector` guarantees at most `MaxLeaders` live local owners for a group
  when contenders use the same key prefix, group name, collection, and bounded
  clock-skew assumptions.
- `StrategicElector` provides a live candidate registry and result counters; it
  is not a distributed mutex and relies on all contenders using the same
  strategy, group, key prefix, collection, and clock-skew assumptions.
- `EnsureIndexes` also creates a `group_key, lease_until` index used by group
  active-slot checks and strategic live-candidate scans.

## Test

```bash
go test -count=1 ./leader ./leader/mongo
go test -race -count=1 ./leader ./leader/mongo
go test -p 1 -count=1 ./leader/mongo ./testcontainers/mongodb
```
