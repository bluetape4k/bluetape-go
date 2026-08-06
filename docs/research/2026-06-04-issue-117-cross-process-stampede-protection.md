# Issue #117 Cross-Process Cache Stampede Protection Research

Issue: #117
Milestone: 0.3.0
Date: 2026-06-04
Work type: Type A - Full Feature

## Research Question

#22 local cache contract와 #23 Redis Pub/Sub NearCache work를 약화하지 않고, `redisnear`를 숨은 value store로 바꾸지 않으면서
`bluetape-go`가 cross-process cache stampede protection을 어떻게 추가해야 하는가?

## Current Behavior

`cache.Memory.GetOrLoad`는 하나의 cache instance 안에서만 duplicate same-key load를 억제한다. 내부적으로 `singleflight`를
사용하고 성공한 loader result를 process-local cache에 저장한다.

`cache/redisnear.NearCache.GetOrLoad`는 local cache에 위임한다. Redis는 peer invalidation에만 사용된다. peer invalidation은
다른 process의 local entry를 evict하지만, 그 뒤의 reload를 coordinate하지 않는다.

`lock/redis`는 reusable single-Redis-instance owner-token lock을 제공한다. acquire에는 `SetNX(ctx, key, token, ttl)`을
사용하고 unlock에는 compare-and-delete Lua script를 쓴다.

## External Evidence

| Source | Relevant point | Decision impact |
|---|---|---|
| Redis `SET` docs, https://redis.io/docs/latest/commands/set/ | `SET`은 `NX`와 `PX` 같은 positive expiration option을 지원한다. | Redis는 short-lived load lease를 atomic하게 만들 수 있다. |
| Redis distributed locks docs, https://redis.io/docs/latest/develop/clients/patterns/distributed-locks/ | single-instance locking은 unique random value를 쓰며 release는 delete 전에 value를 compare해야 한다. mutual exclusion은 lock validity time에 묶인다. | #24 owner-token/TTL lock을 재사용하고 over-TTL loader는 overlap될 수 있음을 문서화한다. |
| Redis `EVAL` docs, https://redis.io/docs/latest/commands/eval/ | script는 accessed key를 `KEYS`로, 추가 argument를 `ARGV`로 받아야 한다. | Redis `DELEX IFEQ` 이전 version에서는 #24 unlock script가 안전한 release primitive다. |
| Redis cache stampede glossary, https://redis.io/glossary/cache-stampede/ | stampede는 많은 client가 expired 또는 missing item을 동시에 regenerate할 때 발생한다. | #117 target은 peer invalidation만이 아니라 miss 이후 load coordination이 필요하다. |

## 핵심 설계 제약

Redis lock만으로는 backend loader execution을 serialize할 수 있지만 process 간 loaded Go value를 공유할 수 없다. `redisnear`는
invalidation-only이며 generic `V any`에 대한 codec이 없다. 따라서 explicit value-sharing mechanism 없이는 loser process가
winner의 loaded value를 받을 수 없다.

그러므로 두 semantics는 실질적으로 다르다.

- **Load serialization:** 한 번에 하나의 process만 loader를 실행하지만, 다른 cold process가 lock release 뒤 loader를 나중에 실행할 수 있다.
- **Load result collapse:** 한 process가 loader를 실행한 뒤 waiter가 같은 result를 재사용하고 user loader를 실행하지 않고 local cache를 채운다.

#117은 cross-process duplicate-load suppression과 같은 cold key를 두고 여러 near-cache instance가 경쟁하는 Testcontainers proof를
요구하므로 load result collapse를 구현해야 한다.

## Options

| Option | Summary | Pros | Cons | Decision |
|---|---|---|---|---|
| `redisnear.NearCache.GetOrLoad` default 변경 | Redis coordination을 모든 near-cache load에 직접 추가한다. | package 하나이고 user discovery cost가 낮다. | hidden latency, 모든 cold miss의 Redis command 증가, `redisnear`가 의도적으로 피한 value codec 필요, #23 invalidation-only contract 변경. | Reject. |
| lock-only opt-in wrapper | #24 Redis lock으로 loader execution만 감싼다. result sharing 없음. | 작고 codec이 필요 없으며 lease expiry를 이해하기 쉽다. | simultaneous backend load는 막지만 cold process 간 result collapse는 하지 못한다. #117의 강한 해석을 만족하지 못한다. | #117 primary implementation으로는 Reject; fallback pattern으로 문서화 가능. |
| result envelope을 가진 opt-in Redis coordinator | `cache.LoadingCache[string,V]`를 감싸고 #24 lock과 caller-supplied codec으로 short-lived Redis result key를 사용한다. | `cache.LoadingCache` 변경 없음, `redisnear` default behavior 보존, true cross-process result collapse 지원, NearCache를 underlying L1으로 사용 가능. | explicit codec이 필요하고 Redis가 transient coordination/result transport라는 점을 문서화해야 한다. | Adopt. |
| Redis L2 cache package 추가 | Redis에 durable cache value를 저장하고 local cache를 L1으로 만든다. | shared value를 원하는 app cache에 장기적으로 유용하다. | eviction, TTL consistency, serialization policy, L2를 우회한 write invalidation 등 더 큰 semantics가 필요하다. | 필요하면 future issue로 Defer. |

## 채택 방향

새 opt-in package를 `cache/rediscoord` 아래에 추가한다.

이 package는 기존 `cache.LoadingCache[string,V]`를 감싼다. #117 test에서는 underlying cache가
`cache/redisnear.NearCache[V]`다. wrapper는 `cache.LoadingCache[string,V]`를 구현하고 `Get`, `Set`, `Delete`, `Clear`를
underlying cache에 위임한다.

`GetOrLoad` behavior:

1. 먼저 underlying cache를 확인한다.
2. miss이면 namespace와 cache key로 Redis owner-token load lock 획득을 시도한다.
3. winner는 underlying cache를 통해 loader를 실행하고 lock release 전에 short-lived result envelope을 Redis에 publish한다.
4. waiter는 lock owner token과 matching result envelope을 poll한다.
5. waiter가 matching result를 보면 decoded value를 반환하는 작은 local loader를 사용해 user loader를 호출하지 않고 underlying cache를 채운다.
6. owner가 실패하면 lock TTL이 wait를 제한한다. peer는 lease expiry 뒤 retry하거나 context error를 반환한다.

## Result Envelope

result key는 durable cache가 아니다. 현재 load attempt의 waiter만 사용하는 short-lived coordination artifact다.

envelope은 다음을 포함한다.

- `version`: result format version.
- `token`: attempt의 Redis lock owner token.
- `payload`: `V`에 대한 codec output.

waiter는 envelope token이 현재 lock attempt에서 관찰한 owner token과 일치할 때만 envelope을 수락한다. 이 규칙은 이전 load
window가 남긴 stale result key 소비를 막는다.

## Public API Direction

```go
package rediscoord

type Codec[V any] interface {
    Marshal(V) ([]byte, error)
    Unmarshal([]byte) (V, error)
}

type JSONCodec[V any] struct{}

type Options[V any] struct {
    Client       redis.Cmdable
    Cache        cache.LoadingCache[string, V]
    Namespace    string
    Codec        Codec[V]
    LockTTL      time.Duration
    ResultTTL    time.Duration
    PollInterval time.Duration
}

type StampedeCache[V any] struct { ... }

func NewStampedeCache[V any](options Options[V]) (*StampedeCache[V], error)
```

defaults:

- `Namespace`: `default`.
- `LockTTL`: `5s`.
- `ResultTTL`: `1s`.
- `PollInterval`: `10ms`.

required:

- `Client`.
- `Cache`.
- `Codec`.

## Failure Semantics

- loader error는 cache하지 않고 result envelope도 publish하지 않는다.
- caller cancellation은 caller의 context error를 반환하고 result를 cache하지 않는다.
- winner가 result publish 전에 crash 또는 return하면 lock TTL이 waiter retry를 가능하게 하여 deadlock을 막는다.
- loader가 `LockTTL`을 넘으면 다른 process가 load lock을 획득해 loader를 동시에 실행할 수 있다. 이는 #24 및 Redis
  single-instance lock guidance와 같은 lease-validity boundary다.
- wrapper는 external system write에 대한 fencing token을 제공하지 않는다.
- result envelope은 transient이며 Redis-backed durable cache value로 다루면 안 된다.

## Test Requirements

- option validation, JSON codec, result envelope token matching, wrapper interface conformance unit test.
- 같은 namespace 아래 두 `redisnear.NearCache` instance를 사용하는 Testcontainers test:
  - 두 local cache를 prime한다.
  - 한 peer를 통해 key를 invalidate한다.
  - 같은 cold key에 대해 coordinator wrapper를 동시에 호출한다.
  - user loader invocation이 하나이고 returned value가 같은지 assert한다.
- 두 wrapper와 하나의 key를 사용하는 `GoroutineStressTester` stress test. 모든 caller가 완료되고 cold burst에서 loader count가
  하나로 남아야 한다.
- `AsyncJobTester`를 사용하는 cancellation test. waiter는 hang하지 않고 context cancellation/deadline error를 반환해야 한다.
- abandoned 또는 over-TTL loader가 peer를 deadlock하지 않는 lease expiry test.
- 새 package와 `cache/redisnear`에 대한 race-targeted test run.

## Benchmark Boundary

coordinator benchmark는 `make ci`에 추가하지 않는다. #107은 다음 opt-in benchmark work를 측정할 수 있게 update 또는 link한다.

- winner path overhead.
- waiter result-sharing latency.
- invalidation pressure 아래 operation당 load count.
- lease expiry recovery.

## 결정

`cache/rediscoord.StampedeCache`를 explicit opt-in cross-process load-result coordinator로 구현한다.
`cache.LoadingCache`는 변경하지 않고 `redisnear.NewPubSub`은 기본적으로 invalidation-only로 유지한다.
