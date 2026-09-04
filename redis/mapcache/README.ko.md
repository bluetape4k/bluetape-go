# Redis MapCache

[English](README.md)

`github.com/bluetape4k/bluetape-go/redis/mapcache`는 key-per-entry 방식으로
독립적인 Redis value를 저장하는 typed map primitive입니다. Java식 분산
`ConcurrentMap`이나 process-local cache가 아니라 내구성 있는 Redis 저장소입니다.

```go
cache, err := redismap.New(client, redismap.Options[Account]{
    Namespace: "catalog:v1", HashTag: "tenant-a",
    Serializer: serialization.NewJSONSerializer[Account](),
})
if err != nil { return err }
ok, err := cache.CompareAndSet(ctx, "sku:42", old, next, 10*time.Minute)
```

`Client`는 `Set`, `SetNX`, `Del`, `Eval`만 포함한 caller-owned interface입니다.
Client lifecycle, retry, iteration, map 전체 clear, entry 간 transaction,
Redis persistence/eviction/ACL/TLS와 maxmemory 정책은 package가 소유하지 않습니다.

## Key, TTL, operation

Entry key는 `Namespace:map:{optional-hash-tag}:<logical-key>`입니다. Logical key
byte는 그대로 보존하고 structural segment는 검증합니다. Hash tag는 Redis Cluster
same-slot hint일 뿐입니다. Entry마다 TTL이 독립적이며 `0`은 persistent, 음수는
`btredis.ErrInvalidTTL`, 1ms 미만 양수는 1ms입니다. 나머지 양수는 millisecond로
내림합니다.

`Get`, `Set`, `SetIfAbsent`, `GetAndDelete`, `CompareAndSet`, `Delete`는
`redisbucket`과 같은 value/context 계약을 사용합니다. `Get`과 Lua operation은
bounded read를 사용하며 `MaxPayloadBytes` 기본값은 `1 MiB`, 허용 범위는
`1..64 MiB`입니다. Oversized write는 dispatch 전에 거부하고, oversized 기존
value는 `Get`, `GetAndDelete`, CAS에서 `ErrPayloadTooLarge`를 반환하며 decode·교체·
삭제하지 않습니다. Lua operation은 하나의 entry만 atomic하게 처리하므로 다른
entry를 lock하지 않습니다. 이를 위해 Redis ACL에 `GETRANGE`, `EXISTS`, `EVAL`과
`SET`, `SETNX`, `DEL` 권한이 필요합니다.

## 경계와 Error

Value 형식은 caller serializer가 정하며 JSON을 암묵적으로 사용하지 않습니다.
`Error`와 내부 `btredis.OpError`는 raw key, payload, provider text를 가립니다.
`errors.Is`/`errors.As`로 package sentinel, provider cause,
`btredis.ErrCommitUnknown`을 확인하세요. Nil/cancelled context는 dispatch하지
않습니다. Mutation error나 malformed Lua result는 commit-unknown이며
`ErrPayloadTooLarge`는 기존 value를 보존합니다. 성공 response 뒤 cancellation은
context error만 반환하고 재시도·보상하지 않습니다.

`cache.Memory`는 process-local, `cache/redisnear`는 near-cache invalidation,
`cache/rediscoord`는 stampede/loading coordination을 소유합니다. 필요하면 caller가
명시적으로 조합하세요. MapCache는 eviction, invalidation, iteration, stampede 방지를
제공하지 않습니다.

일반 CI는 deep-copy fake를 사용합니다. Integration test는 고정 Redis
Testcontainers fixture로 entry별 expiry, empty/oversized value, concurrent CAS와
bounded Lua operation을 검증하며 다른 Docker test와 직렬 실행하세요.
