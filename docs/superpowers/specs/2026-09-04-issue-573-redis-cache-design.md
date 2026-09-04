# Redis Bucket 및 MapCache primitive 설계

## 상태와 범위

- 상태: 사용자가 승인한 0.20.0 Type A 실행의 설계 기준
- 작업 이슈: [#573](https://github.com/bluetape4k/bluetape-go/issues/573)
- 부모 이슈: [#568](https://github.com/bluetape4k/bluetape-go/issues/568)
- 대상 package: `redis/bucket` (`package redisbucket`), `redis/mapcache` (`package redismap`)
- 기준 head: `352c0bdbbef7ef41362027e3ecb591ed38be1c32`
- 실행 경계: caller-owned `go-redis` client와 `serialization.Serializer`를 사용한다.
  Redis server 설정, persistence/eviction/ACL/TLS, near-cache invalidation과
  stampede coordination은 이 PR이 소유하지 않는다.

이 문서는 raw Redis command와 process-local cache 사이의 작은 내구성 있는
value primitive를 정의한다. `cache/redisnear`, `cache/rediscoord`,
`cache/redisvalue`의 기존 public API와 저장 key는 변경하지 않는다. JSON,
RedisJSON, Java `ConcurrentMap` semantics를 암묵적으로 가정하지 않는다.

## 근거와 source parity ledger

| 근거 | 결정 |
|---|---|
| `redis/key.go`, `redis/errors.go`, `redis/ttl.go` | key structural/logical 분리, redacted `OpError`, TTL validation과 commit-unknown 기초를 재사용한다. |
| `redis/script.go` | owner-safe Lua mutation과 script 오류 wrapping 패턴을 재사용한다. |
| `serialization/serializer.go` | caller codec의 generic `Marshal`/`Unmarshal` 계약을 재사용한다. |
| `cache/redisvalue/value_cache.go`, `cache/redisfory/value_cache.go` | typed serializer, context checkpoint, binary payload 및 caller-owned client 관례 |
| `cache/rediscoord` | loading/stampede lifecycle 전용이므로 Bucket/MapCache 저장 책임과 결합하지 않는다. |
| `redis/lock/options.go`, `redis/stream/validation.go` | typed-nil reflection, key/option validation, context preflight 관례 |
| `testcontainers/redis/redis.go` | 고정 Redis fixture와 readiness/직렬 실행 관례 |

새 dependency는 추가하지 않는다. `github.com/redis/go-redis/v9`와 기존
serialization package만 사용한다.

## 문제와 목표

현재 caller는 raw Redis command를 직접 조합하거나, durable Redis와 near-cache/
stampede 계층을 함께 사용해야 한다. 목표는 다음과 같다.

1. `Bucket[V]`에 single-key `Get`, `Set`, `SetIfAbsent`, `GetAndDelete`, `CompareAndSet`, `Delete`를 제공한다.
2. `MapCache[V]`에 key-per-entry 저장과 entry별 TTL을 제공한다.
3. caller codec과 logical key bytes를 보존하고 namespace/hash-tag를 명시적으로 선택한다.
4. Lua가 필요한 get-and-delete/CAS를 atomic하게 실행하고, cancellation과 commit ambiguity를 typed error로 보존한다.
5. expiry/conditional/delete/concurrent CAS를 fake와 Redis Testcontainers로 증명한다.

## 선택지와 경계

| 선택지 | 결정 | 이유 |
|---|---|---|
| Bucket과 MapCache를 sibling package로 분리 | 채택 | 단일 key primitive와 map entry primitive의 key/원자성/문서 경계가 다르다. |
| MapCache 단일 Redis hash | 거부 | field별 TTL을 표현할 수 없고 caller key preservation/expiry 계약이 복잡해진다. |
| MapCache key-per-entry | 채택 | entry별 TTL, exact logical key, 단일-key Lua와 Redis Cluster slot 정책을 명확히 유지한다. |
| `cache/redisnear` 또는 `rediscoord`를 내부 조합 | 거부 | local invalidation/loader waiter lifecycle이 durable value 저장 책임에 섞여 cancellation 위험을 만든다. |
| JSON/RedisJSON/implicit string codec | 거부 | codec과 wire compatibility는 caller가 선택하며 package가 payload 형식을 정하지 않는다. |
| 범용 `redis/cache` core abstraction | 연기 | 현재 두 package의 public contract보다 큰 lowest-common-denominator layer를 만들지 않는다. |

## Public API

```go
package redisbucket

type Client interface {
    Get(context.Context, string) *redis.StringCmd
    Set(context.Context, string, any, time.Duration) *redis.StatusCmd
    SetNX(context.Context, string, any, time.Duration) *redis.BoolCmd
    Del(context.Context, ...string) *redis.IntCmd
    Eval(context.Context, string, []string, ...any) *redis.Cmd
}

type Options[V any] struct {
    Namespace string
    HashTag   string
    Serializer serialization.Serializer[V]
    Logger    *slog.Logger
}

func New[V any](client Client, options Options[V]) (*Bucket[V], error)

type Bucket[V any] struct { /* constructor-only immutable key/codec/client */ }

func (b *Bucket[V]) Get(context.Context, string) (V, bool, error)
func (b *Bucket[V]) Set(context.Context, string, V, time.Duration) error
func (b *Bucket[V]) SetIfAbsent(context.Context, string, V, time.Duration) (bool, error)
func (b *Bucket[V]) GetAndDelete(context.Context, string) (V, bool, error)
func (b *Bucket[V]) CompareAndSet(context.Context, string, V, V, time.Duration) (bool, error)
func (b *Bucket[V]) Delete(context.Context, string) error
```

`redis/mapcache`는 같은 `Client`/`Options[V]`와 method set을 제공하되
constructor가 `map` structural segment를 사용한다. 두 package의 client는
`*redis.Client`와 mutex-safe fake가 구현할 수 있는 최소 subset이며 client를
닫거나 command retry를 추가하지 않는다. `Options`는 생성 시 복사된다.

`Namespace`는 blank/whitespace-only를 거부하고 exact non-blank 문자열을
`btredis.NewKeyBuilder` prefix로 사용한다. `HashTag`가 비어 있지 않으면
`WithHashTag`로 exact bytes를 보존한다. `Serializer`의 nil interface와 모든
reflect nil-capable typed-nil은 constructor에서 거부한다. `Logger`를 생략하면
caller의 `slog.Default()`를 사용하고 global logger 설정을 변경하지 않는다.
운영 log는 operation/result 같은 low-cardinality 필드만 기록하며 raw key,
payload, provider text는 기록하지 않는다.

zero-value `Bucket`/`MapCache`는 constructor-only다. method는 초기화 검증 후
고정 `ErrUninitialized`를 반환하며 panic하지 않는다.

## Key와 TTL 계약

Bucket key layout은 `Namespace:bucket:{optional-hash-tag}:<logical-key>`이고
MapCache는 `Namespace:map:{optional-hash-tag}:<logical-key>`다. structural
segment는 package가 소유하고, logical key는 `btredis.KeyBuilder.LogicalKey`
로 넘겨 spaces, braces, colons와 UTF-8 bytes를 trim/case-fold/normalize하지
않고 보존한다. logical key가 blank/whitespace-only인 경우만
`btredis.ErrInvalidKey`다. hash tag는 Redis Cluster same-slot hint이지
authorization이나 tenant isolation boundary가 아니다.

`ttl == 0`은 persistent entry, `ttl < 0`은 `btredis.ErrInvalidTTL`,
`0 < ttl < 1ms`는 wire에서 1ms로 올림한다. 양수 TTL은 millisecond 단위로
내림해 전달하되 1ms보다 작아지지 않게 한다. Redis persistence/eviction 정책은
caller/operator가 관리하며 package가 durability를 보장한다고 주장하지 않는다.

## 직렬화와 오류

각 write는 dispatch 전에 `Serializer.Marshal`을 호출하고, read는 response와
context를 확인한 뒤에만 `Unmarshal`한다. codec error는
`ErrSerialization`/`ErrInvalidPayload` typed error로 감싸되 payload, logical
key, provider text를 error string에 넣지 않는다. serializer가 반환한 byte slice를
수정하지 않으며 fake는 request bytes를 deep-copy한다.

provider 오류는 `btredis.NewOpErrorWithRedactedKey`를 기반으로 한 package
`Error`로 반환한다. `Error()`와 `%+v`에는 operation과 redacted key ID만
포함한다. `errors.Is`/`errors.As`는 원인, `btredis.ErrCommitUnknown`, package
sentinel을 보존한다. `ErrCommitUnknown`은 mutation command가 dispatch된 뒤
provider error와 cancellation이 함께 발생하거나 malformed result를
reconcile할 수 없을 때 사용한다.

## Bucket 원자성

- `Get`은 `GET` 한 번을 수행한다. Redis `Nil`은 `(zero, false, nil)`이며 empty
  payload는 serializer가 해석한다.
- `Set`은 `SET key payload`를 수행한다. `ttl==0`은 expiration 0, 양수는
  normalized duration으로 전달한다.
- `SetIfAbsent`는 `SETNX`를 사용하고 false는 existing owner/value를 뜻하는
  정상 결과다.
- `GetAndDelete`는 다음 Lua를 한 번 실행한다.

  ```lua
  local value = redis.call("GET", KEYS[1])
  if not value then return {0} end
  redis.call("DEL", KEYS[1])
  return {1, value}
  ```

  결과 `{0}`은 miss, `{1,payload}`는 read와 delete가 모두 같은 invocation에서
  성공한 hit다. malformed result는 `ErrMalformedResult`다.
- `CompareAndSet`는 expected payload와 현재 GET bytes가 정확히 같을 때만
  replacement를 저장한다. Lua는 persistent와 `PX milliseconds` 두 branch를
  사용하고 `{1}`/`{0}` 외 결과를 malformed로 거부한다. missing key는 false다.
- `Delete`는 `DEL`을 호출하고 count와 무관하게 성공한다. provider error가
  dispatch 뒤 확정되지 않으면 commit unknown을 보존한다.

## MapCache 원자성과 범위

MapCache method는 같은 value contract를 사용하고 `map` structural segment 뒤에
각 logical key를 하나의 Redis key로 저장한다. 따라서 entry마다 독립 TTL을
설정하고 한 entry의 `GetAndDelete`/CAS가 다른 entry를 잠그지 않는다. 전체
namespace clear, iteration, bounded scan, local eviction, cross-key atomic
transaction은 제공하지 않는다. 호출자가 여러 key를 함께 갱신해야 하면 Redis
Cluster hash-tag와 별도 transaction/script 소유권을 명시해야 한다.

MapCache는 `cache.Cache`를 구현한다고 주장하지 않는다. `cache.Cache.Get`의
miss sentinel 대신 hit 여부를 반환하는 이유는 missing/expired와 serialized zero
value를 구분하기 위해서다. `cache/redisnear` invalidation과
`cache/rediscoord` loader waiter는 caller가 별도 조합한다.

## Context와 cancellation

nil context는 `ErrInvalidContext`로 거부하고 command를 dispatch하지 않는다.
이미 취소된 context도 provider 호출 없이 정확한 `context.Canceled` 또는
`context.DeadlineExceeded`를 반환한다. 모든 method는 dispatch 직전과 response
직후 context를 확인한다. read는 취소 뒤 decode하지 않는다. mutation은
response error와 context cancellation이 겹치면 context를 보존하고
`btredis.ErrCommitUnknown`을 함께 반환한다. 성공 response 뒤 cancellation은
caller cancellation을 반환하되 provider가 성공한 mutation을 되돌리거나
재시도하지 않는다.

package는 goroutine, timer, retry worker, client close를 소유하지 않는다.
caller가 cleanup deadline을 만들고, unknown mutation은 Redis 상태를 별도로
관찰하거나 idempotent workflow로 재시도한다.

## Failure matrix

| 경계 | 반환/효과 |
|---|---|
| invalid client/serializer/namespace/key/TTL | dispatch 0회, typed validation sentinel |
| codec marshal/unmarshal 오류 | `ErrSerialization` 또는 `ErrInvalidPayload`, raw payload 없음 |
| Redis `Nil` on read | `(zero, false, nil)` |
| SetNX/CAS false | 정상 조건 불충족, 다른 value 변경 없음 |
| provider error before result | redacted `Error`; mutation이면 `ErrCommitUnknown` |
| malformed Lua result | `ErrMalformedResult`; mutation이면 commit unknown |
| response 후 context cancellation | context error 우선, decode/재시도 없음 |
| concurrent CAS | Redis atomic script 중 정확히 하나만 expected를 소비 |

## 테스트와 수용 기준

mutex-safe fake client는 command args와 payload를 deep-copy하고 호출 수,
context, configured result/error, output-plus-error를 기록한다.

- constructor/options, namespace/hash-tag, exact key preservation, typed-nil client/serializer
- Bucket/MapCache Get/Set/SetIfAbsent/Delete success/miss/TTL normalization
- Lua get-and-delete/CAS result parser, malformed/partial result, codec failure
- dispatch 전/후 cancellation, mutation unknown, safe `Error()`/`%+v`, `errors.Is`/`errors.As`
- concurrent CAS exact winner count와 race test
- Redis Testcontainers를 하나씩 실행해 expiry, conditional write, delete,
  cancellation, concurrent CAS 및 readiness/cleanup을 검증
- compile-checked examples가 durable Redis, near-cache, stampede coordination의
  경계를 각각 설명

일반 CI는 fake/unit, examples, `go vet`/lint를 실행한다. Redis Testcontainers는
공유 Docker 자원을 고려해 repository suite와 직렬화한다. real Redis 설정,
persistence/eviction/ACL/TLS와 production rollout은 DoD에서 제외하되 README에
명시한다.

SPW-01 요구사항은 live #573 body와 #568 parent metadata로 확인했다. SPW-02
설계는 이 문서, SPW-03 실행 plan, SPW-04 RED→GREEN 구현, SPW-05 fresh
verification evidence를 plan/review/PR evidence에 연결한다.

## 문서와 rollback

각 package에 English/Korean README와 compile-checked example을 추가하고,
`redis/README.md`와 `README.ko.md`에는 sibling link와 durable Redis vs
near-cache vs stampede coordination 표를 추가한다. README는 Redis persistence,
eviction, ACL/TLS/maxmemory, caller-owned client/codec/retry/timeout, key/hash-tag,
TTL와 cancellation/commit-unknown 의미를 숨기지 않는다.

rollback은 새 package consumer를 제거하거나 PR commit을 revert하는 것으로
한정한다. 기존 `cache/*`, `redis/*` 저장 key와 behavior는 변경하지 않는다.
