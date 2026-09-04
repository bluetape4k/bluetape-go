# Issue #572 Redis Fenced Lock and Semaphore Design

> 한국어 요구사항 경계: 이 spec/design 문서는 구현자가 API와 안전성 계약을
> 한국어로 추적할 수 있도록 작성한다. API 이름, package path, Redis command,
> code identifier, issue/PR 번호, acceptance keyword, DoD/test evidence는
> 호환성과 검증력을 위해 원문 그대로 보존한다. 구현자는 아래 결정과
> invariant를 축소하거나 암묵적인 동작으로 바꾸지 않는다.

Status: design approved; written spec review pending

Issue: [#572](https://github.com/bluetape4k/bluetape-go/issues/572)

Parent: [#568](https://github.com/bluetape4k/bluetape-go/issues/568)

Target packages: `redis/lock`, `redis/semaphore`

## 문제와 목표

공유 Redis substrate(`#569`)와 owner-token lock substrate(`#579`)는
키/TTL/lease/token/Lua 오류의 공통 경계를 제공하지만, Go 서비스가 사용할
수 있는 두 가지 고수준 동기화 primitive가 아직 없다.

이 issue는 다음을 추가한다.

1. 성공적인 획득마다 증가하는 `FencedLock`과 fencing token을 반환하는 lease.
2. bounded permit 수를 원자적으로 관리하는 Redis semaphore.
3. `Acquire(ctx)`의 context-aware 대기와 즉시형 `TryAcquire(ctx)`.
4. TTL 만료, owner-safe release, cancellation, Redis mutation ambiguity에
   대한 명시적인 계약과 Testcontainers 검증.
5. stale holder를 실제로 거부하는 외부 resource 예제와 TTL을 넘긴 작업의
   overlap caveat 문서.

## 범위와 비범위

### 범위

- `redis/lock`에 `FencedLock`과 `Lease`를 추가한다.
- `redis/semaphore`에 bounded `Semaphore`와 `Lease`를 추가한다.
- caller-owned `redis.Cmdable` client와 caller-owned `context.Context`를
  사용한다. 생성자는 client를 닫지 않는다.
- `btredis.KeyBuilder`, `btredis.OwnerToken`, `btredis.Lease`,
  `btredis.ValidateTTL`/`TTLMillis`, `btredis.OpError`,
  `btredis.ErrCommitUnknown`, `btredis.CompareAndDelete`를 가능한 경로에서
  재사용한다.
- Redis Cluster에서 한 Lua invocation의 모든 key가 동일 hash slot에
  있도록 내부 key를 만든다.
- package README, `README.ko.md`, compile-checked examples, unit/fake tests,
  Redis Testcontainers tests, same-key concurrency tests를 추가한다.

### 비범위

- 기존 `lock/redis.Mutex`의 public API나 저장 key를 변경하지 않는다.
- Redlock quorum, Redisson protocol, fair/FIFO queue를 추가하지 않는다.
- goroutine 기반 watchdog renewal, background goroutine, implicit retry
  worker, `Close` lifecycle을 추가하지 않는다.
- semaphore에 fencing token을 가장하거나 외부 resource 보호를 주장하지
  않는다.
- lease TTL이 critical section의 실행 시간보다 길다는 보장을 하지 않는다.

## 선택지와 결정

| 접근 | 결정 | 이유 |
|---|---|---|
| `redis/lock`과 `redis/semaphore`를 독립 package로 두고 shared substrate를 재사용 | 채택 | issue의 두 primitive 경계와 Redis 자료구조가 명확하고, 기존 `lock/redis` 계약과 충돌하지 않는다. |
| 범용 `redis/coordination` core를 먼저 만들고 wrapper를 제공 | 거부 | 현재 요구보다 큰 추상화가 생기고 primitive별 원자성/소유권 규칙이 lowest-common-denominator API에 묻힌다. |
| 기존 `lock/redis.Mutex`를 fenced lock으로 확장 | 거부 | 기존 owner-token lock의 호환성을 깨며 issue의 별도 primitive 경계를 위반한다. |

이번 slice에서는 retry backoff도 public option으로 노출하지 않는다. 고정된
bounded backoff는 API를 얇게 유지하고, caller는 `context.WithTimeout`으로
대기 상한을 제어한다.

## 아키텍처

```text
caller-owned redis.Cmdable + context.Context
             |
             +--------------------------+
             |                          |
             v                          v
       redis/lock                  redis/semaphore
       FencedLock                  Semaphore
             |                          |
             | Lua: owner + counter     | Lua: ZSET leases
             v                          v
       Redis shared substrate: KeyBuilder / OwnerToken / Lease / OpError
```

두 package는 public API는 분리하지만 다음 규칙을 공유한다.

- 생성자에서 client, key, TTL, permit 수를 검증한다.
- mutation dispatch 전 `ctx.Err()`를 확인한다.
- Redis 오류는 raw key/token을 노출하지 않는 `btredis.OpError`로 감싼다.
- command가 dispatch된 뒤 결과가 확정되지 않으면 `btredis.ErrCommitUnknown`을
  원인에 함께 보존한다.
- `Acquire`는 `TryAcquire`에서 `ErrNotAcquired`만 재시도하며 provider 오류,
  context 오류, validation 오류는 재시도하지 않는다.
- 대기는 timer와 context select로만 수행하며 goroutine을 남기지 않는다.

## Key 설계와 Cluster 계약

caller의 logical key는 빈 문자열/공백만인 경우를 제외하고 byte-for-byte로
식별에 사용한다. 실제 Redis key에는 raw caller key를 직접 넣지 않고
`btredis.RedactedKeyID(logicalKey)`를 hash tag로 사용한다. 따라서 공백,
colon, braces가 포함된 서로 다른 logical key가 서로 같은 key로 정규화되지
않으며, raw key가 Redis 오류나 로그에 새어 나오지 않는다.

개념적인 key layout은 다음과 같다.

```text
bluetape:redis:lock:{redis-key:<digest>}:owner
bluetape:redis:lock:{redis-key:<digest>}:counter
bluetape:redis:semaphore:{redis-key:<digest>}:leases
```

`KeyBuilder.WithHashTag`와 `StructuralKey`로 생성하므로 lock의 owner/counter
두 key와 semaphore의 leases key는 모두 동일 slot에 놓인다. fencing counter는
owner key와 달리 TTL을 설정하지 않는다. counter를 만료시키면 lock을 여러 번
획득한 뒤 token이 다시 낮아질 수 있으므로, counter의 영속성은 fencing
contract의 일부다.

## `redis/lock` public API

```go
package redislock

type Options struct {
    Key string
    TTL time.Duration
}

type FencedLock struct { /* constructor-owned immutable state */ }

func New(client redis.Cmdable, options Options) (*FencedLock, error)
func (l *FencedLock) TryAcquire(ctx context.Context) (*Lease, error)
func (l *FencedLock) Acquire(ctx context.Context) (*Lease, error)

type Lease struct { /* immutable owner token and fencing token */ }

func (lease *Lease) Key() string
func (lease *Lease) OwnerToken() btredis.OwnerToken
func (lease *Lease) FencingToken() uint64
func (lease *Lease) Release(ctx context.Context) (bool, error)
```

`ErrNotAcquired`는 해당 package의 sentinel error다. `TryAcquire`는 lock이
이미 점유 중이면 `(nil, ErrNotAcquired)`를 반환한다. `Acquire`는 같은
원자적 시도를 bounded backoff로 반복하며 `ctx`가 취소되거나 deadline에
도달하면 정확한 `context.Canceled` 또는 `context.DeadlineExceeded`를
보존한다.

### FencedLock 원자성

획득 script는 다음 순서를 하나의 Redis Lua invocation으로 수행한다.

1. owner key가 존재하면 `{0, 0}`을 반환한다.
2. owner key가 없으면 counter key를 `INCR`한다.
3. generated `OwnerToken`을 owner key에 `SET ... PX ttl` 한다.
4. `{1, fencingToken}`을 반환한다.

`fencingToken`은 성공한 획득에만 배정되고, owner TTL과 무관하게 단조
증가한다. lease의 fencing token은 외부 resource가 저장한 마지막 token보다
작거나 같은 요청을 거부할 때만 stale-holder 보호를 제공한다. token 자체는
Redis lock의 소유권 증명이 아니다.

`Release`는 `btredis.NewLease(ownerKey, ownerToken)`를 만들고 shared
`btredis.CompareAndDelete`를 사용한다. owner key가 이미 만료됐거나 다른
owner가 새로 획득했다면 `(false, nil)`이며 다른 owner의 key를 삭제하지
않는다. 성공한 release는 `(true, nil)`이다. 같은 lease의 두 번째 release도
`(false, nil)`이다.

획득 command가 오류를 반환했지만 owner key가 해당 token으로 확인되면
bounded reconciliation을 시도해 lease와 counter를 복원한다. owner 상태와
fencing token을 확정할 수 없으면 `btredis.OpError`와
`btredis.ErrCommitUnknown`을 함께 반환한다. 호출자는 이 경우 cleanup
context로 상태를 정리하거나 새 lease를 발급받기 전에 외부 resource를
재검증해야 한다.

## `redis/semaphore` public API

```go
package redissem

type Options struct {
    Key     string
    Permits int
    TTL     time.Duration
}

type Semaphore struct { /* constructor-owned immutable state */ }

func New(client redis.Cmdable, options Options) (*Semaphore, error)
func (s *Semaphore) TryAcquire(ctx context.Context) (*Lease, error)
func (s *Semaphore) Acquire(ctx context.Context) (*Lease, error)

type Lease struct { /* immutable owner token */ }

func (lease *Lease) Key() string
func (lease *Lease) OwnerToken() btredis.OwnerToken
func (lease *Lease) Release(ctx context.Context) (bool, error)
```

각 성공적인 permit 획득은 새 `btredis.OwnerToken`을 생성하고, 그 token을
sorted-set member로 사용한다. 같은 caller가 여러 permit을 획득해도 member가
겹치지 않는다. permit 수가 `1` 이상이어야 하며, `TryAcquire`가 bounded
capacity에 도달하면 `ErrNotAcquired`를 반환한다.

### Semaphore 원자성

획득 script는 Redis server time을 기준으로 다음을 원자적으로 수행한다.

1. `ZREMRANGEBYSCORE leases -inf now`로 만료 member를 제거한다.
2. 현재 `ZCARD leases`가 `Permits` 이상이면 `{0}`을 반환한다.
3. owner token을 `ZADD leases (now + ttl) ownerToken`으로 추가한다.
4. 성공 시 `{1}`을 반환한다.

release script는 정확한 owner token member만 `ZREM`한다. 따라서 만료 후 같은
key에 새 permit이 발급돼도 오래된 lease가 새 member를 제거할 수 없다.
만료 member의 정리는 다음 acquire 시 수행되며, idle 상태에서 별도 cleanup
goroutine을 만들지 않는다.

semaphore lease에는 fencing token이 없다. TTL이 지난 작업은 여전히 실행될
수 있고 새 permit과 overlap할 수 있으므로, caller는 critical section을
TTL보다 짧게 제한하거나 외부 resource의 별도 version/ownership 검증을
사용해야 한다.

## Context, retry, 오류 계약

- `ctx == nil`은 기존 Redis primitive와 일관되게 `context.Background()`로
  정규화한다. 이미 취소된 context는 Redis를 호출하지 않는다.
- `TryAcquire`는 대기하지 않는다. `Acquire`는 timer 기반 bounded backoff와
  `select { case <-ctx.Done(): ... }`를 사용한다.
- `Release`는 호출자의 cleanup context를 사용한다. cleanup 중 deadline이
  만료되면 mutation 결과가 확정되지 않을 수 있으므로 원인 오류와
  `btredis.ErrCommitUnknown`을 함께 보존한다.
- provider error의 `errors.Is`/`errors.As` 탐색 가능성은 유지하되, error
  string에는 raw Redis key, owner token, fencing token을 넣지 않는다.
- validation 오류는 dispatch 전에 반환하며, invalid client/key/TTL/permit은
  package operation label을 가진 `btredis.OpError` 또는 shared sentinel을
  사용한다.

## 예제 계약

각 package README와 examples는 다음 세 가지 사용 위험을 코드로 보여준다.

### Stale owner rejection

```go
if lease.FencingToken() <= resource.LastFencingToken {
    return ErrStaleOwner
}
resource.LastFencingToken = lease.FencingToken()
```

resource가 마지막 fencing token을 저장·비교하지 않으면 `FencedLock`만으로
stale holder가 외부 DB나 API를 보호한다고 주장하지 않는다.

### Cleanup timeout

```go
cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()
_, _ = lease.Release(cleanupCtx)
```

업무 context가 이미 취소된 뒤에는 release를 같은 context로 강제하지 않고,
명시적인 짧은 cleanup context를 사용한다. `ErrCommitUnknown`이면 외부
관찰/재시도를 통해 실제 Redis 상태를 확인한다.

### Over-TTL overlap

```text
TTL은 자동 만료 경계이지 critical section의 실행 시간 연장이나 watchdog가
아니다. holder가 TTL 이후에도 실행되면 새 holder와 overlap할 수 있다.
FencedLock은 외부 resource의 token 비교가 이 overlap을 거부할 때만 안전하다.
```

## 검증 전략

### Unit/fake 검증

- `Options` validation: blank key, invalid TTL, zero/negative `Permits`, nil
  client.
- 이미 취소된 context에서 Redis dispatch가 없는지 확인한다.
- fake script 결과 `{0,0}`, `{1,fence}`, malformed result, provider error를
  각각 확인한다.
- `TryAcquire`는 busy 상태에서 즉시 `ErrNotAcquired`를 반환하고,
  `Acquire`는 context deadline까지 기다린다.
- `OpError`가 raw key/token/fence를 노출하지 않으며 원인 error를
  `errors.Is`/`errors.As`로 찾을 수 있는지 확인한다.

### Redis Testcontainers 검증

- Fenced lock acquire/release, contention, owner mismatch, expiry.
- fencing token monotonicity: expiry 후 재획득 token이 이전보다 큰지 확인.
- stale owner example: 낮은 token을 가진 resource write를 거부.
- semaphore permit accounting, capacity exhaustion, lease expiry cleanup,
  cancellation path의 permit leakage, owner mismatch, release idempotency.
- same-key concurrent stress와 `Acquire` cancellation/deadline.
- Redis Cluster key-slot 전제는 key layout unit test와 provider fixture에서
  structural key가 같은 hash tag를 쓰는지 확인한다.

검증 명령은 다음을 최소 기준으로 삼는다.

```bash
go test -p 1 -count=1 ./redis/lock ./redis/semaphore
go test -p 1 -race -count=1 ./redis/lock ./redis/semaphore
go test -p 1 -count=1 ./redis ./lock/redis
make fmt-check
make tidy-check
make vet
make lint
make ci
```

Testcontainers-backed package는 Docker resource와 port 충돌을 피하기 위해
순차 실행한다.

## 위험과 완화

| 위험 | 심각도 | 완화 |
|---|---|---|
| fencing counter를 실수로 TTL 처리해 token이 감소한다. | P0 | counter key는 별도 영속 key로 두고 monotonicity integration test를 추가한다. |
| TTL 만료 후 stale holder와 새 holder가 overlap한다. | P0 | resource-side fencing compare 예제와 over-TTL caveat를 README에 포함하고, lock 자체가 외부 보호를 주장하지 않는다. |
| context deadline 뒤 mutation commit 여부가 불명확하다. | P1 | pre-dispatch check, bounded reconciliation, `ErrCommitUnknown`, cleanup example을 함께 적용한다. |
| semaphore release가 다른 owner의 permit을 제거한다. | P0 | token을 sorted-set member로 사용하고 exact-member `ZREM`만 허용한다. |
| multi-key Lua가 Redis Cluster에서 cross-slot 오류를 낸다. | P1 | 모든 structural key에 동일 digest hash tag를 사용하고 key-slot test를 추가한다. |
| 대기 loop가 goroutine/worker를 누수한다. | P1 | timer/select 기반 loop만 허용하고 shutdown API나 watchdog를 만들지 않는다. |

## 완료 조건

- [ ] `redis/lock`와 `redis/semaphore`의 public API와 위 invariant가 구현된다.
- [ ] fencing token, expiry, owner-safe release, semaphore permit accounting,
      cancellation/deadline, idempotency 테스트가 통과한다.
- [ ] stale owner rejection, cleanup timeout, over-TTL overlap examples가
      양쪽 README와 compile-checked examples에 반영된다.
- [ ] 기존 `lock/redis` 계약과 전체 repository 테스트가 회귀하지 않는다.
- [ ] `make ci`와 targeted race test 결과가 PR DoD에 기록된다.

## 참고

- Redis `EVAL`: <https://redis.io/docs/latest/commands/eval/>
- Redis `INCR`: <https://redis.io/docs/latest/commands/incr/>
- Redis `TIME`: <https://redis.io/docs/latest/commands/time/>
- Redis sorted sets: <https://redis.io/docs/latest/develop/data-types/sorted-sets/>
- Redis `ZADD`: <https://redis.io/docs/latest/commands/zadd/>
