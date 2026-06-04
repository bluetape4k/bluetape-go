# leader/redis

`leader/redis` implements `leader.Elector` and `leader.GroupElector` with
Redis TTL ownership. Use it when only one replica, or a bounded number of
replicas, may run a coordination lane.

## Import

```go
import redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
```

## Usage

```go
elector, err := redisleader.New(client, leader.Options{
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
group, err := redisleader.NewGroup(client, leader.GroupOptions{
    Options: leader.Options{
        Group:    "batch-workers",
        MemberID: "worker-1",
    },
    MaxLeaders: 3,
})
```

## Behavior

- Single leader election stores `memberID:random` in
  `bluetape:leader:<group>` with a Redis TTL.
- Group election stores slot tokens in `bluetape:leader-group:<group>` as ZSET
  members with expiry scores.
- Renewal runs in the background after a successful campaign.
- Duplicate campaign calls on the same elector are rejected.
- Expired group slots are pruned during acquire and status checks.

## Operational Boundaries

- Redis key formats are Go-owned and are not compatible with Kotlin/JVM
  bluetape4k-leader Lettuce or Redisson participants.
- Campaign waits until leadership is acquired or the caller context is
  cancelled.

## Test

```bash
go test -count=1 ./leader/redis
```
