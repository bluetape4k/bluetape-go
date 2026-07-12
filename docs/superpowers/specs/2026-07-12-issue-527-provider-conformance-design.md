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

- 모든 single `leader.Elector` provider가 같은 acquisition, renewal, resign, expiry,
  cancellation, duplicate campaign 및 stale-owner 계약을 실행하게 한다.
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
- `GroupElector`와 `StrategicElector` conformance 확장. 이 issue는 single
  `leader.Elector`만 고정하고 두 interface는 후속 작업으로 명시적으로 defer한다.
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

`leadertest`는 `leader.Elector` factory와 필수 backend control을 받아 다음 black-box
계약을 실행한다.

- empty state에서 acquire하고 `IsLeader` 및 `Leader` 관측값을 검증한다.
- 동일 instance의 concurrent/duplicate `Campaign`은 `leader.ErrAlreadyLeader`로
  거절되며 기존 ownership을 잃지 않는다.
- 다른 owner가 살아 있는 동안 contender는 대기하고 context cancellation/deadline을
  `errors.Is`로 보존한다.
- cancellation 반환 뒤 contender가 늦게 acquire하지 않는다.
- renewal이 원래 lease를 넘겨 active owner의 takeover를 막는다.
- backend renew error와 owner-token mismatch 모두 `IsLeader=false`, renewal worker 종료,
  추가 renew traffic 부재 및 lease expiry 뒤 takeover로 이어진다.
- renewal이 없는 expired lease는 다른 owner가 takeover할 수 있다.
- `Resign`은 idempotent하고 owner key만 제거하며 cancellation/provider failure 뒤
  cleanup-pending state를 보존해 다음 `Resign`이 삭제를 재시도한다.
- expired first owner의 stale `Resign`은 새 owner를 제거하지 않는다.
- bounded contention에서 동시에 active한 leader는 정확히 하나다.

Public shape는 다음을 기준으로 plan에서 compile-check한다.

```go
type Operation string

const (
    OperationRenew  Operation = "renew"
    OperationResign Operation = "resign"
)

type Factory func(t testing.TB, options leader.Options) (leader.Elector, error)

type Control interface {
    ReplaceOwner(ctx context.Context, options leader.Options, owner string) error
    FailNext(ctx context.Context, options leader.Options, operation Operation, cause error) error
    Owner(ctx context.Context, options leader.Options) (string, error)
    OperationCount(options leader.Options, operation Operation) int64
}

type Harness struct {
    New     Factory
    Control Control
}

func Run(t *testing.T, harness Harness)
func MemoryHarness() Harness
```

`Control`은 test-only backend seam이며 production provider API가 아니다. 모든 method는
필수다. Redis adapter는 command interceptor/control client, Mongo adapter는 control
collection/failpoint처럼 backend에 맞는 구현을 사용한다. `OperationCount`는 failure 또는
ownership loss 뒤 worker가 멈춰 추가 traffic이 없음을 bounded window에서 증명한다.

`Run`은 각 case의 `leader.Options`를 한 번 normalize하고 factory와 Control에는 그
normalized value만 전달한다. Control은 normalized `leader.Options` identity를 그대로
사용한다. `ReplaceOwner`는 nonblank structural owner만 허용한다. `FailNext`는 non-nil
cause와 위 두 operation만 허용하고 같은 options identity의 다음 matching operation
정확히 하나에 적용된다. Direct invalid call에서 error-returning Control method는 공통
validation error를 반환하고 side effect를 만들지 않는다. `Owner`도 invalid options를
같이 거절한다. Error를 반환할 수 없는 `OperationCount`는 invalid options/operation에 0을
반환한다. Valid identity의 `OperationCount`는 identity/operation별 concurrency-safe
monotonic cumulative count이며 reset하지 않는다. Runner는 subtest 시작 baseline과 종료
count의 delta만 비교한다.

모든 leader Control context는 non-nil이어야 한다. Nil은 공통 validation error,
pre-canceled/deadline context는 해당 context error를 backend mutation이나 failure injection
arming 전에 반환한다. Runner는 owner와 operation count가 그대로임을 확인한다.

`Run`은 개별 contract를 named subtest로 실행한다. Harness, factory, control 및 반환
elector는 nil일 수 없다. Construction/control failure는 `t.Helper()` attribution으로 해당
named subtest를 즉시 실패시킨다. Runner에는 skip/capability option을 제공하지 않는다.

Memory fixture는 group별 lease record와 owner token을 mutex로 보호하고 모든 control
operation과 deterministic failure injection을 구현한다. Renewal worker와
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

type Operation string
type Phase string
type Gate interface {
    AwaitStarted(context.Context) error
    Resume()
}
type Control interface {
    GateNext(context.Context, Config, Operation, Phase) (Gate, error)
    Owner(context.Context, Config) (string, error)
    OperationCount(Config, Operation) int64
}
type Harness struct {
    New     Factory
    Control Control
}

func Run(t *testing.T, harness Harness)
func MemoryHarness() Harness
```

`Operation`은 acquire/release, `Phase`는 before/after-linearize의 closed constants만
허용한다. `GateNext`는 같은 identity의 다음 matching operation 하나를 해당 boundary에서
멈추고 `Gate`로 started/resume handshake를 제공한다. `Owner`와 cumulative
`OperationCount`는 backend record와 traffic을 검증한다. 이 Control은 test-only이며 모든
provider adapter에 필수다. Control context는 non-nil이어야 하며 nil/pre-canceled/deadline
입력은 gate arming이나 backend mutation 전에 validation/context error를 반환한다.

Factory는 같은 key와 다른 owner로 여러 mutex를 만들 수 있어야 한다. Runner는 acquire,
contention rejection, owner release, repeated release, expiry takeover, pre-canceled acquire,
pre-canceled release, in-flight cancellation, stale release 및 exact-one-owner stress를
모두 실행한다. Provider의 공개 sentinel은 adapter에서 원형을 유지하고 runner는 공통
observable outcome과 `context.Canceled`/`DeadlineExceeded`만 강제한다.

`Config.Key`와 `Config.Owner`는 nonblank이며 byte-for-byte 보존되고 `TTL`은 positive다.
Runner가 factory 호출 전에 이를 검증하며 zero/negative 값에는 factory를 호출하지 않고
named subtest failure를 만든다. 같은 key로 만든 factory 결과는 같은 backend namespace를
공유하고 서로 다른 owner는 서로 다른 stable identity여야 한다.

Acquire/release 결과 계약은 다음과 같다.

| 상황 | `AcquireFunc` 결과 | `ReleaseFunc` 결과 |
|---|---|---|
| 획득 성공 | non-nil release, nil error | 아직 호출하지 않음 |
| contention | nil release, provider sentinel | 해당 없음 |
| validation/context/provider 실패 | nil release, non-nil error | 해당 없음 |
| 현재 owner의 최초 release | 해당 없음 | `true, nil` |
| 이미 release/expiry된 owner | 해당 없음 | `false, nil` |
| stale owner mismatch | 해당 없음 | `false, nil` |
| cancellation/provider failure | 해당 없음 | `false, non-nil error` |

Compile-checked examples는 성공뿐 아니라 contention, repeated release, stale owner 및
cancellation 결과를 보여준다. Nil factory, nil acquire/release function 및 factory error는
leader helper와 같은 즉시 named-subtest failure 계약을 사용한다.

Acquire/release의 context 결과는 side-effect linearization과 일치해야 한다. Before-linearize
gate에서 context가 끝나면 context error, nil/false 결과를 반환하고 owner 및 operation
side effect가 없어야 한다. After-linearize gate까지 성공한 뒤 context가 끝나면 acquire는
non-nil release와 nil error, release는 실제 compare-and-delete 결과와 nil error를 반환한다.
Runner는 두 phase를 deterministic gate로 재현하고 owner probe, operation-count delta 및
takeover로 late side effect가 없음을 검증한다.

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

type Phase string
type Gate interface {
    AwaitStarted(context.Context) error
    Resume()
}
type Control interface {
    GateNext(context.Context, string, Phase) (Gate, error)
    OperationCount(string) int64
}
type Harness struct {
    New     Factory
    Control Control
}

func Run(t *testing.T, harness Harness)
func MemoryHarness() Harness
```

`Phase`는 before/after-linearize의 closed constants다. `GateNext`는 key의 다음 `Allow` 하나를
해당 boundary에서 멈추며, cumulative `OperationCount`와 이후 public `Allow` result가 quota
side effect를 증명한다. 이 test-only Control은 모든 provider adapter에 필수다. Control
context는 non-nil이어야 하며 nil/pre-canceled/deadline 입력은 gate arming 전에
validation/context error를 반환한다.

`RatePerSecond`는 positive finite, `Burst`는 positive, `IdleTTL`은 non-negative다.
`IdleTTL==0`은 provider의 bounded default를 선택하고 positive 값은 full-refill duration
이상이어야 한다. Runner가 invalid/boundary config를 table-driven self-test로 검증하며 nil
factory, nil limiter 및 factory error는 즉시 named-subtest failure다. Config 단위는
tokens/second, tokens 및 `time.Duration`이다.

Runner는 initial burst, over-burst validation, rejection result, refill, key isolation,
pre-canceled call, in-flight cancellation 및 concurrent callers의 exact admission total을
검증한다. Before-linearize gate에서 context가 끝나면 context error가 quota를 소비하지
않고, 이어지는 full-burst request가 전부 허용되어야 한다. After-linearize gate까지 token
소비가 성공한 뒤 context가 끝나면 successful `Result`와 nil error를 반환해야 하며 다음
request가 정확히 그 소비량을 반영해야 한다. Runner는 operation-count delta와 refill이 없는
bounded window에서 이를 검증한다. `Requested`, `Remaining`, `RetryAfter`, `ResetAfter`의
공통 invariants를 검사하되 provider clock의 절대 timestamp나 millisecond rounding은
강제하지 않는다.

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
4. `Campaign`, `Resign`, `Leader`의 nil context는 backend call과 state mutation 전에 새
   `leader.ErrInvalidContext`로 거절한다.
5. context 종료를 반환한 뒤 acquire, renewal worker 또는 owner record가 남지 않는다.
   Acquire success가 먼저 linearize되면 nil을 반환하며, context error를 반환하는 path는
   contender token 부재를 control probe로 증명한다.
6. 같은 elector instance가 이미 owner이거나 campaign 중이거나 cleanup pending이면 즉시
   `leader.ErrAlreadyLeader`를 반환한다.

Redis single elector를 이 계약으로 변경한다. Mongo single 및 Redis/Mongo group의
기존 context-wait 동작과 정렬되며 향후 etcd `Election.Campaign` 의미와도 일치한다.
Retry cadence는 public timing guarantee가 아닌 provider 운영 세부사항이다.
Redis single elector는 25ms base, 250ms cap의 exponential delay와 owner-token 기반
deterministic ±20% jitter를 사용한다. Zero-delay polling은 금지하고 모든 timer는 context로
취소한다. 1초 contention window의 acquire command 수는 12회를 넘지 않아야 한다.

`leader.ErrNotLeader`는 source compatibility를 위해 유지한다. 새 single-elector
contention의 정상 반환값으로는 사용하지 않으며 GoDoc과 README에서 legacy sentinel임을
명시한다. 이 issue에서 symbol을 제거하거나 다른 실패에 재사용하지 않는다.

## Data Flow And Ownership

### Contract runner

1. Runner가 test별 unique key/group/member를 만든다.
2. Factory가 caller-owned backend fixture를 사용해 provider instance를 만든다.
3. 정상 동작은 public API만 호출해 관측한다. Fault injection, operation count 및 backend
   side-effect/owner probe만 필수 test-only `Control`을 사용한다.
4. 각 획득 뒤 `t.Cleanup`에 bounded cleanup을 즉시 등록한다.
5. Cancellation test는 operation 종료와 backend side effect 부재를 모두 확인한다.
6. Container/client lifecycle은 provider test가 소유하며 runner는 닫지 않는다.

### Redis leader campaign

1. nil context를 공통 validation error로 backend call 전에 거절한다.
2. pre-canceled context와 duplicate/cleanup-pending campaign을 backend call 전에 거절한다.
3. atomic acquire를 시도한다.
4. 성공하면 renewal worker를 시작하고 반환한다.
5. contention이면 cancel-aware bounded wait 후 다시 시도한다.
6. provider error는 기존 redacted `redis.OpError` 계약을 유지한다.
7. context 종료 또는 provider failure 시 campaigning state를 반드시 해제한다.

### Leader resign state machine

1. Pre-canceled context는 renewal과 ownership state를 바꾸기 전에 반환한다.
2. 유효한 resign이 시작되면 local `owned=false`, `cleanupPending=true`로 전환하고 renewal을
   취소한 뒤 bounded하게 worker 종료를 기다린다.
3. Compare-and-delete 성공 또는 owner mismatch는 cleanup 완료이며 pending state를 지운다.
4. Worker-stop deadline 또는 backend delete failure는 pending state, in-flight renewal
   state와 owner token을 유지한다. 이후 `Resign`은 worker wait/delete를 재시도하고 성공
   전까지 같은 instance의 `Campaign`을 거절한다.
5. Deadline 반환 뒤 새 renewal attempt는 schedule하지 않는다. 이미 linearize된 in-flight
   renew는 owned `RenewInterval` timeout 안에 최대 한 번 완료해 lease를 연장할 수 있다.
   Worker 종료 뒤 추가 renew는 0회이며 backend record는 retry 성공 또는 마지막 연장 lease
   expiry로 사라진다. 새 owner의 record는 삭제하지 않는다.
6. Runner는 pre-cancel, injected blocked/late renew, worker-stop deadline, injected delete
   failure, retry success, exact late-renew upper bound 및 eventual takeover를 검증한다.

## Error Contract

- Leader duplicate instance: `leader.ErrAlreadyLeader`
- Leader waiting cancellation: `context.Canceled` 또는 `context.DeadlineExceeded`
- Leader nil context: `leader.ErrInvalidContext`, backend side effect 없음
- Lock contention: provider sentinel을 유지하되 release callback과 side effect는 nil
- Rate-limit rejection: error가 아닌 `ratelimit.Result{Allowed:false}`
- Invalid input: 기존 provider validation error 유지
- Provider failure: 기존 typed/redacted wrapper와 `errors.Is`/`errors.As` 유지

모든 single leader provider failure는 새 `*leader.OperationError`를 outer contract로
사용한다.

```go
type OperationError struct { /* private state */ }

func NewOperationError(backend string, operation string, cause error) error
func (e *OperationError) Error() string
func (e *OperationError) Unwrap() error
func (e *OperationError) Backend() string
func (e *OperationError) Operation() string
```

Valid input에서 constructor는 `*OperationError`를 `error`로 반환한다. `Error()`는 stable
low-cardinality backend/operation만 출력하고 cause text를 호출하지
않는다. `Unwrap()`은 `errors.Is`/`errors.As` inspection을 보존한다. Redis는 기존
`*redis.OpError`를 cause로 감싸 기존 caller의 `errors.As`를 유지하고 Mongo는 driver
cause를 감싸 raw command, namespace, endpoint 및 credential text를 숨긴다. Nil cause와
blank/control/32-byte 초과 backend/operation에는 validation error를 반환하며 provider는
package-owned constants만 전달한다.

공개 concrete type의 constructor 우회를 안전하게 만들기 위해 zero value와 nil receiver도
명시한다. `(*OperationError)(nil).Error()`와 zero-value `Error()`는 cause나 동적 값을
포함하지 않는 generic `"leader operation failed"`를 반환한다. 두 경우 `Unwrap()`은 nil,
`Backend()`와 `Operation()`은 각각 stable fallback `"unknown"`을 반환한다. 정상 생성된
값의 accessor만 validated package-owned label을 반환한다. 이 동작과 cause의 `Error()`가
호출되지 않는다는 사실을 unit test로 고정한다.

Conformance runner는 formatted provider error 문자열을 비교하지 않는다. Sentinel,
context 원인, boolean/result state 및 backend-independent side effect만 검사한다. Runner
failure diagnostics는 raw key/group/member/owner token, endpoint, credential, payload 및 raw
provider text를 출력하지 않고 stable case name과 redacted identifier만 사용한다. 각 실제
provider의 injected error test는 `errors.Is`/`errors.As`를 보존하면서 forbidden marker가
rendered error와 captured test output에 없음을 검증한다.

## Concurrency And Cleanup

- 모든 stress case는 bounded worker/round/timeout을 사용한다.
- 모든 contention test는 start barrier로 caller를 동시에 release한다.
- Test-only `Gate.Resume`은 idempotent/non-blocking이고 runner는 gate를 얻자마자
  `t.Cleanup(gate.Resume)`을 등록해 실패 path에서도 blocked operation을 해제한다.
- Leader는 `successes==1`, `maxActive==1`, 나머지 contender는 bounded context 종료이며
  release 뒤 정확히 한 contender가 takeover함을 검증한다.
- Lock은 `successes==1`, contention sentinel 수 `workers-1`, `maxActive==1`, successful
  release 뒤 정확히 한 takeover를 검증한다.
- Rate limiter는 refill이 일어나지 않는 measured window에서 `allowed==Burst`,
  `rejected==requests-Burst`, consumed token 합계가 정확히 `Burst`임을 검증한다.
- Race test는 helper fixture와 실제 provider를 모두 포함한다.
- Runner가 생성한 goroutine, timer 및 channel은 test 종료 전에 닫힌다.
- Testcontainers-backed provider package는 `-p 1` 또는 명시적 직렬 command로 실행하고
  shared fixture subtest에서 `t.Parallel`을 금지한다. Startup/teardown context를 제한하고
  Redis `PING`/Mongo command probe가 성공한 뒤 runner를 시작한다. Allocation 직후 client
  cleanup을 등록하고 client를 container보다 먼저 닫는다. Factory/subtest failure를 포함해
  unique namespace와 final owner/key absence를 검증한다.

## Public Documentation

새 public helper package마다 다음을 제공한다.

- English GoDoc과 constructor/runner contract
- compile-checked `ExampleRun`
- `README.md` 및 `README.ko.md` language switch와 동등한 내용
- factory adapter 예제와 provider-owned fixture lifecycle 설명
- conformance가 보장하는 항목과 보장하지 않는 retry cadence, clock source, key precision,
  backend lifecycle, group/strategic 범위를 구분한 provider caveat matrix

Examples는 fixture/client 생성, 즉시 cleanup 등록, unique namespace, adapter 구성,
bounded context, construction error 처리 및 `Run` 호출의 전체 흐름을 compile-check한다.

`leader/README.md`와 `leader/README.ko.md`는 blocking `Campaign` semantics와 legacy
`ErrNotLeader` 상태를 반영한다. 공개 behavior 변경은 `CHANGELOG.md`의 0.19.0 대상
`Unreleased` section에 old/new behavior, timeout migration 및 rollback compatibility와 함께
기록한다. 새 module/dependency/workflow/catalog 등록은 없다. 단순 runner 사용
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
7. **defective adapter false PASS**: owner 무시, key rewrite, shared release callback 또는
   fake substitution이 실제 provider를 우회할 수 있다. Runner core의 관측 evaluator를
   분리해 intentionally broken factory가 각각 deterministic failure를 내는 self-test를 둔다.
8. **retry storm**: 동일 cadence contender가 Redis에 synchronized polling을 만들 수 있다.
   Bounded exponential delay, deterministic jitter 및 fixed-window command budget으로 막는다.

## Compatibility And Migration

- 기존 public method와 constructor signature를 유지한다.
- Redis single elector contention은 즉시 `ErrNotLeader`에서 context-controlled wait로
  변경된다. 이는 의도적인 behavior migration이며 README, GoDoc, CHANGELOG에 기록한다.
- 기존 `Campaign(context.Background/TODO)`와 `errors.Is(err, ErrNotLeader)` caller를 release
  전에 audit한다. 새 caller는 업무 lease/latency보다 짧지 않고 서비스 shutdown보다 긴
  명시적 `context.WithTimeout`을 소유하고 cancel을 defer한다. README는 old/new code와
  expected `DeadlineExceeded`를 함께 보여준다.
- `ErrNotLeader` symbol은 제거하지 않아 source compatibility를 보존한다.
- Nil `Campaign`/`Resign`/`Leader` context는 더 이상 background로 정규화하지 않고
  `leader.ErrInvalidContext`를 반환한다.
- `Options.Normalize`는 `Group`과 `MemberID`를 structural segment로, `KeyPrefix`를
  colon-separated structural prefix로 검증한다. Segment는 trim-stable이고 `:`, `{`, `}` 및
  control character를 포함하지 않으며 group/member는 각각 256 bytes, 최종 key는 512 bytes
  이하이다. Unicode bytes는 normalize하지 않아 canonically equivalent text도 서로 다른
  identity로 유지한다. Valid input의 기존 key bytes는 그대로다.
- Redis key bytes, owner token, lease TTL, compare-and-delete 및 error redaction contract는
  valid input에 대해 변경하지 않는다. Collision/whitespace/control/delimiter/oversize case는
  backend call 전에 공통 rejection된다.
- Mongo storage schema와 success/sentinel semantics는 변경하지 않는다. Provider failure는
  raw driver text wrapping에서 공통 sanitized `*leader.OperationError`로 migration하며
  underlying `errors.Is`/`errors.As` inspection은 유지한다.
- Rate limiter 결과 rounding의 기존 provider-specific 정밀도는 허용하되 공통 invariants는
  동일하게 강제한다.

### Rollout and rollback

- 0.19.0 배포 전에 모든 caller의 bounded context와 legacy `ErrNotLeader` branch audit을
  완료한다.
- Canary는 caller-owned low-cardinality campaign wait duration, acquisition success,
  timeout/cancellation 및 provider failure count를 관측한다. Library는 global logger/metric을
  만들지 않으며 raw group/member/token/provider error를 기록하지 않는다.
- Mixed old/new binaries는 같은 key/token/TTL format을 사용하지만 old binary는 즉시
  `ErrNotLeader`, new binary는 context까지 대기한다. 이 차이를 rollout window에 명시한다.
- Rollback 전 service shutdown context로 outstanding campaign을 취소한다. Binary rollback은
  storage format이 같아 안전하며 남은 lease는 owner-aware resign 또는 TTL로 정리된다.

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
8. 승인된 Redis single contention 및 common input-validation migration을 제외하고 기존
   provider typed/sentinel error, valid key bytes, TTL 및 owner-token semantics가 유지된다.
9. 모든 public helper에 GoDoc, compile-checked example, English/Korean README parity가 있다.
10. `go test -p 1 -count=1 ./leader/... ./lock/... ./ratelimit/...`와 해당 package race tests,
    `make ci`가 통과한다.
11. Spec/plan/pre-PR/post-PR 7-Tier review가 각각 P0=0/P1=0으로 수렴한다.
12. PR은 #527, milestone 0.19.0, assignee `debop` 및 관련 label을 반영하고 명시적 merge
    승인 전 대기한다.
13. Renewal/lost-owner 및 resign partial-failure injection이 worker 종료, retry, redaction,
    operation budget과 eventual takeover를 증명한다.
14. Release 문서에 caller audit, mixed-version canary, monitoring 및 storage-safe rollback
    절차가 있다.

## Definition Of Done

- 공통 contract runner가 기존 및 reference provider에 실제 적용됨
- Single leader provider의 contention/cancellation, lost-owner 및 resign-recovery semantics가
  통일됨
- 취소, lifecycle, stale ownership 및 exact contention 결과가 race test로 증명됨
- English/Korean public documentation과 examples가 source와 일치함
- 새 dependency, module registration, workflow 변경 및 diagram은 근거 있는 N/A
- P0=0, P1=0, checklist Blocked=0
- Live PR CI/review가 green이고 merge는 사용자 결정 대기
