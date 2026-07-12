# Issue #527 Provider Conformance Suites Design

## 배경

Issue #527은 SQL, etcd 및 이후 backend를 추가하기 전에 leader, lock, rate limiter의
공통 동작을 재사용 가능한 contract test로 고정한다. 현재 각 provider는 package-local
테스트로 유사한 동작을 검증하지만 공통 runner가 없어 cancellation, lease ownership,
stale state, duplicate admission 및 동시성 의미가 달라질 수 있다.

현재 저장소에서 확인한 기반은 다음과 같다.

- `leader.Elector`는 `Campaign`, `Resign`, `IsLeader`, `Leader`를 제공한다.
- Mongo single elector의 `Campaign`은 leadership을 얻거나 context가 끝날 때까지
  재시도하지만 Redis single elector는 첫 contention에서 `leader.ErrNotLeader`를
  반환한다. Redis/Mongo group elector는 context 기반 대기 계약을 이미 사용한다.
- `lock/redis`는 `Mutex.TryLock`과 owner-bound `Lease.Unlock`을 제공하지만 root
  `lock` production interface는 없다.
- `ratelimit.Limiter`는 local 및 Redis 구현이 공유하는 좁은 `Allow` interface다.
- 기존 provider 테스트는 acquire, renewal, expiry, stale owner, cancellation 및
  contention을 반복해서 검증하고 `testing/concurrency` helper를 사용한다.
- `bluetape4k-leader`는 backend별 선택적 capability가 아니라 shared test fixture를
  각 backend가 동일하게 실행하는 contract 방식을 사용한다. Kotlin test 상속 구조를
  Go로 복제하지 않고 factory와 runner로 치환한다.
- #501 연구 gate는 provider 확장 전에 conformance suite를 만들고 provider-specific
  caveat를 숨기지 않되 최저 공통분모 abstraction으로 의미를 약화하지 말 것을 요구한다.

관련 근거:

- GitHub issues #501, #527, #528, #529, #537
- `leader/{elector,options,errors}.go`
- `leader/{redis,mongo}/elector.go` 및 provider tests
- `lock/redis/{mutex,options}.go` 및 `mutex_test.go`
- `ratelimit/{result,options,token_bucket}.go`
- `ratelimit/redis/{limiter,options}.go`
- `bluetape4k-leader/leader-core/src/testFixtures/.../contract`
- `bluetape4k-wiki/research/2026-07-09-bluetape-go-ecosystem-parity-research-gate.md`

## 목표

- 모든 leader provider가 같은 acquisition, renewal, resign, expiry, cancellation,
  duplicate campaign 및 stale-owner 계약을 실행하게 한다.
- 모든 lock provider가 같은 immediate acquire/release, expiry, cancellation,
  ownership mismatch 및 contention 계약을 실행하게 한다.
- 모든 rate limiter가 같은 token bucket, burst, refill, cancellation, time-source 및
  concurrent admission 계약을 실행하게 한다.
- 각 contract package가 외부 service 없이 스스로 검증 가능한 in-memory reference
  fixture를 제공한다.
- 기존 Redis, Mongo 및 local provider가 새 runner를 실제로 실행하게 하며 이후
  SQL/etcd provider가 같은 진입점을 사용하게 한다.
- public helper API, examples, README 및 localized README가 동일한 사용 계약을 설명하게
  한다.

## 비목표

- 새 production backend 추가
- Kotlin/JVM API 또는 abstract test class의 기계적 이식
- backend client, pool, schema 또는 container lifecycle의 공통 abstraction
- provider별 저장 형식, retry cadence, clock source 또는 운영 topology 은폐
- benchmark 또는 provider 성능 순위 주장
- group/strategic leader election의 전체 conformance 확장
- production lock root interface 신설
- 기존 provider 의미를 선택적 capability flag로 면제하거나 약화

## 검토한 접근

### 접근 1: 필수 factory와 공개 contract runner

각 test helper package는 provider-specific factory를 받고 모든 contract case를
필수로 실행한다. Backend test는 client/container와 concrete type adapter만 제공한다.
공통 결과를 강제하면서 production provider type을 새 interface에 억지로 맞추지 않는다.
이 접근을 채택한다.

### 접근 2: configurable `Suite`와 capability flags

Provider가 clock, wait, stale-owner 같은 capability를 선언하고 일부 case를 끌 수 있다.
현재 차이를 수용하기 쉽지만 새 provider가 어려운 contract를 비활성화해 drift를
고착할 수 있다. 사용자가 요청한 provider 통일 및 `bluetape4k-leader` fixture 방식과
맞지 않아 제외한다.

### 접근 3: backend-local shared helper 또는 테스트 복제

Public API를 만들지 않아도 되지만 SQL/etcd provider가 같은 contract를 재사용하지
못하고 package별 의미가 다시 갈라진다. #501과 #527의 목적을 충족하지 않아 제외한다.

## Architecture

### `leader/leadertest`

`leadertest`는 `leader.Elector` factory를 받아 다음 black-box 계약을 실행한다.

- empty state에서 acquire하고 `IsLeader` 및 `Leader` 관측값을 검증한다.
- 동일 instance의 concurrent/duplicate `Campaign`은 `leader.ErrAlreadyLeader`로
  거절되며 기존 ownership을 잃지 않는다.
- 다른 owner가 살아 있는 동안 contender는 대기하고 context cancellation/deadline을
  `errors.Is`로 보존한다.
- cancellation 반환 뒤 contender가 늦게 acquire하지 않는다.
- renewal이 원래 lease를 넘겨 active owner의 takeover를 막는다.
- renewal이 없는 expired lease는 다른 owner가 takeover할 수 있다.
- `Resign`은 idempotent하고 owner key만 제거한다.
- expired first owner의 stale `Resign`은 새 owner를 제거하지 않는다.
- bounded contention에서 동시에 active한 leader는 정확히 하나다.

Public shape는 다음을 기준으로 plan에서 compile-check한다.

```go
type Factory func(t testing.TB, options leader.Options) (leader.Elector, error)

func Run(t *testing.T, factory Factory)
func MemoryFactory() Factory
```

`Run`은 개별 contract를 named subtest로 실행한다. Factory와 반환 elector는 nil일 수
없다. Factory construction failure는 해당 subtest를 즉시 실패시킨다. Runner에는
skip/capability option을 제공하지 않는다.

Memory fixture는 group별 lease record와 owner token을 mutex로 보호한다. Renewal worker와
timer의 소유권, cancel 및 cleanup path를 명시하고 production package로 노출하지 않는다.
Fixture는 reference contract를 증명하기 위한 test utility이며 backend 동작을 대신
검증하지 않는다.

### `lock/locktest`

Go에는 공통 production lock interface가 없고 concrete provider의 acquire 반환 type도
다르므로 test-only function adapter를 사용한다.

```go
type Config struct {
    Key   string
    Owner string
    TTL   time.Duration
}

type ReleaseFunc func(context.Context) (bool, error)
type AcquireFunc func(context.Context) (ReleaseFunc, error)
type Factory func(t testing.TB, config Config) (AcquireFunc, error)

func Run(t *testing.T, factory Factory)
func MemoryFactory() Factory
```

Factory는 같은 key와 다른 owner로 여러 mutex를 만들 수 있어야 한다. Runner는 acquire,
contention rejection, owner release, repeated release, expiry takeover, pre-canceled acquire,
pre-canceled release, stale release 및 exact-one-owner stress를 모두 실행한다. Provider의
공개 sentinel은 adapter에서 원형을 유지하고 runner는 공통 observable outcome과
`context.Canceled`/`DeadlineExceeded`만 강제한다.

Memory fixture는 owner와 expiry를 mutex로 보호하고 release 시 owner를 compare한다.
Background goroutine은 사용하지 않으며 operation 시각에 expiry를 판정해 deterministic
cleanup을 제공한다.

### `ratelimit/ratelimittest`

Rate limiter는 이미 `ratelimit.Limiter`를 공유하므로 neutral configuration factory만
필요하다.

```go
type Config struct {
    RatePerSecond float64
    Burst         int64
    IdleTTL       time.Duration
}

type Factory func(t testing.TB, config Config) (ratelimit.Limiter, error)

func Run(t *testing.T, factory Factory)
func MemoryFactory() Factory
```

Runner는 initial burst, over-burst validation, rejection result, refill, key isolation,
pre-canceled call 및 concurrent callers의 exact admission total을 검증한다. `Requested`,
`Remaining`, `RetryAfter`, `ResetAfter`의 공통 invariants를 검사하되 provider clock의
절대 timestamp나 millisecond rounding은 강제하지 않는다.

Local clock과 Redis server clock은 운영상 다른 source지만 observable token bucket
contract는 같다. Refill test는 bounded duration과 eventual assertion을 사용한다. Fake
fixture는 controllable clock으로 runner 자체의 boundary case를 추가 검증하지만 provider
runner를 fake로 대체하지 않는다.

## Leader Contract Unification

`leader.Elector.Campaign(ctx)`의 공통 의미를 다음과 같이 고정한다.

1. leadership을 얻으면 nil을 반환하고 renewal을 시작한다.
2. 다른 owner가 살아 있으면 backend-specific bounded cadence로 재시도한다.
3. caller context가 취소되거나 deadline에 도달하면 그 오류를 `errors.Is` 가능한 형태로
   반환한다.
4. context 종료를 반환한 뒤 acquire, renewal worker 또는 owner record가 남지 않는다.
5. 같은 elector instance가 이미 owner이거나 campaign 중이면 즉시
   `leader.ErrAlreadyLeader`를 반환한다.

Redis single elector를 이 계약으로 변경한다. Mongo single 및 Redis/Mongo group의
기존 context-wait 동작과 정렬되며 향후 etcd `Election.Campaign` 의미와도 일치한다.
Retry cadence는 public timing guarantee가 아닌 provider 운영 세부사항이다.

`leader.ErrNotLeader`는 source compatibility를 위해 유지한다. 새 single-elector
contention의 정상 반환값으로는 사용하지 않으며 GoDoc과 README에서 legacy sentinel임을
명시한다. 이 issue에서 symbol을 제거하거나 다른 실패에 재사용하지 않는다.

## Data Flow And Ownership

### Contract runner

1. Runner가 test별 unique key/group/member를 만든다.
2. Factory가 caller-owned backend fixture를 사용해 provider instance를 만든다.
3. Runner가 public API만 호출해 동작을 관측한다.
4. 각 획득 뒤 `t.Cleanup`에 bounded cleanup을 즉시 등록한다.
5. Cancellation test는 operation 종료와 backend side effect 부재를 모두 확인한다.
6. Container/client lifecycle은 provider test가 소유하며 runner는 닫지 않는다.

### Redis leader campaign

1. nil context를 기존 관례대로 `context.Background()`로 정규화한다.
2. pre-canceled context와 duplicate campaign을 backend call 전에 거절한다.
3. atomic acquire를 시도한다.
4. 성공하면 renewal worker를 시작하고 반환한다.
5. contention이면 cancel-aware bounded wait 후 다시 시도한다.
6. provider error는 기존 redacted `redis.OpError` 계약을 유지한다.
7. context 종료 또는 provider failure 시 campaigning state를 반드시 해제한다.

## Error Contract

- Leader duplicate instance: `leader.ErrAlreadyLeader`
- Leader waiting cancellation: `context.Canceled` 또는 `context.DeadlineExceeded`
- Lock contention: provider sentinel을 유지하되 release callback과 side effect는 nil
- Rate-limit rejection: error가 아닌 `ratelimit.Result{Allowed:false}`
- Invalid input: 기존 provider validation error 유지
- Provider failure: 기존 typed/redacted wrapper와 `errors.Is`/`errors.As` 유지

Conformance runner는 formatted provider error 문자열을 비교하지 않는다. Sentinel,
context 원인, boolean/result state 및 backend-independent side effect만 검사한다.

## Concurrency And Cleanup

- 모든 stress case는 bounded worker/round/timeout을 사용한다.
- Leader는 simultaneous active count의 최대값이 정확히 1임을 검증한다.
- Lock은 동일 key에서 successful owner가 겹치지 않고 모든 successful release가 owner
  record를 제거했음을 검증한다.
- Rate limiter는 refill이 없는 한 한 burst window의 successful token 총합이 정확히
  `Burst`를 넘지 않음을 검증한다.
- Race test는 helper fixture와 실제 provider를 모두 포함한다.
- Runner가 생성한 goroutine, timer 및 channel은 test 종료 전에 닫힌다.
- Testcontainers-backed provider package는 `-p 1` 또는 명시적 직렬 command로 실행한다.

## Public Documentation

새 public helper package마다 다음을 제공한다.

- English GoDoc과 constructor/runner contract
- compile-checked `ExampleRun`
- `README.md` 및 `README.ko.md` language switch와 동등한 내용
- factory adapter 예제와 provider-owned fixture lifecycle 설명

`leader/README.md`와 `leader/README.ko.md`는 blocking `Campaign` semantics와 legacy
`ErrNotLeader` 상태를 반영한다. 공개 behavior 변경은 `CHANGELOG.md`의 다음 release
section에 기록한다. 새 module/dependency/workflow/catalog 등록은 없다. 단순 runner 사용
흐름은 코드 예제로 충분하므로 새 diagram은 만들지 않는다.

## Failure Modes And Guards

1. **취소 후 늦은 획득**: campaign loop가 context 종료와 acquire 사이에서 race하면
   caller가 실패를 받은 뒤 owner record가 생길 수 있다. 각 retry 전후 context를
   확인하고 conformance test가 owner 부재를 검증한다.
2. **stale owner 삭제**: expired owner의 cleanup이 새 owner record를 지우면 leadership이
   손실된다. 모든 release/resign은 owner token compare-and-delete이며 takeover 뒤 stale
   cleanup test를 필수로 실행한다.
3. **renewal worker 누수**: resign/cancellation이 ticker/goroutine을 남기면 race와 backend
   traffic이 지속된다. cancel/done ownership을 한 instance에 두고 bounded cleanup 및 race
   test로 검증한다.
4. **시간 기반 flaky test**: provider clock source와 scheduler 지연이 refill/expiry assertion을
   흔들 수 있다. fake fixture는 controllable clock을 사용하고 실제 provider는 여유 있는
   lease window와 `Eventually`를 사용하되 timeout을 제한한다.
5. **느슨한 adapter로 false PASS**: adapter가 다른 key/backend instance를 만들면 isolation
   test가 거짓 통과할 수 있다. Factory 문서와 self-test는 같은 fixture/namespace 공유를
   요구하고 identity/key-isolation case를 포함한다.
6. **contract 우회**: optional flags나 skip이 provider drift를 숨길 수 있다. Public runner는
   capability option을 제공하지 않으며 unsupported contract는 해당 provider 실패다.

## Compatibility And Migration

- 기존 public method와 constructor signature를 유지한다.
- Redis single elector contention은 즉시 `ErrNotLeader`에서 context-controlled wait로
  변경된다. 이는 의도적인 behavior migration이며 README, GoDoc, CHANGELOG에 기록한다.
- 짧은 one-shot 동작이 필요한 caller는 짧은 timeout context를 전달한다.
- `ErrNotLeader` symbol은 제거하지 않아 source compatibility를 보존한다.
- Redis key bytes, owner token, lease TTL, compare-and-delete 및 error redaction contract는
  변경하지 않는다.
- Mongo behavior와 storage schema는 변경하지 않고 새 shared runner를 적용한다.
- Rate limiter 결과 rounding의 기존 provider-specific 정밀도는 허용하되 공통 invariants는
  동일하게 강제한다.

## Acceptance Criteria

1. `leader/leadertest`, `lock/locktest`, `ratelimit/ratelimittest`가 public factory runner와
   in-memory reference fixture를 제공한다.
2. 각 runner에는 capability skip 없이 named contract cases가 있다.
3. Redis와 Mongo single leader가 동일 `leadertest.Run`을 통과한다.
4. Redis single `Campaign`이 acquire 또는 context 종료까지 대기하며 취소 뒤 늦은
   ownership을 남기지 않는다.
5. Redis lock이 동일 `locktest.Run`을 통과한다.
6. Local 및 Redis rate limiter가 동일 `ratelimittest.Run`을 통과한다.
7. Fake fixtures와 실제 provider에서 cancellation, expiry/stale ownership 및 bounded
   concurrency evidence가 있다.
8. 기존 provider typed/sentinel error, key bytes, TTL 및 owner-token semantics가 유지된다.
9. 모든 public helper에 GoDoc, compile-checked example, English/Korean README parity가 있다.
10. `go test -p 1 -count=1 ./leader/... ./lock/... ./ratelimit/...`와 해당 package race tests,
    `make ci`가 통과한다.
11. Spec/plan/pre-PR/post-PR 7-Tier review가 각각 P0=0/P1=0으로 수렴한다.
12. PR은 #527, milestone 0.19.0, assignee `debop` 및 관련 label을 반영하고 명시적 merge
    승인 전 대기한다.

## Definition Of Done

- 공통 contract runner가 기존 및 reference provider에 실제 적용됨
- Leader provider의 contention/cancellation semantics가 통일됨
- 취소, lifecycle, stale ownership 및 exact contention 결과가 race test로 증명됨
- English/Korean public documentation과 examples가 source와 일치함
- 새 dependency, module registration, workflow 변경 및 diagram은 근거 있는 N/A
- P0=0, P1=0, checklist Blocked=0
- Live PR CI/review가 green이고 merge는 사용자 결정 대기
