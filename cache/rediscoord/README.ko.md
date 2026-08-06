# cache/rediscoord

[English](README.md) | [한국어](README.ko.md)

`cache/rediscoord`는 process 사이의 cache stampede를 막기 위해 선택적으로
붙이는 Redis coordination wrapper입니다. `cache/redisnear.NearCache`를 포함한
기존 `cache.LoadingCache[string,V]`를 감싸고, cold burst 동안 waiter가 winning
loader의 결과를 재사용하게 합니다.

이 패키지는 durable Redis L2 cache가 아닙니다. Redis에는 active load attempt를 위한 짧은 수명의 owner-token result envelope만 저장됩니다.

## 다이어그램

![Redis cache stampede coordination flow](../../docs/images/readme-diagrams/rediscoord-cold-burst-coordination.png)

![rediscoord cold burst sequence](../../docs/images/readme-diagrams/rediscoord-cold-burst-sequence.png)

## 가져오기

```go
import (
    "github.com/apache/fory/go/fory"
    "github.com/bluetape4k/bluetape-go/cache/rediscoord"
    rediscoordfory "github.com/bluetape4k/bluetape-go/cache/rediscoord/fory"
)
```

## 사용 예

```go
client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

near, err := redisnear.NewPubSub[string](ctx, redisnear.Options[string]{
    Client:    client,
    Namespace: "catalog",
})
if err != nil {
    return err
}
defer func() { _ = near.Close() }()

coordinated, err := rediscoord.NewStampedeCache[string](rediscoord.Options[string]{
    Client:    client,
    Cache:     near,
    Namespace: "catalog",
    Codec:     rediscoord.JSONCodec[string]{},
    MaxResultBytes: 2 << 20,
})
if err != nil {
    return err
}

value, err := coordinated.GetOrLoad(ctx, "sku:42", time.Minute,
    func(ctx context.Context, key string) (string, error) {
        return loadCatalogValue(ctx, key)
    },
)
```

## Go-native Apache Fory Codec

신뢰할 수 있는 내부 Go-only coordination payload에는
`cache/rediscoord/fory`를 import하고 profile을 명시적으로 선택합니다.

```go
codec, err := rediscoordfory.NewNativeFast[CatalogValue](rediscoordfory.Options{
    Register: func(runtime *fory.Fory) error {
        return runtime.RegisterStructByName(CatalogValue{}, "catalog.ValueV1")
    },
})
if err != nil {
    return err
}

coordinated, err := rediscoord.NewStampedeCache[CatalogValue](rediscoord.Options[CatalogValue]{
    Client: client, Cache: localCache, Namespace: "catalog:fory-native-fast:schema-v1",
    Codec: codec, MaxResultBytes: 2 << 20,
})
```

`NewNativeFast`는 schema가 고정된 값에 사용합니다.
`NewNativeCompatible`은 Fory가 지원하는 field 호환 진화를 허용하지만 semantic
변경이나 incompatible type 변경까지 안전하게 만들지는 않습니다. 두 constructor
모두 xlang과 reference tracking을 끕니다. Bool, integer, unsigned integer,
floating-point, struct, string, `[]byte` root를
지원하며 pointer, complex, map, array, non-byte slice, interface, function,
channel, unsafe-pointer root는 거부합니다.

기본 제한값은 payload 1 MiB, depth 20, field 512개, type metadata 4096 bytes,
type별 schema version 10개, 평균 schema version 3개입니다. `CodecError`는 payload나
provider 상세를 형식화하지 않고 안정적인 operation, profile, reason label을
제공합니다. Reason은 `configuration`, `uninitialized`, `registration`,
`payload-too-large`, `invalid-magic`, `unsupported-version`, `profile-mismatch`,
`length-mismatch`, `unsupported-value`, `fory-failure`입니다.

Fory는 암호화가 아닙니다. Redis operator는 byte를 관찰할 수 있습니다. 민감한
값에는 Redis ACL, TLS, namespace isolation을 사용하세요.

### Rollout과 rollback

같은 namespace를 공유하는 모든 process는 동일한 codec profile, registration
set, `MaxResultBytes`, Fory resource limit을 사용해야 합니다.
`catalog:fory-native-fast:schema-v1` 같은 namespace를
사용하고 JSON, native-fast, native-compatible 값을 한 namespace에 섞지 마세요.
Reader와 writer를 함께 전환한 뒤 이전 namespace를 최소
`LockTTL + ResultTTL + safety margin` 동안 유지합니다. Rollback은 이전
codec/namespace pair로 되돌립니다. Cleanup은 TTL을 고려한 bounded `SCAN MATCH`를
사용하고 `KEYS`는 사용하지 않습니다. Lock과 result key는 각각
`bluetape:cache:coord:<namespace>:lock:*`와
`bluetape:cache:coord:<namespace>:result:*` pattern으로 scan합니다.

## 동작

- `Get`, `Set`, `Delete`, `Clear`는 감싼 cache로 위임합니다.
- `GetOrLoad`는 먼저 감싼 cache를 확인합니다.
- Cold miss에서는 한 process가 Redis owner-token load lease를 획득합니다.
- Winner는 감싼 cache를 통해 user loader를 실행하고 짧은 수명의 result envelope를 게시합니다.
- Waiter는 관찰한 load owner와 token이 일치하는 envelope만 수락합니다.
- Waiter는 `Set`이 아니라 감싼 `GetOrLoad`로 local cache를 채우므로 `redisnear`가 우발적인 invalidation을 게시하지 않습니다.

## 운영 경계

- Redis는 encoded payload byte를 볼 수 있습니다. 민감한 payload에는 ACL/TLS와 namespace isolation을 사용하세요.
- Result envelope는 일시적인 coordination metadata이며 durable cache value가 아닙니다.
- Mutual exclusion은 `LockTTL`로 제한됩니다. Loader가 lease보다 오래 실행되면 다른 process가 load lease를 획득해 loader를 실행할 수 있습니다.
- 직접 Redis command 실패는 `errors.Is`로 원인을 유지하고 `errors.As`로 typed
  `redis.OpError`를 제공합니다. 형식화된 진단에는 raw Redis key, owner token,
  payload, provider text가 노출되지 않습니다.
- `MaxResultBytes`는 encoded JSON/base64 owner-result envelope를 Redis 게시 전과
  JSON decode 전에 제한합니다. 0은 기존 unlimited 동작을 유지합니다.
- Benchmark는 `make bench-cache`로 opt-in 실행합니다. 일반 `make ci`는 benchmark workload를 실행하지 않습니다.

## 테스트

```bash
go test -count=1 ./cache/rediscoord
```

## 벤치마크

```bash
go test -run '^$' -bench '^BenchmarkStampedeCache' -benchmem ./cache/rediscoord
```

## 벤치마크 스냅샷

아래 수치는 local smoke number이며 production capacity ranking이 아닙니다. 실행 환경은 macOS arm64 Apple M4 Pro, `-benchtime=100ms`였고 Redis-backed benchmark는 Testcontainers Redis 7.4를 사용했습니다. 낮은 `ns/op`가 더 좋습니다. Local hit path와 Redis coordination path의 차이가 커서 chart는 log scale을 사용합니다.

![Redis coordinator benchmark latency](../../docs/images/readme-charts/rediscoord-benchmark-latency.png)

| Benchmark | ns/op | B/op | allocs/op | Extra |
|---|---:|---:|---:|---:|
| `BenchmarkMemoryGetHit` | 42.68 | 0 | 0 |  |
| `BenchmarkStampedeCacheGetOrLoadHot` | 52.92 | 16 | 1 |  |
| `BenchmarkNearCacheGetLocalHit` | 57.83 | 16 | 1 |  |
| `BenchmarkNearCacheGetOrLoadUnderInvalidation` | 279.9 | 43 | 2 | `0.005107 loads/op` |
| `BenchmarkMemoryGetOrLoadCold` | 1065 | 784 | 10 | `1.000 loads/op` |
| `BenchmarkMemoryGetOrLoadSameKeyConcurrent` | 11030 | 4189 | 57 | `1.000 loads/op` |
| `BenchmarkNearCacheSetPublish` | 424923 | 1209 | 29 |  |
| `BenchmarkStampedeCacheGetOrLoadColdWinner` | 1685522 | 2692 | 58 | `1.000 loads/op` |
