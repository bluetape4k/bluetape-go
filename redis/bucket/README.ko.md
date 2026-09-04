# Redis Bucket

[English](README.md)

`github.com/bluetape4k/bluetape-go/redis/bucket`는 단일 Redis key를 다루는
작은 내구성 있는 primitive입니다. `serialization.Serializer[V]`를 caller가
제공하며 JSON이나 RedisJSON을 가정하지 않습니다.

```go
bucket, err := redisbucket.New(client, redisbucket.Options[Account]{
    Namespace: "catalog:v1", HashTag: "tenant-a",
    Serializer: serialization.NewJSONSerializer[Account](),
})
if err != nil { return err }
ok, err := bucket.SetIfAbsent(ctx, "sku:42", account, 5*time.Minute)
```

Client는 `Set`, `SetNX`, `Del`, `Eval`만 포함한 caller-owned narrow interface입니다.
Package가 client를 생성·종료하거나 retry를 추가하지 않으며,
Redis persistence/eviction/ACL/TLS/maxmemory 정책과 metric도 소유하지 않습니다.
Deadline과 retry 결정은 호출 경계에서 지정하세요.

## Operation

`Get`은 하나의 Lua invocation에서 `GETRANGE`와 `EXISTS`를 실행합니다. Key가
없거나 만료되면 `(zero, false, nil)`을 반환하고, 응답 context를 확인한 뒤 present
payload만 decode합니다. `Set`/`SetIfAbsent`는 `SET`/`SETNX`를 사용합니다.
`GetAndDelete`와 `CompareAndSet`은 단일-key Lua로 read/delete 또는
expected/replacement 결정을 atomic하게 수행합니다. `Delete`는 삭제 count와
무관하게 idempotent 성공입니다.

`MaxPayloadBytes`는 serialized value 상한입니다(기본 `1 MiB`, 허용 범위
`1..64 MiB`). Write는 dispatch 전에 oversized payload를 거부합니다. Bounded
read, `GetAndDelete`, CAS는 기존 value가 상한을 넘으면 `ErrPayloadTooLarge`를
반환하며 decode·교체·삭제하지 않습니다. 이 bounded script를 사용하려면 caller의
Redis ACL에 `GETRANGE`, `EXISTS`, `EVAL`과 `SET`, `SETNX`, `DEL` 권한이 필요합니다.

Key layout은 `Namespace:bucket:{optional-hash-tag}:<logical-key>`입니다. Structural
segment는 공용 Redis `KeyBuilder`로 검증하고 logical key는 space, brace, colon을
포함해 byte 그대로 보존합니다. Hash tag는 Redis Cluster same-slot hint이며 tenant
또는 authorization boundary가 아닙니다.

TTL `0`은 persistent, 음수는 `btredis.ErrInvalidTTL`, 1ms 미만 양수는 1ms로
전달합니다. 나머지 양수는 whole millisecond로 내림합니다. Redis persistence와
eviction은 operator가 설정합니다.

## Error와 cancellation

`Error`와 내부 `btredis.OpError`는 `Error()`/`%+v`에 operation label과 안정적인
redacted key ID만 노출합니다. `errors.Is`/`errors.As`로 `ErrSerialization`,
`ErrInvalidPayload`, `ErrMalformedResult`, `ErrPayloadTooLarge`, provider cause,
`btredis.ErrCommitUnknown`을 확인하세요. Mutation command error나 malformed Lua
result는 Redis가 이미 변경됐을 수 있으므로 commit-unknown입니다. Oversized legacy
value는 caller가 명시적으로 migrate/remove할 수 있도록 보존합니다. 성공 response
뒤 caller cancellation은 context error를 반환하며 보상 write를 하지 않습니다.

Nil 또는 이미 취소된 context는 dispatch 전에 거부합니다. Goroutine이나 cleanup
worker는 소유하지 않습니다. Local/near-cache는 `cache.Memory` 또는
`cache/redisnear`, stampede coordination은 `cache/rediscoord`를 명시적으로 조합하세요.
이 package는 durable Redis entry만 저장합니다.

일반 CI는 mutex-safe fake를 사용합니다. Integration test는 고정 Redis
Testcontainers fixture로 expiry, empty value, oversized legacy value 보존, entry
operation, concurrent CAS와 Lua atomicity를 검증하며 다른 Docker suite와 직렬로
실행하세요.
