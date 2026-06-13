# jwt

[English](README.md) | [한국어](README.ko.md)

`jwt`는 명시적 algorithm과 repo-owned error를 사용하는 JSON Web Token 생성,
검증, claim 읽기, local key rotation helper를 제공합니다.

## Import

```go
import (
    "context"
    "errors"
    "time"

    "github.com/bluetape4k/bluetape-go/cache"
    "github.com/bluetape4k/bluetape-go/jwt"
    redisjwt "github.com/bluetape4k/bluetape-go/jwt/redis"
    "github.com/redis/go-redis/v9"
)
```

## 선택 가이드

| 필요 | 사용 | 메모 |
|---|---|---|
| 고정 symmetric signing key | `NewFixedHMACProvider` | HS256은 최소 32-byte secret, HS384는 48-byte, HS512는 64-byte가 필요합니다. |
| 고정 asymmetric signing key | `NewFixedRSAProvider` | 검증된 2048-bit 이상 RSA private key로 RS256/384/512, PS256/384/512를 지원합니다. |
| local in-memory key rotation | `NewHMACProvider` 또는 `NewRSAProvider` | in-memory KeyChain repository, `kid` header, TTL, retained key를 사용합니다. |
| Redis-backed distributed key rotation | `jwt/redis.New`와 `NewDistributedHMACProvider` 또는 `NewDistributedRSAProvider` | context-aware Redis I/O로 여러 process instance가 signing key를 공유합니다. |
| MongoDB-backed distributed key rotation | Backlog | MongoDB storage는 #198로 이관했습니다. |
| signed JWT compression | Non-goal | `zip`은 signed JWT helper가 아니라 JWE 경계에 속합니다. |
| JWE, JWK, JWKS | Deferred | 실제 사용 사례가 생기면 JWE/JWKS를 명시적인 optional JOSE 경계로 추가할 수 있습니다. |
| provider cache adapter | `NewCachedProvider` 또는 `NewCachedDistributedProvider` | optional trusted `cache.Cache[string,*jwt.Reader]` wrapper로 provider validation을 우회하지 않고 반복 parse/signature verification 비용을 줄입니다. |

## 사용법

```go
provider, err := jwt.NewFixedHMACProvider(
    jwt.HS256,
    []byte("0123456789abcdef0123456789abcdef"),
)
if err != nil {
    return err
}

token, err := provider.Compose(
    jwt.WithSubject("account-42"),
    jwt.WithAudience("api"),
    jwt.WithExpiresAfter(time.Hour),
    jwt.WithClaim("role", "admin"),
)
if err != nil {
    return err
}

reader, err := provider.Parse(
    token,
    jwt.WithExpectedSubject("account-42"),
    jwt.WithExpectedAudience("api"),
    jwt.WithExpirationRequired(),
)
if err != nil {
    return err
}
role, ok := reader.ClaimString("role")
```

## Provider Cache Adapter

같은 token을 반복 parse하는 경로가 hot path라면 local provider에는
`NewCachedProvider`, distributed provider에는 `NewCachedDistributedProvider`를
사용합니다. Adapter는 성공한 `*jwt.Reader` 결과만 trusted cache backend에
저장하고, token digest, parse profile, provider algorithm, key prefix, trust
scope로 cache key를 만듭니다. Warm hit도 반환 전에 wrapped provider의 현재 key
상태로 다시 검증합니다.

![JWT provider cache adapter flow](../docs/images/readme-diagrams/jwt-provider-cache-adapter-flow.png)

```go
provider, err := jwt.NewFixedHMACProvider(
    jwt.HS256,
    []byte("0123456789abcdef0123456789abcdef"),
)
if err != nil {
    return err
}
readerCache := cache.NewMemory[string, *jwt.Reader]()
cached, err := jwt.NewCachedProvider(provider, readerCache)
if err != nil {
    return err
}

token, err := cached.Compose(
    jwt.WithSubject("account-42"),
    jwt.WithExpiresAfter(time.Hour),
)
if err != nil {
    return err
}
reader, err := cached.Parse(token, jwt.WithExpectedSubject("account-42"))
if err != nil {
    return err
}
```

Distributed provider는 같은 cache contract를 쓰되 repository I/O에 대한 명시적
context를 유지합니다.

```go
opCtx := context.Background()
repo, err := redisjwt.New(redisjwt.Options{
    Client:    redisClient,
    Namespace: "service-auth",
})
if err != nil {
    return err
}
provider, err := jwt.NewDistributedHMACProvider(opCtx, repo, jwt.HS256)
if err != nil {
    return err
}
token, err := provider.ComposeContext(opCtx,
    jwt.WithSubject("account-42"),
    jwt.WithExpiresAfter(time.Hour),
)
if err != nil {
    return err
}
readerCache := cache.NewMemory[string, *jwt.Reader]()
cached, err := jwt.NewCachedDistributedProvider(provider, readerCache)
if err != nil {
    return err
}
reader, err := cached.ParseContext(opCtx, token, jwt.WithExpectedSubject("account-42"))
```

Cache adapter는 성능 helper일 뿐입니다. Auth middleware, session storage, token
revocation, authorization policy, JWKS, 외부 trust service가 아닙니다.
`*jwt.Reader` 값은 이미 검증된 token 결과이므로 trusted application-process cache
backend만 사용하세요. Untrusted shared/external cache backend는 이번 범위에서
지원하지 않습니다.

`WithParseClock`을 쓰는 parse는 cache를 우회합니다. Cache hit도 cached Reader가
nil이 아닌지, algorithm이 일치하는지, `kid`가 live key로 해석되는지, key
algorithm이 여전히 일치하는지 확인합니다. Adapter를 통해 실행한 `ForcedRotate`,
`ForcedRotateContext`, `DeleteKeyChainsContext`는 wrapped operation 성공 후 설정된
cache를 clear합니다. `ClearCache` 범위는 supplied cache backend에 한정됩니다.
`cache.Memory`라면 process-local state만 지웁니다.

운영 메모:

- `WithCacheTrustScope`는 private provider/tenant/key namespace로 다룹니다.
  tenant, algorithm, key store 사이에서 재사용하지 마세요. 기본 scope는 adapter
  construction마다 random으로 생성됩니다. 여러 adapter instance가 의도적으로 같은
  cache namespace를 공유해야 할 때만 stable private scope를 지정하세요.
- Cache get/set/delete/clear failure에 대한 diagnostics/monitoring을
  application boundary에 추가하세요. Non-miss cache error와 stale-entry delete
  failure는 caller-visible입니다.
- Adapter metrics/log는 parse sentinel(`ErrInvalidToken`, `ErrExpiredToken`,
  `ErrNotYetValid`, `ErrInvalidKey`, `ErrKeyNotFound`), unknown `kid`, key
  revalidation failure, timeout/cancellation, cache get/set/delete/clear
  failure, stale-entry delete failure, rotation/reset 이후 clear failure로
  분류하세요.
- `ForcedRotate`, `ForcedRotateContext`, `DeleteKeyChainsContext`가
  `jwt cache clear failed`를 반환해도 wrapped key operation은 이미 성공했을 수
  있습니다. Key state를 확인하고 affected process-local cache를 clear/recreate하거나
  필요한 instance를 restart한 뒤, rollback으로 간주하지 말고 roll forward하세요.
- Multi-instance deployment에서 process-local cache는 instance마다 따로
  clear됩니다. 안전성은 global invalidation이 아니라 hit revalidation에서 옵니다.
- Raw bearer token, cache key, token digest, parse-profile hash, raw-token
  correlation value를 log에 남기지 마세요.

## Redis Distributed Provider

여러 service instance가 같은 signing authority를 공유해야 할 때는
`github.com/bluetape4k/bluetape-go/jwt/redis`를 사용합니다. Redis repository는
`kid`별 Go-owned KeyChain payload를 저장합니다. Kotlin/JVM wire-compatible
format이 아니고 Kotlin/JVM wire compatibility를 제공하지 않으며, cross-language
storage contract로 취급하면 안 됩니다.

```go
setupCtx, cancelSetup := context.WithTimeout(context.Background(), 2*time.Second)
defer cancelSetup()

client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
defer client.Close()

repo, err := redisjwt.New(redisjwt.Options{
    Client:    client,
    Namespace: "service-auth",
    Capacity:  8,
})
if err != nil {
    return err
}
provider, err := jwt.NewDistributedHMACProvider(setupCtx, repo, jwt.HS256)
if err != nil {
    return err
}

opCtx, cancelOp := context.WithTimeout(context.Background(), time.Second)
defer cancelOp()

token, err := provider.ComposeContext(opCtx, jwt.WithSubject("account-42"))
if err != nil {
    return err
}
reader, err := provider.ParseContext(opCtx, token, jwt.WithExpectedSubject("account-42"))
if err != nil {
    return err
}
if reader.Subject() != "account-42" {
    return errors.New("subject missing")
}
```

Distributed operation은 모두 명시적인 `context.Context`를 요구합니다:
`ComposeContext`, `ParseContext`, `CurrentKeyChainContext`, `RotateContext`,
`ForcedRotateContext`, `FindKeyChainContext`, `DeleteKeyChainsContext`.
`DeleteKeyChainsContext`는 test와 operator reset 절차 전용이며 일반 rotation
경로가 아닙니다.

`redisjwt.Options.KeyTTL`을 설정하는 경우 provider key lifetime과
`RetentionLeeway`를 모두 포함해야 합니다. 그렇지 않으면 token validation leeway가
끝나기 전에 retained signing key가 사라지는 상황을 막기 위해 bootstrap과 rotation이
candidate key를 거부합니다.

Provider의 기본 key lifetime은 365일입니다. 이 값을 줄일 때는 provider와 Redis
repository를 함께 설정합니다.

```go
keyTTL := 24 * time.Hour
retention := 10 * time.Minute

repo, err := redisjwt.New(redisjwt.Options{
    Client:          client,
    Namespace:       "service-auth",
    Capacity:        8,
    KeyTTL:          keyTTL + retention,
    RetentionLeeway: retention,
})
if err != nil {
    return err
}
provider, err := jwt.NewDistributedHMACProvider(setupCtx, repo, jwt.HS256, jwt.WithKeyTTL(keyTTL))
```

`RotateContext`는 살아 있는 current key가 있으면 그대로 반환합니다. Current key가
없거나 만료된 경우 Redis atomic compare-and-store로 concurrent instance가 하나의
winner에 수렴합니다. `ForcedRotateContext`는 항상 새 current key를 저장하고,
retained key는 만료, Redis TTL, repository capacity trim 전까지 검증에 사용할 수
있습니다.

Fixed provider와 local in-memory rotating provider는 process restart나 독립된
service instance 사이에서 token continuity를 보장할 수 없습니다. Redis-backed
distributed provider는 retained key가 남아 있는 동안 continuity를 유지할 수
있습니다. Retained key가 손실, eviction, 삭제, 오래된 backup 복구로 사라진 경우
service owner가 token invalidation 여부를 명시적으로 결정해야 합니다.

![Redis distributed JWT key rotation](../docs/images/readme-diagrams/redis-jwt-distributed-key-rotation.png)

## 동작

- `jwt`는 auth framework가 아닙니다. HTTP middleware, session, OIDC, JWKS,
  authorization rule, role, user model, auth middleware, background rotation
  timer, token revocation storage를 제공하지 않습니다.
- Parse는 항상 `WithValidMethods`로 허용 algorithm을 제한하고, token의 `alg`
  header가 provider algorithm과 일치해야 verification key material을 반환합니다.
- Reader API는 검증된 header와 claim만 노출하며 raw bearer token은 노출하지
  않습니다.
- Fixed provider는 exactly one fixed key일 때만 inbound missing `kid`를
  허용합니다. Rotating provider는 lookup을 위해 `kid`가 필요합니다.
- In-memory rotation은 `kid`별 retained key를 저장하고 repository capacity를
  넘는 오래된 key를 evict합니다. Retained key는 key가 만료되거나 evict되기 전까지
  old token을 검증할 수 있습니다.
- HMAC fixed secret은 선택한 hash size 이상이어야 합니다. 약한 secret은
  `ErrInvalidKey`를 반환합니다.
- RSA provider constructor는 signing을 위한 검증된 2048-bit 이상 private key를
  요구합니다. Provider는 내부 복사본을 저장하고 verification에는 public key
  material을 사용하므로, 생성 후 caller가 원본 key를 mutate해도 provider 동작은
  바뀌지 않습니다.
- Error는 `errors.Is`가 동작하도록 `ErrInvalidToken`, `ErrExpiredToken`,
  `ErrNotYetValid`, `ErrInvalidKey`, `ErrKeyNotFound` 같은 repo-owned sentinel을
  감쌉니다. Error string에는 raw token, HMAC secret, private key를 포함하지
  않습니다.
- Unsupported JOSE/compression header인 `zip`, `crit`, `jku`, `jwk`, `x5u`,
  `x5c`를 포함한 inbound signed token은 거부합니다. #174 결론에 따라 signed
  JWT compression은 non-goal이며, 표준 호환 compression은 future explicit JWE
  API 경계에 속합니다.
- Redis-backed context-aware distributed key storage를 제공합니다. MongoDB는
  #198로 이관했습니다.

## Rotation 계약

`Rotate`는 만료되지 않은 current key가 있으면 그대로 반환하고 만료된 경우에만 새
key를 생성합니다. `ForcedRotate`는 rotating provider에서 항상 새 key를 만듭니다.
Fixed provider는 rotate하지 않습니다.

Key generation은 설정된 entropy가 있으면 그것을 사용하고, 없으면 `crypto/rand`를
사용합니다. 하나의 provider를 여러 goroutine에서 공유할 때 custom entropy reader,
clock, key ID generator는 caller가 concurrent use safety를 보장해야 합니다.

## Redis 운영 Runbook

Redis repository는 다음 key layout을 소유합니다.

```text
bluetape:jwt:v1:<namespace>:meta
bluetape:jwt:v1:<namespace>:current
bluetape:jwt:v1:<namespace>:keys
bluetape:jwt:v1:<namespace>:order
```

Redis는 trusted signing-key boundary로 운영해야 합니다. Remote Redis에는 TLS가
필수입니다. TLS, persistence와 backup, 적절한 sizing policy를 적용하고,
`bluetape:jwt:v1:<namespace>:*`로 제한된 least-privilege ACL user를 구성하세요.
공유 untrusted Redis를 사용하거나 tenant 사이에서 namespace를 재사용하면 안
됩니다. Eviction policy는 `maxmemory-policy noeviction` 또는 동등한 sizing을
권장합니다. Retained signing key가 evict되면 아직 살아 있는 token도 invalidation될
수 있습니다. Redis outage retry policy와 request deadline은 provider method에
전달하는 context로 application이 소유합니다.

대표 진단 명령:

```bash
redis-cli --tls HGET bluetape:jwt:v1:<namespace>:meta version
redis-cli --tls HMGET bluetape:jwt:v1:<namespace>:meta version algorithm
redis-cli --tls GET bluetape:jwt:v1:<namespace>:current
redis-cli --tls HLEN bluetape:jwt:v1:<namespace>:keys
redis-cli --tls ZCARD bluetape:jwt:v1:<namespace>:order
redis-cli --tls PTTL bluetape:jwt:v1:<namespace>:keys
```

Connection, TLS, ACL failure는 empty namespace나 wrong namespace와 분리해서
진단하세요. 위 명령으로 current pointer, hash cardinality, zset cardinality, TTL,
`meta` version/algorithm을 확인합니다. 저장된 algorithm family가 roll-forward할
provider와 다르면 reset 전에 namespace/config를 고치세요. Namespace
misconfiguration은 namespace/config를 고친 뒤 reset 전에 roll-forward하는 방식으로
복구하세요. Reset이 불가피하면 token invalidation을 명시적으로 조율하고,
`DeleteKeyChainsContext`는 test 또는 operator reset workflow로 제한하세요. Rollback은
필요한 retained key가 남아 있는 Redis state로 instance를 되돌리는 방식이어야 하며,
`DeleteKeyChainsContext`를 rollback mechanism으로 사용하면 안 됩니다.

Unknown `kid` lookup, `ErrKeyNotFound`, `ErrInvalidKey`, context timeout/cancel,
Redis command error를 모니터링하세요. Log에는 namespace, `kid`, operation,
sentinel error class를 남길 수 있지만 raw token, HMAC secret, RSA private key,
serialized key payload는 포함하지 않아야 합니다.

## Redis Benchmark Snapshot

아래 benchmark snapshot은 local Testcontainers Redis로 측정한 distributed
repository와 provider path의 근거입니다.

![Redis distributed JWT benchmark](../docs/images/readme-charts/distributed-jwt-redis-benchmark.png)

Raw benchmark output은
`docs/research/outputs/issue-173/distributed-jwt-redis-bench.txt`에 있고, chart
source는 `docs/images/readme-charts/distributed-jwt-redis-benchmark.vl.json`입니다.

## Test

```bash
go test -count=1 ./jwt
go test -count=1 ./jwt/redis
go test -race -count=1 ./jwt
go test -race -count=1 ./jwt ./jwt/redis
```
