# redis/lock

`redis/lock`는 단일 Redis 인스턴스용 fenced lock입니다. 성공한 acquire마다
영속적이고 단조 증가하는 fencing token을 발급합니다.

## 사용법

```go
import redislock "github.com/bluetape4k/bluetape-go/redis/lock"

lock, err := redislock.New(client, redislock.Options{
    Key: "locks:billing-rollup",
    TTL: 30 * time.Second,
})
if err != nil {
    return err
}

lease, err := lock.Acquire(ctx)
if err != nil {
    return err
}
defer func() {
    cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()
    _, _ = lease.Release(cleanupCtx)
}()

fencingToken := lease.FencingToken()
```

`TryAcquire`는 즉시 한 번만 시도합니다. `Acquire`는 `ErrNotAcquired`만 bounded
backoff으로 재시도하고 caller context가 취소되거나 deadline에 도달하면
반환합니다. `Release`는 owner-safe하고 idempotent합니다. 만료되었거나 교체된
lease는 `(false, nil)`을 반환합니다.

## Fencing 계약

`FencingToken`은 외부 resource가 가장 큰 token을 저장하고 이전 token을 거부할
때만 외부 resource를 보호합니다. Lock 자체는 TTL 이후 계속 실행되는 작업을
중단시키지 않습니다. Stale holder와 fresh holder가 overlap할 수 있으므로
critical section을 포함하는 TTL을 선택하고 resource boundary에서 token 순서를
검증해야 합니다.

## 오류와 cleanup

Provider failure는 원인을 `errors.Is`/`errors.As`로 확인할 수 있는 sanitized
`btredis.OpError`로 반환합니다. Redis dispatch 이후 mutation 오류에는
`btredis.ErrCommitUnknown`도 포함될 수 있으므로 acquire/release commit 여부를
단정하기 전에 Redis 상태를 확인하세요. Request context가 취소된 뒤에는 별도의
bounded cleanup context를 사용하세요.

Caller-owned logical key는 Redis storage, cluster hash tag, diagnostic에서 hash됩니다.
Provider error string에는 raw key와 owner token이 포함되지 않습니다. 이 primitive는
Redlock이나 FIFO fairness를 제공하지 않으며 watchdog renewal도 의도적으로
포함하지 않습니다.

## 검증

```bash
go test -p 1 -count=1 ./redis/lock
go test -p 1 -race -count=1 ./redis/lock
go test -count=1 ./redis/lock -run Example
```
