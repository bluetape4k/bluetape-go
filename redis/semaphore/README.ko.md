# redis/semaphore

`redis/semaphore`는 bounded Redis semaphore입니다. 각 permit은 Redis sorted set의
정확한 owner-token member이며 설정된 TTL 뒤 만료됩니다.

## 사용법

```go
import redissem "github.com/bluetape4k/bluetape-go/redis/semaphore"

semaphore, err := redissem.New(client, redissem.Options{
    Key:     "limits:partner-api",
    Permits: 8,
    TTL:     30 * time.Second,
})
if err != nil {
    return err
}

lease, err := semaphore.Acquire(ctx)
if err != nil {
    return err
}
defer func() {
    cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()
    _, _ = lease.Release(cleanupCtx)
}()
```

`TryAcquire`는 즉시 한 번만 시도합니다. `Acquire`는 `ErrNotAcquired`만 bounded
backoff으로 재시도하고 caller context 취소 또는 deadline에서 반환합니다. 각
acquire는 capacity를 확인하기 전에 만료된 sorted-set member를 atomic하게
삭제합니다. `Release`는 정확히 일치하는 owner-token member만 삭제하며
idempotent합니다.

## 운영 경계

Semaphore는 fencing token을 발급하지 않습니다. Permit TTL이 만료된 뒤에도 old
holder의 작업이 계속되면 fresh holder와 overlap할 수 있습니다. Critical section을
TTL 안에 끝내거나 외부 resource version/ownership 검증을 적용하세요. 이 primitive는
FIFO/fairness나 Redlock을 제공하지 않고 watchdog renewal도 사용하지 않습니다.
Redis client, context, provider 오류 retry 정책, cleanup context는 caller가
소유합니다.

Provider failure는 sanitized `btredis.OpError`로 반환됩니다. Mutation이 dispatch된
뒤 결과가 불명확하면 `btredis.ErrCommitUnknown`을 보존하므로 permit이 생성되었는지
단정하기 전에 Redis state를 재확인하세요. Error string에는 raw logical key와 owner
token이 포함되지 않습니다.

## 검증

```bash
go test -p 1 -count=1 ./redis/semaphore
go test -p 1 -race -count=1 ./redis/semaphore
go test -count=1 ./redis/semaphore -run Example
```
