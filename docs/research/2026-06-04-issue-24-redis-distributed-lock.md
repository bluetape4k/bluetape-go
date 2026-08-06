# Issue #24 Redis Distributed Lock 연구

Issue: #24
Milestone: 0.3.0
Date: 2026-06-04
Work type: Type A - Full Feature

## 연구 질문

`bluetape-go`는 Go 관용성을 지키고 Testcontainers로 검증 가능한 형태를 유지하면서,
owner token과 TTL을 사용하는 작은 Redis 분산 락 패키지를 어떻게 노출해야 하는가?

## 결정

`lock/redis` 아래에 package name `redislock`을 사용하는 단일 Redis 인스턴스용 락
패키지를 구현한다.

락 획득 primitive는 다음 명령이다.

```text
SET key token NX PX ttl
```

락 해제 primitive는 저장된 값이 여전히 owner token과 같을 때만 key를 삭제하는 Lua
script다.

## 근거 자료

| Source | Relevant point | Decision impact |
|---|---|---|
| Redis `SET` command docs, https://redis.io/docs/latest/commands/set/ | `SET` supports `NX` and millisecond expiration (`PX`). | Use `go-redis` `SetNX(ctx, key, token, ttl)` for atomic acquire with TTL. |
| Redis distributed lock docs, https://redis.io/docs/latest/develop/clients/patterns/distributed-locks/ | Single-instance acquire uses a random value and release must compare that value before deleting. | Store an owner token and use compare-before-delete Lua unlock. |
| Redis `EVAL` docs, https://redis.io/docs/latest/commands/eval/ | Scripts receive keys through `KEYS` and additional arguments through `ARGV`; accessed keys should be explicit. | Unlock script takes one key and the token as `ARGV[1]`. |
| Local `leader/redis` package | Existing leader election uses `SetNX` plus Lua release/renew with random `memberID:token`. | Reuse the same owner-token and Lua safety pattern without depending on leader semantics. |
| `go-redis/v9` local docs | `Client.SetNX` returns `*BoolCmd`; `Client.Eval` returns `*Cmd`; `redis.Cmdable` already covers both. | Accept `redis.Cmdable` directly, matching existing `leader/redis`. |

## API 방향

```go
package redislock

type Options struct {
    Key   string
    TTL   time.Duration
    Token string
}

type Mutex struct { ... }
type Lease struct { ... }

var ErrNotAcquired = errors.New("redis lock not acquired")

func New(client redis.Cmdable, options Options) (*Mutex, error)
func (m *Mutex) TryLock(ctx context.Context) (*Lease, error)
func (m *Mutex) Key() string
func (l *Lease) Token() string
func (l *Lease) Unlock(ctx context.Context) (bool, error)
```

`Options.Token`은 선택값이다. 비어 있으면 성공한 각 `TryLock` 시도마다 새 random
token을 생성한다. 값을 지정하면 결정적 테스트와 interoperability 실험에 사용할 수
있다.

## 비목표

- 여러 Redis node에 걸친 Redlock quorum은 구현하지 않는다.
- blocking wait/retry loop는 구현하지 않는다.
- #24에서는 TTL renewal/extend를 구현하지 않는다.
- 더 많은 backend가 생기기 전에는 generic lock abstraction을 노출하지 않는다.
- Kotlin/JVM Redis lock key/value interop을 지원 contract로 삼지 않는다.

## 테스트 요구사항

- 같은 key를 두 client가 경합하면 하나만 성공하고 다른 하나는 `ErrNotAcquired`를
  반환한다.
- Unlock은 owner token만 삭제한다.
- expiration 이후이거나 다른 owner가 key를 획득한 뒤의 Unlock은 새 owner를 삭제하지
  않고 `false, nil`을 반환한다.
- TTL expiration 이후에는 다른 owner가 같은 key를 획득할 수 있다.
- context cancellation은 `errors.Is`로 보존된다.
- `GoroutineStressTester`는 동시 경합에서 한 번에 여러 owner를 허용하지 않음을
  증명한다.
- `AsyncJobTester`는 취소된 시도가 깨끗하게 반환되고 key를 남기지 않음을 증명한다.

## 운영 경계

이 락은 단일 Redis 인스턴스에서 mutual exclusion을 제공하는 작은 Redis primitive다.
TTL 기반 누수 복구만으로 충분한 low/medium risk service coordination에 적합하다.
fencing token은 제공하지 않으며, TTL이 만료된 뒤에도 계속 작업하는 client로부터 외부
system을 보호하지 않는다.
