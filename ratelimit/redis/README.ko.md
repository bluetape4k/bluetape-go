# ratelimit/redis

[English](README.md) | [한국어](README.ko.md)

`ratelimit/redis`는 여러 process가 공유할 수 있는 Redis-backed token-bucket
limiter를 제공합니다. 각 `Allow` 호출은 하나의 Redis Lua script로 refill,
consume, bucket state 저장, key expiration 갱신을 atomically 수행합니다.

## 다이어그램

![Redis token-bucket rate limit flow](../../docs/images/readme-diagrams/redis-ratelimit-token-bucket-flow.png)

![Redis token-bucket sequence](../../docs/images/readme-diagrams/redis-ratelimit-token-bucket-sequence.png)

## 설치

```go
import redisratelimit "github.com/bluetape4k/bluetape-go/ratelimit/redis"
```

## 사용 예

```go
limiter, err := redisratelimit.New(redisratelimit.Options{
    Client:        redisClient,
    Namespace:     "api",
    RatePerSecond: 100,
    Burst:         200,
})
if err != nil {
    return err
}

result, err := limiter.Allow(ctx, "tenant:blue", 1)
if err != nil {
    return err
}
if !result.Allowed {
    return fmt.Errorf("retry after %s", result.RetryAfter)
}
```

Redis client는 caller-owned입니다. 이 패키지는 Redis connection을 만들거나 닫지
않습니다.

## Redis 상태

기본 key shape:

```text
bluetape:ratelimit:<namespace>:bucket:<key>
```

Bucket key는 Redis hash입니다.

- `tokens`: 남은 microtoken;
- `updated_ms`: 마지막 refill을 위한 Redis server timestamp.

Script는 Redis `TIME`을 사용하므로 distributed caller는 local machine clock에
의존하지 않습니다. `PEXPIRE`는 inactive bucket key를 `IdleTTL`로 bounded하게
유지합니다.

## 운영 경계

- 하나의 key에 대한 concurrent client는 Redis script execution으로 serialize됩니다.
- Rejected attempt는 error가 아니라 정상 `ratelimit.Result` value입니다.
- Redis command/script failure는 error로 반환됩니다.
- Redis command/script failure는 `errors.Is`로 원래 cause를 유지하고,
  `errors.As`를 통한 typed diagnostics를 제공하며 raw Redis key와 provider
  detail은 redaction합니다.
- FIFO fairness, waiting, reservation, adaptive limit, Redis Cluster multi-key
  behavior는 제공하지 않습니다.
- `MaxKeyBytes`는 untrusted logical key 길이를 제한합니다. 기본값은 512 bytes입니다.
- `Burst`보다 많은 token 요청은 validation error입니다.

## 테스트

Redis test는 Testcontainers를 사용하므로 Docker가 필요합니다.

```bash
go test -count=1 ./ratelimit/redis
go test -race -count=1 ./ratelimit/redis
```

Coverage는 burst/rejection, refill, namespace isolation, idle key expiration,
context cancellation, `GoroutineStressTester` 기반 concurrent-client stress를
포함합니다.

## Conformance 및 Commit-Unknown 복구

Provider는 Lua debit boundary에서 `ratelimittest.Run`을 실행합니다. Eval 응답 유실은
zero `Result`와 `redis.ErrCommitUnknown`을 match하는 typed `redis.OpError`를 반환하며 한 번
debit됐을 수 있습니다. Replay하지 말고 최소 `Burst / RatePerSecond`를 기다리거나 caller
budget에 한 번의 debit을 반영합니다.
