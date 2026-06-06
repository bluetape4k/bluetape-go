# lock/redis

[English](README.md) | [한국어](README.ko.md)

`lock/redis`는 TTL cleanup과 owner-safe unlock semantic이 필요한 coordination 작업을 위해 작은 single-Redis-instance owner-token lock을 제공합니다.

## Diagram

![Redis lock owner-token lifecycle](../../docs/images/readme-diagrams/redis-lock-owner-token-lifecycle.png)

## 가져오기

```go
import redislock "github.com/bluetape4k/bluetape-go/lock/redis"
```

## 사용 예

```go
mutex, err := redislock.New(client, redislock.Options{
    Key: "locks:billing-rollup",
    TTL: 30 * time.Second,
})
if err != nil {
    return err
}

lease, err := mutex.TryLock(ctx)
if errors.Is(err, redislock.ErrNotAcquired) {
    return nil
}
if err != nil {
    return err
}
defer lease.Unlock(context.Background())
```

## 동작

- `TryLock`은 Redis `SET NX`와 TTL로 non-blocking acquire를 한 번 시도합니다.
- `Lease.Unlock`은 저장된 token이 lease token과 여전히 일치할 때만 Redis key를 삭제합니다.
- `Options.Token`으로 custom token을 제공할 수 있습니다. 제공하지 않으면 acquire마다 random owner token을 생성합니다.
- Redis command의 context cancellation은 보존됩니다.

## 운영 경계

- 이 구현은 Redlock quorum이 아니며 fencing token을 제공하지 않습니다.
- TTL renewal과 blocking retry loop는 의도적으로 포함하지 않았습니다.
- Protected operation을 안전하게 덮는 TTL을 선택하거나 higher layer에서 renewal을 조합하세요.

## 테스트

```bash
go test -count=1 ./lock/redis
```
