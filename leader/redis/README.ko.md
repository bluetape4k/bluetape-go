# leader/redis

[English](README.md) | [한국어](README.ko.md)

`leader/redis`는 Redis TTL ownership으로 `leader.Elector`, `leader.GroupElector`,
`leader.StrategicElector`를 구현합니다. 하나의 replica, 제한된 수의 replica, 또는
strategy로 선출된 하나의 candidate만 coordination lane을 실행해야 할 때 사용합니다.

## 다이어그램

![Redis leader election lifecycle](../../docs/images/readme-diagrams/redis-leader-election-lifecycle.png)

## 가져오기

```go
import redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
```

## 사용 예

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

최대 `MaxLeaders`개의 worker가 동시에 실행될 수 있으면 `NewGroup`을 사용합니다.

```go
group, err := redisleader.NewGroup(client, leader.GroupOptions{
    Options: leader.Options{
        Group:    "batch-workers",
        MemberID: "worker-1",
    },
    MaxLeaders: 3,
})
```

Candidate가 lock 경쟁 대신 metadata로 ranking되어야 하면 `NewStrategic`을 사용합니다.

```go
elector, err := redisleader.NewStrategic[string](client, leader.Options{
    Group:    "nightly-jobs",
    MemberID: "worker-1",
})
if err != nil {
    return err
}

err = elector.RegisterCandidate(ctx, "nightly-jobs", leader.CandidateInfo{
    NodeID: "worker-1",
}, 30*time.Second)
if err != nil {
    return err
}

strategy := leader.ScoredStrategy{Scorer: leader.IdleTimeScorer{}}
result, ran, err := elector.RunIfLeader(ctx, "nightly-jobs", strategy, func(context.Context) (string, error) {
    return "report-created", nil
})
_ = result
_ = ran
```

## 동작

- Single leader election은 `memberID:random` 값을 Redis TTL이 있는 `bluetape:leader:<group>`에 저장합니다.
- Group election은 expiry score를 가진 ZSET member로 slot token을 `bluetape:leader-group:<group>`에 저장합니다.
- Strategic election은 candidate JSON value를 `bluetape:leader-strategy:<group>:candidates:<nodeID>` 아래에 저장하고 live candidate를 `bluetape:leader-strategy:<group>:index`에서 추적합니다.
- 성공적인 campaign 이후 renewal은 background에서 실행됩니다.
- 같은 elector에서 duplicate campaign call은 거부됩니다.
- Expired group slot은 acquire/status check 중 prune됩니다.
- Expired strategic candidate는 list operation 중 prune됩니다.

## 운영 경계

- Redis key format은 Go-owned이며 Kotlin/JVM bluetape4k-leader Lettuce 또는 Redisson participant와 호환되지 않습니다.
- Campaign은 leadership을 획득하거나 caller context가 cancel될 때까지 대기합니다.

## 실행 가능한 Batch 예제

`coordination_example_test.go`는 `leader/redis`와 `batch` 패키지를 연결합니다.

- `TestBatchSchedulerExample`은 현재 leader만 scheduled batch job을 실행하게 합니다.
- `TestMigrationGateExample`은 현재 leader만 migration batch를 실행하고, leadership이 바뀐 뒤 이미 적용된 migration은 skip합니다.

예제는 `testcontainers/redis`를 사용하므로 Docker 또는 Testcontainers-compatible runtime이 필요합니다.

```bash
go test -count=1 ./leader/redis -run 'Test(BatchSchedulerExample|MigrationGateExample)'
```

## 테스트

```bash
go test -count=1 ./leader/redis
```
