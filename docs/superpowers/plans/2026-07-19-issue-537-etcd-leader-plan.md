# etcd leader elector 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 필수 하위 스킬: 사용 superpowers:subagent-driven-development (권장) 또는 superpowers:executing-plans to 이 계획을 작업 단위로 구현. 단계는 checkbox (`- [ ]`) 추적 문법을 사용.

**목표:** 추가 a 호출자-client-owned etcd implementation of `leader.Elector` that passes 모든 15 mandatory provider-conformance cases without weakening their semantics.

**아키텍처:** `leader/etcd` creates one explicitly granted etcd lease, official `concurrency.Session`, 및 official `concurrency.Election` per Campaign generation. A mutex-protected generation record owns cancellation, publication, exact-key watch, single-flight Proclaim renewal, Session join, monitor join, 및 proof-based cleanup; `leader/leadertest.RunWithConfig` supplies etcd-compatible timing plus fail-stop timeout containment while preserving `Harness` 및 `Run` source compatibility.

**기술 스택:** Go 1.26, etcd client/v3 v3.6.13, Testcontainers etcd v0.42.0, `leader/leadertest`, `internal/testcleanup`, standard Go race/lint/CI gates.

---

## 파일 지도

| Area | 파일 | 책임 |
|---|---|---|
| Conformance timing | `leader/leadertest/timing.go`, `leader/leadertest/runner.go`, `leader/leadertest/{timing_test.go,runner_test.go}` | Source-compatible timing normalization, cancel/abort/join containment, unchanged 15-case table. |
| Public etcd API | `leader/etcd/doc.go`, `leader/etcd/elector.go`, `leader/etcd/options.go` | Constructor, key encoding, rounded TTL, 호출자-owned client 계약, `EffectiveTTL`. |
| Generation lifecycle | `leader/etcd/generation.go`, `leader/etcd/campaign.go`, `leader/etcd/monitor.go`, `leader/etcd/cleanup.go`, `leader/etcd/observe.go` | Campaign publication, Session/Election ownership, exact watch, Proclaim renewal, resign/reconciliation, redacted observation. |
| Unit 테스트 | `leader/etcd/{options_test.go,generation_test.go,campaign_test.go,monitor_test.go,cleanup_test.go,errors_test.go}` | Deterministic state, cancellation, nil-session, join, reconciliation, redaction, 및 concurrency contracts. |
| Real etcd 테스트 | `leader/etcd/{etcd_test_fixture_test.go,integration_test.go,conformance_test.go,security_test.go,resource_test.go}` | Official server fixture, real election semantics, fault control, mandatory conformance, trust-boundary 및 resource evidence. |
| Examples/docs | `leader/etcd/{example_test.go,README.md,README.ko.md}`, provider indexes, `CHANGELOG.md`, release runbook | Compile-checked usage, shutdown, migration, 보안, 없음-fencing, discoverability, release notes. |
| Dependency/review evidence | `go.mod`, `go.sum`, `docs/superpowers/reviews/*issue-537*`, `docs/lessons/*issue-537*` | Pinned dependency graph, risk prediction, 단계 6-R/7-R reviews, Type A reusable lesson. |

## 의존 순서 및 Execution Constraints

작업 0 freezes reviewed artifacts 및 records baseline evidence. 작업 1 must land 전에 provider
테스트 because etcd cannot represent the current 300ms harness lease. Tasks 2-5 build the provider
in TDD order: value/key 계약, Campaign, monitoring, then cleanup/observation. 작업 6 adds the
real-server adapter 및 mandatory conformance. 작업 7 hardens faults, trust boundaries, 및
resource containment. 작업 8 updates 공개 documentation 만 후 the API 및 테스트 settle.
작업 9 is the final repository gate.

다음을 하지 않는다: run Docker-backed packages in parallel. 다음을 하지 않는다: change `leader.Elector`, `leader.Options`,
또는 the 15 conformance cases. 다음을 하지 않는다: add capability flags, skips, relaxed assertions, a generic etcd
wrapper, a 공개 Testcontainers wrapper, an elector-owned client, 또는 a detached Campaign wrapper
goroutine. Any required deviation returns to 단계 2 design review.

### 작업 0: 고정 Artifacts, Baseline CI, 및 예측 Risks

**파일:**
- 검증: `docs/superpowers/specs/2026-07-19-issue-537-etcd-leader-design.md`
- 검증: `docs/superpowers/reviews/2026-07-19-issue-537-etcd-leader-step-2r-spec-review.md`
- 생성: `docs/superpowers/reviews/2026-07-19-issue-537-etcd-leader-step-3p-risk-prediction.md`

- [ ] **단계 1: 검증 the approved artifact-만 branch**

실행:

```bash
git status --short
git log --oneline origin/develop..HEAD
git diff --check origin/develop...HEAD
```

예상: 만 issue #537 design, review, 및 plan artifacts are ahead of `develop`; 없음
`leader/etcd` source 또는 dependency change exists.

- [ ] **단계 2: 기록 dependency 및 CI baselines**

실행 serially 및 paste exact duration, module, Go, platform, 및 Docker availability into the risk
artifact:

```bash
go version
go env GOOS GOARCH
go list -m github.com/testcontainers/testcontainers-go
/usr/bin/time -p make ci
```

예상: the pre-change full-CI baseline passes 및 is be낮음 the current 25-minute job timeout.
If Docker is unavailable, record that gap without substituting a mock-server claim.

- [ ] **단계 3: Write the pre-implementation risk ledger**

생성 a table 함께 `Risk`, `Trigger`, `Signal`, `Prevention`, `Recovery`, 및 `Owner`. Include:
integer-TTL drift, Campaign cleanup blocked on `client.Ctx`, cancellation/publication race,
nil Session 후 Grant/NewSession failure, monitor created 전에 publication failure,
watch-created handshake timeout, compaction, mismatched PUT, overlapping key ranges,
Proclaim overlap, stale monitor ABA, nil official Resign without delete, stale revision cleanup,
lease-level cross-principal revoke/keepalive, 공유-client hard stop, Testcontainers leak,
dependency graph churn, 32-contender leak, 및 rapid reacquisition without 호출자 fencing.

- [ ] **단계 4: 커밋 risk evidence 전에 source work**

```bash
git add docs/superpowers/reviews/2026-07-19-issue-537-etcd-leader-step-3p-risk-prediction.md
git commit -m "Predict etcd election delivery risks" \
  --trailer "Constraint: Official Campaign cleanup can outlive the caller deadline." \
  --trailer "Confidence: high" \
  --trailer "Scope-risk: narrow" \
  --trailer "Tested: pre-change make ci baseline and artifact diff checks"
```

예상: the risk commit predates every source 또는 module commit.

### 작업 1: 추가 Source-Compatible Conformance Timing 및 Containment

**파일:**
- 생성: `leader/leadertest/timing.go`
- 생성: `leader/leadertest/timing_test.go`
- Modify: `leader/leadertest/runner.go`
- Modify: `leader/leadertest/runner_test.go`
- Test: `leader/leadertest/example_test.go`

- [ ] **단계 1: Write RED normalization 및 compatibility 테스트**

Put the source-compatibility assertion in `package leadertest_test` so it proves the external API:

```go
var _ = leadertest.Harness{nil, nil}
```

추가 internal runtime assertions for normalization:

```go
func TestNormalizeTimingDefaultsAndPartialOverride(t *testing.T) {
    got, err := normalizeConfig(Config{Timing: Timing{Lease: 3 * time.Second}})
    if err != nil { t.Fatal(err) }
    if got.Timing.Lease != 3*time.Second || got.Timing.RenewInterval != 50*time.Millisecond ||
        got.Timing.CaseTimeout != 5*time.Second || got.Timing.WaitTimeout != 2*time.Second ||
        got.Timing.ResignTimeout != 250*time.Millisecond {
        t.Fatalf("normalized timing = %+v", got.Timing)
    }
}

func TestNormalizeTimingRejectsContainmentViolations(t *testing.T) {
    invalid := []Timing{
        {Lease: time.Second, RenewInterval: time.Second},
        {CaseTimeout: -time.Second},
        {CaseTimeout: 3 * time.Second, WaitTimeout: 2 * time.Second, ResignTimeout: time.Second},
        {CaseTimeout: time.Duration(1<<63 - 1), WaitTimeout: time.Duration(1<<63 - 2), ResignTimeout: time.Second},
    }
    for _, timing := range invalid {
        if _, err := normalizeConfig(Config{Timing: timing}); err == nil {
            t.Fatalf("accepted invalid timing %+v", timing)
        }
    }
}
```

추가 a runner 테스트 whose injected private evaluator waits on its received case context. Cancel the
case root 및 assert the evaluator observes cancellation 및 joins 전에 `Abort` can run. 추가 a
subprocess 테스트 whose elector remains blocked 후 root cancellation; 함께 a successful `Abort`,
assert event order `cancel, abort, joined`. With nil/failing Abort 및 an unjoined call, assert the
subprocess reaches the outer `go test -timeout` fail-stop instead of returning to a later case.

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./leader/leadertest -run 'TestNormalizeTiming|TestRunWithConfig|TestRunContainment'
```

예상: build FAIL because `Timing`, `Config`, `AbortFunc`, `RunWithConfig`, 및 normalization do
아님 exist.

- [ ] **단계 3: 구현 the timing API 및 containment formulas**

추가 Go doc comments for each exported type, field, 및 function. 유지 the blank fields 및 this
exact 공개 signature:

```go
type Timing struct {
    // Lease configures the case lease duration.
    Lease time.Duration
    // RenewInterval configures the case renewal cadence.
    RenewInterval time.Duration
    // CaseTimeout bounds evaluator work before cancellation and containment.
    CaseTimeout time.Duration
    // WaitTimeout bounds backend-state observation within a case.
    WaitTimeout time.Duration
    // ResignTimeout bounds normal cleanup before abort containment.
    ResignTimeout time.Duration
    _ struct{}
}

type AbortFunc func(context.Context, leader.Options) error

type Config struct {
    // Timing overrides zero-valued conformance timing fields.
    Timing Timing
    // Abort contains every timed-out case after root cancellation, whether the
    // evaluator joins during grace or requires a provider hard stop.
    Abort AbortFunc
    _ struct{}
}

func RunWithConfig(t *testing.T, harness Harness, config Config)
```

Resolve zero fields independently to `300ms`, `50ms`, `5s`, `2s`, 및 `250ms`; reject negatives,
`RenewInterval >= Lease`, 및 profiles that fail either inequality. Avoid duration addition so
호출자-controlled near-`MaxInt64` values cannot wrap:

```go
joinGrace := min(timing.ResignTimeout, timing.CaseTimeout/10)
abortBudget := min(timing.ResignTimeout, time.Second)
fits := func(first time.Duration) bool {
    return first < timing.CaseTimeout &&
        joinGrace < timing.CaseTimeout-first &&
        abortBudget < timing.CaseTimeout-first-joinGrace
}
if !fits(timing.WaitTimeout) || !fits(timing.ResignTimeout) {
    return Config{}, errors.New("leadertest: timing cannot contain a timed out case")
}
```

Make `Run` exactly `RunWithConfig(t, harness, Config{})`. Move the 기존 15-case table unchanged
into `RunWithConfig`, but change the private table function shape to:

```go
type evaluator func(context.Context, *testing.T, Harness, leader.Options, Timing) error
```

Each case creates one cancelable root 및 passes it plus normalized Timing to its evaluator.
Campaign, Leader, Control, wait, 및 contention-worker contexts derive from that root; `waitFor`
accepts the root 및 exits on `ctx.Done`. Bounded Resign cleanup uses a fresh context so root
cancellation cannot skip release. 보존 모든 15 names, order, 및 assertions. On timeout: cancel
the root, wait `joinGrace`, call Abort 함께 a fresh `abortBudget` context whether already joined 또는
still running, then join. If the call
remains unjoined, block so the outer 테스트 timeout fails the process rather than leaking work into
the next subtest.

- [ ] **단계 4: 검증 기존 및 custom profiles**

```bash
go test -count=1 ./leader/leadertest
go test -race -count=1 ./leader/leadertest
```

예상: 기존 `Run(t, MemoryHarness())`, redaction subprocesses, broken-provider detection,
custom timing, abort ordering, 및 fail-stop subprocess 테스트 PASS.

- [ ] **단계 5: 커밋 the harness amendment**

```bash
git add leader/leadertest
git commit -m "Contain provider conformance timeouts" \
  --trailer "Constraint: etcd leases use integer-second TTLs and Campaign cleanup may block." \
  --trailer "Rejected: Provider capability skips | every mandatory case remains unchanged." \
  --trailer "Confidence: high" \
  --trailer "Scope-risk: moderate" \
  --trailer "Tested: go test and go test -race ./leader/leadertest"
```

### 작업 2: Pin Dependencies 및 정의 the etcd Value/Key Contract

**파일:**
- Modify: `go.mod`
- Modify: `go.sum`
- 생성: `leader/etcd/doc.go`
- 생성: `leader/etcd/elector.go`
- 생성: `leader/etcd/generation.go`
- 생성: `leader/etcd/options.go`
- 생성: `leader/etcd/options_test.go`

- [ ] **단계 1: 추가 RED constructor, TTL, key, 및 없음-I/O 테스트**

Test nil client, invalid options, `RenewInterval < 100ms`, `RenewInterval >= Lease`, exact-second TTL, fractional round-up,
overf낮음, token uniqueness, slash-containing input, encoded sibling isolation, 및 없음 backend I/O.
사용 these exact expectations:

```go
func TestEncodeElectionRange(t *testing.T) {
    got := electionPaths(leader.Options{KeyPrefix: "tenant/a", Group: "billing/b"})
    wantBase := "/bluetape4k/leader/dGVuYW50L2E/YmlsbGluZy9i"
    if got.base != wantBase || got.root != wantBase+"/" ||
        got.end != clientv3.GetPrefixRangeEnd(wantBase+"/") {
        t.Fatalf("paths = %+v", got)
    }
}

func TestRequestedTTL(t *testing.T) {
    cases := []struct{ lease time.Duration; want int64 }{
        {time.Second, 1}, {time.Second + time.Nanosecond, 2}, {500 * time.Millisecond, 1},
    }
    for _, tc := range cases {
        got, err := requestedTTL(tc.lease)
        if err != nil || got != tc.want { t.Fatalf("ttl(%s) = %d, %v", tc.lease, got, err) }
    }
}
```

- [ ] **단계 2: Observe RED 및 add pinned modules**

```bash
go test -count=1 ./leader/etcd -run 'TestNew|TestEncode|TestRequestedTTL'
go get go.etcd.io/etcd/client/v3@v3.6.13
go mod tidy
```

예상: the first command fails on the deliberately missing etcd module 및 implementation
symbols (`clientv3`, `New`, `electionPaths`, 및 `requestedTTL`), 아님 because the already-created
`leader/etcd` 테스트 패키지 is absent. The module commands add 만 the selected etcd production
client plus its required transitive graph. The Testcontainers etcd module is deferred until 작업 6
creates its importing fixture so `go mod tidy` cannot remove it.

- [ ] **단계 3: 구현 the constructor-만 API**

사용 `base64.RawURLEncoding` for both segments 및 this state shape:

```go
type Elector struct {
    client *clientv3.Client
    opts leader.Options
    paths electionPath
    token string
    requestedTTL int64

    mu sync.RWMutex
    campaigning bool
    current *generation
    lastTTL time.Duration
    nextGeneration uint64
    testHook func(operation, phase string) error
}

func New(client *clientv3.Client, opts leader.Options) (*Elector, error)
func (e *Elector) EffectiveTTL() time.Duration
```

유지 the 패키지 name `etcdleader` 및 add Go doc comments for `Elector`, `New`, 및 `EffectiveTTL`.
The docs must state that the 호출자 owns the client, `New` performs 없음 network I/O, the `Elector`
zero value is unusable, 및 callers must construct it 함께 `New`. Defer
`var _ leader.Elector = (*Elector)(nil)` until 작업 5 has implemented the complete interface; do 아님
add temporary 공개 method stubs merely to satisfy an early assertion.

작업 2's initial `generation` contains 만 `ttl time.Duration` 및 `published bool`; 작업 3 expands
the same type 함께 lifecycle ownership fields. This keeps `current *generation` compilable without
inventing a second state type.

`New` performs 만 normalization, duration math, key encoding, 및 a 128-bit `crypto/rand` token.
It never calls the client, creates a lease/session, 또는 closes 호출자 resources. `EffectiveTTL`
reads current published TTL, otherwise last published TTL, otherwise requested rounded TTL under
the state lock. 추가 synchronized transition 테스트 proving requested -> published current ->
last-published behavior under concurrent readers 및 `go test -race`; an in-progress Grant 및 any
invalid/overf낮음 server-granted TTL remain invisible 및 cannot replace the last valid value.

- [ ] **단계 4: 검증 및 commit**

```bash
gofmt -w leader/etcd
go test -count=1 ./leader/etcd -run 'TestNew|TestEncode|TestRequestedTTL|TestEffectiveTTL'
go mod verify
git add go.mod go.sum leader/etcd/doc.go leader/etcd/elector.go leader/etcd/generation.go leader/etcd/options.go leader/etcd/options_test.go
git commit -m "Define the etcd election boundary" \
  --trailer "Constraint: Election ranges must not overlap raw caller identities." \
  --trailer "Rejected: Exposing SessionOption | the provider owns lease and Session semantics." \
  --trailer "Confidence: high" \
  --trailer "Scope-risk: moderate" \
  --trailer "Tested: targeted constructor and key tests; go mod verify"
```

### 작업 3: 구현 Generation-Safe Campaign 및 Local Session Join

**파일:**
- Modify: `leader/etcd/generation.go`
- 생성: `leader/etcd/campaign.go`
- 생성: `leader/etcd/monitor.go`
- 생성: `leader/etcd/generation_test.go`
- 생성: `leader/etcd/campaign_test.go`
- Modify: `leader/etcd/elector.go`

- [ ] **단계 1: Write RED lifecycle 테스트 around injectable boundaries**

유지 production on official etcd types but give each elector its own unexported `etcdOps` bundle
for Grant, NewSession, Campaign, Proclaim, `snapshotElection`, watch-created, revoke, Get,
`sessionDone`, `orphanSession`, 및 ticker construction. `snapshotElection` returns an
`electionSnapshot{key string, createRev, headerRev int64}`; the production adapter reads the
official `Election.Key()`, `Election.Rev()`, 및 `Election.Header().Revision`, rejects a nil Header,
및 validates the key plus both revisions 전에 publication. Per-elector seams prevent parallel
테스트 from racing 패키지 globals while the official `*concurrency.Session` remains available to
`NewElection`. Tests must prove:

작업 3 adds `ops etcdOps` to `Elector`; production `New` installs real operations 및 테스트 replace
the bundle on an individual elector. Copy the bundle by value into each generation 전에 any
remote dispatch. Once Campaign starts, neither production nor 테스트 mutate that generation's copy.

```go
func TestShutdownGenerationIsNilSessionSafe(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    g := &generation{ctx: ctx, cancel: cancel, shutdownDone: make(chan struct{})}
    var wg sync.WaitGroup
    for range 8 {
        wg.Add(1)
        go func() { defer wg.Done(); _ = g.shutdown(context.Background()) }()
    }
    wg.Wait()
    select { case <-g.shutdownDone: default: t.Fatal("shutdown did not close") }
}

func TestPublicationAndCancellationHaveOneWinner(t *testing.T) {
    // Synchronize cancellation and publish on one barrier.
    // Assert either published=true with a stopped/joined callback, or cleanup=true;
    // never both published success and caller cancellation ownership.
}
```

추가 cases for Grant failure, NewSession failure 후 a known lease, Campaign failure,
Proclaim failure, created-notify timeout, mismatched created PUT, callback join, failure 후 a
monitor handle exists, 및 Created acknowledgement immediately fol낮음ed by DELETE 또는 mismatched
PUT. For every successfully created Session, assert `Session.Done()` is closed 전에 Campaign
returns. For every pre-publication monitor failure, assert its handle joins too.

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./leader/etcd -run 'TestShutdownGeneration|TestCampaign|TestPublication'
```

예상: FAIL on the deliberately missing generation lifecycle, operations bundle, 및 Campaign
symbols.

- [ ] **단계 3: 구현 the generation record 및 one shutdown owner**

사용 this minimum ownership record:

```go
type generation struct {
    id uint64
    ctx context.Context
    cancel context.CancelFunc
    leaseID clientv3.LeaseID
    ttl time.Duration
    published bool
    ops etcdOps
    session *concurrency.Session
    election *concurrency.Election
    key string
    createRev int64
    proclaimRev int64
    monitorDone chan struct{}
    shutdownOnce sync.Once
    shutdownDone chan struct{}
    shutdownErr error
}
```

`shutdown` is nil-session-safe. One 호출자 runs cancel then the per-elector `orphanSession`
operation when Session exists; 모든 callers wait for `shutdownDone`. Production operations call
`Session.Orphan` 및 return `Session.Done`. A successfully created Session must have a closed Done
channel 전에 shutdown completes. Unit 테스트 inject a controllable Done channel 및 assert exact
orphan-전에-close/join ordering on every exit. This proves local goroutine termination 만 및
must 아님 clear remote cleanup inventory.

- [ ] **단계 4: 구현 synchronous Campaign publication**

구현 the eight spec steps in order: reconcile pending cleanup; explicit Grant; NewSession
함께 `WithContext` 및 `WithLease`; `NewElection(session, electionBase)`; synchronous Campaign;
bounded Proclaim; call the injected `snapshotElection` adapter 및 validate key/revision/header;
create the exact-key watch from `headerRev+1` 함께 `WithCreatedNotify`, hand that WatchChan plus the
injected `sessionDone` signal to a joinable monitor 및 receive its created acknowledgement; then
serialize 호출자 cancellation against publication under `e.mu`. Publication is forbidden until
the monitor owns both observation inputs 및 has acknowledged watch creation. 작업 3's
`monitor.go` owns this minimal start/Created/terminal/join primitive. Under `e.mu`, publication must
recheck both generation cancellation 및 the monitor's terminal state so a Created-then-loss event
cannot publish stale leadership.

Precompute the deterministic key as `candidateRoot + fmt.Sprintf("%x", leaseID)` 및 retain the
known lease even when NewSession fails. Any post-dispatch failure cancels 및 joins the generation,
attempts bounded revoke, joins any created monitor, 및 clears state 만 후 revoke 또는 exact
linearizable absence/replacement proof. Wrap 공개 failures 함께
`leader.NewOperationError("etcd", "campaign", cause)` 및 join `leader.ErrCommitUnknown` when proof
is unavailable.

- [ ] **단계 5: 검증 races 및 commit**

```bash
go test -count=20 ./leader/etcd -run 'TestShutdownGeneration|TestCampaign|TestPublication'
go test -race -count=1 ./leader/etcd -run 'TestShutdownGeneration|TestCampaign|TestPublication'
git add leader/etcd
git commit -m "Serialize etcd campaign publication" \
  --trailer "Constraint: Caller cancellation and official Campaign cleanup can race after mutation." \
  --trailer "Rejected: Detached Campaign goroutine | it can mutate after method return." \
  --trailer "Confidence: high" \
  --trailer "Scope-risk: broad" \
  --trailer "Tested: 20x lifecycle tests and targeted race tests"
```

### 작업 4: 추가 Exact-Key Monitoring 및 Single-Flight Proclaim

**파일:**
- Modify: `leader/etcd/monitor.go`
- 생성: `leader/etcd/monitor_test.go`
- Modify: `leader/etcd/generation.go`
- Modify: `leader/etcd/campaign.go`

- [ ] **단계 1: Write RED monitor 테스트**

사용 synchronized fake watch 및 Proclaim boundaries to cover Session loss, exact DELETE, mismatched
PUT token/revision/lease, compaction, watch cancellation/오류, Proclaim failure, stale generation,
concurrent Resign, s낮음 Proclaim, 및 operation counts. 검증:

```go
wantMax := int64(math.Ceil(float64(elapsed)/float64(opts.RenewInterval))) + 1
if got := control.OperationCount(opts, leadertest.OperationRenew); got > wantMax {
    t.Fatalf("renew count = %d, want <= %d", got, wantMax)
}
if maxInFlight.Load() != 1 { t.Fatalf("max in-flight Proclaim = %d", maxInFlight.Load()) }
```

Every loss must clear `IsLeader` 전에 protected work can continue, invoke the same generation
shutdown helper, preserve cleanup when exact absence is unproved, 및 close the monitor done handle.

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./leader/etcd -run 'TestMonitor|TestProclaim'
```

예상: FAIL because monitor 및 renewal code are absent.

- [ ] **단계 3: 구현 the monitor loop**

Select on `Session.Done`, the exact-key watch, 및 a `RenewInterval` ticker. Validate every PUT by
key, token, creation revision, 및 lease. Treat DELETE, mismatched PUT, compaction, watch 오류,
Session loss, 및 Proclaim failure as terminal loss. Proclaim uses a fresh generation-derived
context bounded by `min(RenewInterval, grantedTTL/4, 1s)` 및 never overlaps 또는 queues. No mutex is
held during RPC, wait, shutdown, 또는 join. A generation ID check prevents an old monitor clearing a
new owner. Construct the ticker 함께 `time.NewTicker`, defer `ticker.Stop`, 및 add a terminal-path
테스트 using an injected ticker factory whose stop counter must equal one for every monitor exit.

- [ ] **단계 4: 검증 및 commit**

```bash
go test -count=20 ./leader/etcd -run 'TestMonitor|TestProclaim'
go test -race -count=1 ./leader/etcd -run 'TestMonitor|TestProclaim'
git add leader/etcd/monitor.go leader/etcd/monitor_test.go leader/etcd/generation.go leader/etcd/campaign.go
git commit -m "Fail closed on etcd ownership loss" \
  --trailer "Constraint: Session health alone does not prove the candidate key still exists." \
  --trailer "Confidence: high" \
  --trailer "Scope-risk: broad" \
  --trailer "Tested: 20x monitor tests and targeted race tests"
```

### 작업 5: 구현 Proof-Based Resign, Reconciliation, 및 Leader Lookup

**파일:**
- 생성: `leader/etcd/cleanup.go`
- 생성: `leader/etcd/observe.go`
- 생성: `leader/etcd/cleanup_test.go`
- 생성: `leader/etcd/errors_test.go`
- 생성: `leader/etcd/elector_contract_test.go`
- Modify: `leader/etcd/elector.go`

- [ ] **단계 1: Write RED cleanup 및 redaction 테스트**

Cover nil/canceled context, idempotent 없음-owner Resign, known/unknown revision, official nil Resign
함께 surviving key, revoke success, revoke ambiguity, exact absence, exact replacement, stale
revision, expired TTL without proof, retry on the same elector, concurrent Resign, 호출자-client
usability, linearizable oldest-candidate lookup, 및 forbidden markers. Require:

```go
if err == nil || !errors.Is(err, leader.ErrCommitUnknown) { t.Fatalf("resign error = %v", err) }
if !errors.Is(elector.Campaign(context.Background()), leader.ErrCleanupPending) {
    t.Fatal("Campaign did not preserve unresolved cleanup")
}
for _, marker := range []string{endpoint, username, password, token, encodedGroup, leaseID, certPath, rawCause} {
    if strings.Contains(err.Error(), marker) { t.Fatalf("error leaked %q", marker) }
}
```

추가 a table for each synchronous 공개 operation (`Campaign`, `Resign`, 및 `Leader`) proving nil
context returns bare `leader.ErrInvalidContext`, pre-dispatch cancellation/deadline returns the bare
context 오류, post-dispatch failure declares `var operationErr *leader.OperationError` 및 requires
`errors.As(err, &operationErr)`, preserves causes/sentinels through `errors.Is`, 및 exposes 만
backend `etcd` plus operation `campaign`, `resign`, 또는 `lookup`. Test asynchronous `renew` failure
separately: it must fail closed by clearing leadership, preserve unresolved cleanup inventory, 및
emit 없음 raw diagnostic through repository-owned observability. Feed the same forbidden markers
through every repository-owned 테스트 logger, example diagnostic helper, 및 telemetry stub; assert
none renders an unwrapped 원인 또는 raw marker.

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./leader/etcd -run 'TestResign|TestReconcile|TestLeader|TestRedaction'
```

예상: FAIL because cleanup 및 observation boundaries are absent.

- [ ] **단계 3: 구현 공유 cleanup**

Resign first clears local leadership 및 cancels the generation. 실행/join generation shutdown,
then join the exact monitor. With a known revision, call
`concurrency.ResumeElection(session, candidateRoot, key, createRev)` 및
official Resign. With unknown revision, reconcile deterministic key first. Every dispatched
official Resign result, nil 또는 non-nil, is fol낮음ed by bounded lease revoke when budget remains 및
a default-linearizable exact-key Get; a non-nil response may still fol낮음 a committed delete. Clear state 만
후 successful revoke 또는 proof of exact absence/replacement; compare key, creation revision,
token, 및 lease wherever known. Elapsed TTL 및 `Session.Done` never clear inventory.

If proof fails, return a redacted resign operation 오류 joined 함께 `leader.ErrCommitUnknown` 및
retain the generation for a same-elector retry. Concurrent calls share the shutdown result 및 만
one may clear the generation.

- [ ] **단계 4: 구현 observation 및 공개 오류 labels**

`Leader` uses one default-linearizable Get over `[candidateRoot, rangeEnd)`. Assemble options as
`opts := append([]clientv3.OpOption{clientv3.WithRange(rangeEnd)}, clientv3.WithFirstCreate()...)`
because `WithFirstCreate` returns a slice. Return empty on 없음 candidate 및 the oldest candidate value otherwise.
Expose 만 `campaign`, `renew`, `resign`, 및 `lookup` as operation labels; map Grant, Session,
watch, Proclaim, revoke, 및 reconciliation phases to the owning 공개 operation.
At this point add `var _ leader.Elector = (*Elector)(nil)` beside the implementation. 추가 an
external-패키지 zero-value 계약 테스트: `IsLeader` is false, Campaign 및 Leader fail
deterministically for valid contexts without panic 또는 backend dispatch, 및 Resign 함께 a valid
context is a safe 없음-op; nil contexts still return `leader.ErrInvalidContext`. The 테스트 does 아님 make
the zero value supported; it enforces the documented constructor-만 failure boundary.

- [ ] **단계 5: 검증 및 commit**

```bash
go test -count=20 ./leader/etcd -run 'TestResign|TestReconcile|TestLeader|TestRedaction'
go test -race -count=1 ./leader/etcd -run 'TestResign|TestReconcile|TestLeader|TestRedaction'
git add leader/etcd
git commit -m "Require proof before clearing etcd ownership" \
  --trailer "Constraint: Session termination and TTL passage are not remote deletion proof." \
  --trailer "Rejected: Trusting nil Election.Resign | its compare can miss." \
  --trailer "Confidence: high" \
  --trailer "Scope-risk: broad" \
  --trailer "Tested: 20x cleanup tests and targeted race tests"
```

### 작업 6: 구성 the Real etcd Fixture 및 Mandatory Conformance Adapter

**파일:**
- Modify: `go.mod`
- Modify: `go.sum`
- 생성: `leader/etcd/etcd_test_fixture_test.go`
- 생성: `leader/etcd/integration_test.go`
- 생성: `leader/etcd/conformance_test.go`

- [ ] **단계 1: Write the serial real-server fixture**

가져오기 the Testcontainers etcd module in the fixture first, then add it so tidy retains the direct
테스트 dependency:

```bash
go get github.com/testcontainers/testcontainers-go/modules/etcd@v0.42.0
go mod tidy
go mod verify
```

Resolve the Docker container target platform, never the Go host OS, 및 map it to the approved
digest:

```go
var etcdDigest = map[string]string{
    "linux/amd64": "sha256:946dfbae58b1dec56af786a23e7322484b58281547bef1e848321f6beeb388d5",
    "linux/arm64": "sha256:23c14fbdf70105a54146cf5ed3a81613b99a973c60d5907851a251ca15664e96",
}
```

`containerPlatform(ctx)` first honors a non-empty `DOCKER_DEFAULT_PLATFORM`; otherwise it opens the
Testcontainers Docker client, reads daemon `Info`, 및 derives `OSType/Architecture`. Normalize 만
the architecture aliases `x86_64 -> amd64` 및 `aarch64 -> arm64`, require Linux, 및 accept 만
`linux/amd64` 또는 `linux/arm64`. The fixture may import `github.com/moby/moby/client` for
`InfoOptions`; `go mod tidy` may promote that already-present Testcontainers dependency to direct,
but must 아님 change its selected version.

Reject every platform absent from the map. Pass both the immutable reference
`"gcr.io/etcd-development/etcd@" + etcdDigest[platform]` 및
`testcontainers.WithImagePlatform(platform)` directly to `etcd.Run`; keep `v3.6.13` 만 as
recorded version metadata. 추가 table 테스트 for environment override, daemon fallback, aliases,
unsupported OS/architecture, 및 env-over-daemon precedence. 다음을 하지 않는다: launch the mutable tag.

Readiness requires bounded `Status` member/leader evidence 및 one linearizable Put/Get/Delete
roundtrip. Register client 및 partially created container cleanup 함께 `internal/testcleanup`; log
tag, digest, platform, 및 endpoint count without credentials.

- [ ] **단계 2: 추가 RED real-election 테스트 및 control**

추가 acquire/observe, 16-contender exact winner, canceled waiter 함께 없음 late candidate, keepalive
beyond requested lease, external revoke, key loss, watch interruption, idempotent resign,
lost-response retry, stale revision, 호출자-client reuse, 및 stable single-node restart cases.

구현 a concurrency-safe 테스트 control whose `ReplaceOwner` revokes the current leader lease,
creates a control lease/session, Campaigns the replacement, immediately calls `Session.Orphan`, 및
retains its lease ID for teardown. `FailNext` runs the real Campaign/Proclaim/Resign first 및 loses
만 the response. `OperationCount` is key-및-operation-specific 및 monotonic.

- [ ] **단계 3: Observe RED then run the mandatory suite 함께 exact timing**

```bash
go test -p 1 -count=1 ./leader/etcd -run 'TestEtcdIntegration|TestEtcdElectorConformance'
```

사용:

```go
leadertest.RunWithConfig(t, harness, leadertest.Config{
    Timing: leadertest.Timing{
        Lease: 3 * time.Second, RenewInterval: time.Second,
        CaseTimeout: 12 * time.Second, WaitTimeout: 4 * time.Second,
        ResignTimeout: 2 * time.Second,
    },
    Abort: func(ctx context.Context, opts leader.Options) error { return clients.closeFor(ctx, opts) },
})
```

The adapter owns a mutex-protected `caseClients` registry keyed by normalized group.
`Harness.New` calls `clientFor(group)` so every elector created by one conformance case—including
the seven exact-contention electors—reuses the same case-dedicated concurrency-safe client. The
first creation registers one cleanup; later factories cannot replace 또는 leak the entry. `closeFor`
atomically detaches the entry 및 best-effort closes it even when the Abort context is already
ended, returning `errors.Join(ctx.Err(), closeErr)` as applicable. Thus Abort closes every 공유
사용자 in that case 및 never a client used by a different case. 예상: 모든 15 기존 named
cases PASS 함께 없음 skips 또는 relaxed assertions.

- [ ] **단계 4: 검증 및 commit**

```bash
go test -p 1 -count=1 ./leader/leadertest ./leader/etcd
go mod verify
git add go.mod go.sum leader/etcd
git commit -m "Prove etcd leader conformance on a real server" \
  --trailer "Constraint: Docker-backed etcd tests must run serially with case-owned abort clients." \
  --trailer "Confidence: high" \
  --trailer "Scope-risk: broad" \
  --trailer "Tested: all 15 leadertest cases and real etcd integration"
```

### 작업 7: 강화 Trust Boundaries, Faults, 및 Resource Containment

**파일:**
- 생성: `leader/etcd/security_test.go`
- 생성: `leader/etcd/resource_test.go`
- 생성: `leader/etcd/shutdown_test.go`
- Modify: `leader/etcd/integration_test.go`
- Modify: `leader/etcd/conformance_test.go`

- [ ] **단계 1: 추가 authenticated boundary 테스트**

사용 a separate serial container that 없음 unauthenticated 테스트 shares. Bootstrap the root 사용자 및
root role 전에 `AuthEnable`, reconnect 함께 root credentials, then create principals/roles A 및
B. Grant each role exactly its encoded `[candidateRoot, rangeEnd)` through
`RoleGrantPermission`; create separate authenticated clients 및 register bounded teardown that
disables auth 및 terminates the isolated container. 검증 A can Put/Get/Delete/Watch its own
range 및 every same operation is denied on B's sibling range, 함께 symmetric assertions for B.

On v3.6.13, attach A's candidate key to A's lease 및 assert B's authenticated cross-principal
Revoke 및 `KeepAliveOnce` are denied because server authorization checks every attached key. Also
prove B can revoke an unattached lease, then loses that ability 후 an A-range key is attached.
기록 these pinned results without treating lease IDs as creator-owned 또는 prefix-scoped
capabilities. Principals 함께 the same range remain mutually trusted, mutually untrusted tenants
always use separate clusters, 및 every server-version change reruns both denial 테스트. Mark 모든
plaintext fixtures 테스트-만.

- [ ] **단계 2: 추가 deterministic lifecycle interleavings**

실행 barrier-controlled 테스트 for cancellation versus publication, Resign versus monitor loss,
watch-created response fol낮음ed immediately by mismatched PUT/DELETE, compaction, Proclaim delay,
post-success lost responses, cleanup reconciliation ABA, 및 rapid reacquisition. The rapid
reacquisition 테스트 must stop 및 join prior protected work 전에 a new Campaign 또는 use an explicit
테스트 fencing generation.

- [ ] **단계 3: 증명 the blocked-Campaign hard-stop branch**

Inject a Campaign blocked in official cleanup on `client.Ctx`. Cancel the case root, wait the join
grace, coordinate every 사용자 of that case-dedicated client, close it, 및 assert the Campaign plus
Session/monitor handles join. 보존 cleanup inventory, open a separate healthy diagnostic
client, 및 require linearizable exact-range absence/replacement proof 전에 clearing the 테스트's
restart gate. A timeout invokes the subprocess fail-stop instead of leaking the blocked call.

- [ ] **단계 4: 추가 the 32-contender resource 테스트**

Before starting contenders, capture `Lease.Leases` count, scrape the server
`etcd_debugging_mvcc_watcher_total` metric, 및 read 패키지-private atomic counters for live
Sessions, published monitors, 및 in-flight Proclaims. Start 32 contenders in one group; subtract
the baseline 및 poll 함께 the configured wait deadline. 검증 at most 32 live leases, 32 live
Sessions, 32 server watchers, one published monitor, 및 one in-flight Proclaim. Cancel,
Resign/reconcile, close case clients, 및 poll until every baseline delta is exactly zero. 하지 않는다
assert the process-global goroutine count.

- [ ] **단계 5: 실행 normal 및 race gates 및 measure admission inputs**

```bash
/usr/bin/time -p go test -p 1 -count=1 ./leader/etcd
/usr/bin/time -p go test -race -p 1 -count=1 ./leader/leadertest ./leader/etcd
```

예상: PASS 함께 one in-flight Proclaim, 없음 late candidate, 모든 local Session/monitor handles
joined, 없음 forbidden diagnostic marker, 및 fixture resources back at baseline. 기록 durations;
the ten-run soak remains 작업 9's admission-gated pre-release command.

- [ ] **단계 6: 커밋 hardening evidence**

```bash
git add leader/etcd
git commit -m "Harden etcd election failure boundaries" \
  --trailer "Constraint: etcd KV prefixes do not automatically isolate lease revocation." \
  --trailer "Confidence: high" \
  --trailer "Scope-risk: broad" \
  --trailer "Tested: real-server suite, race suite, auth, hard-stop, and 32-contender tests"
```

### 작업 8: 공개 Bilingual Usage, Shutdown, Migration, 및 Lesson Guidance

**파일:**
- 생성: `leader/etcd/README.md`
- 생성: `leader/etcd/README.ko.md`
- 생성: `leader/etcd/example_test.go`
- 생성: `leader/etcd/readme_test.go`
- 생성: `docs/lessons/2026-07-19-issue-537-etcd-leader.md`
- Modify: `leader/elector.go`
- Modify: `leader/README.md`
- Modify: `leader/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`
- Modify: `docs/release/v0.19.0-provider-conformance-runbook.md`

- [ ] **단계 1: 추가 compile-checked example 전에 prose**

추가 one acquire/work/resign example 및 one shutdown-supervisor example. Both must inspect
`campaignCtx.Err()` even 후 nil Campaign return, stop 및 join protected work when `IsLeader`
clears, preserve initiating plus cleanup 오류, retry Resign on the same elector, use
`EffectiveTTL` 만 to schedule another reconciliation, 및 never infer cleanup from elapsed time.
The supervisor sequence is: cancel campaigns, bounded join grace, same-elector Resign while the
client is healthy, coordinate 공유-client users, close the 호출자 client 만 for blocked calls,
join them, persist unresolved inventory, then require a separate healthy client to prove exact
range absence 전에 restart.
Extend the compile-checked supervisor through symmetric rollback: stop protected work 및 every
etcd contender, perform bounded same-elector cleanup, prove exact range absence 함께 a healthy
diagnostic client, re-enable the previous provider, 및 verify zero etcd contenders.

추가 a compile-checked 호출자-owned production client example that loads a CA pool, sets a non-empty
`ServerName`, leaves `InsecureSkipVerify=false`, supplies scoped authentication, 및 passes the
resulting `*clientv3.Client` to `New`. A focused 테스트 rejects an example config 함께 an empty root
pool, empty ServerName, invalid credentials, 또는 `InsecureSkipVerify=true`; 없음 production TLS
ownership moves into the provider.

- [ ] **단계 2: Write section-for-section 영문/한국어 provider docs**

Include install/client ownership, the constructor-만 계약 및 unusable `Elector` zero value,
encoded ranges, integer/server-granted TTL, Proclaim versus keepalive, cancellation limitation,
exact local Session join, cleanup pending, RBAC lease-level trust, TLS, quorum/compaction, 없음
fencing, observability sampling, shutdown, stop-the-world cutover, symmetric rollback, rapid
reacquisition rule, tested server version, 및 unsupported scope. State in both languages that
callers must use `New` 및 that `errors.Unwrap` exposes diagnostic-sensitive raw etcd causes which
must 아님 be logged 또는 emitted to telemetry without sanitization.

- [ ] **단계 3: 업데이트 indexes, changelog, 및 release runbook**

Register `leader/etcd` in both leader 및 root README pairs. 추가 the v0.19.0 change 함께 the
호출자-owned client 및 없음-fencing caveat. Extend the provider runbook 함께 preflight 상태 plus
linearizable roundtrip, exact-range RBAC, campaign drain, unresolved inventory, cutover/rollback,
quorum recovery, 및 dependency rollback gates.

업데이트 the backend-neutral `leader.Elector.Campaign` Go doc so it 없음 longer promises TTL expiry as
a universal cleanup fallback: callers retry bounded Resign on the same elector 및 then fol낮음 the
provider-specific proof/expiry 계약; etcd requires successful revoke 또는 linearizable exact-key
reconciliation.

- [ ] **단계 4: 기록 the Type A lesson**

캡처 four reusable findings 함께 concrete evidence: official Campaign uses long-lived client
context for cleanup, server-granted TTL controls budgets, Session plus exact watch plus Proclaim are
distinct ownership signals, 및 provider timing profiles must preserve cases while containing
timed-out goroutines.

- [ ] **단계 5: 검증 parity 및 commit**

```bash
go test -count=1 ./leader/etcd -run '^Example'
go test -count=1 ./leader/etcd -run 'TestReadmeParity|TestRunbookContract|TestTLSExample'
git diff --check
git add leader/elector.go leader/etcd README.md README.ko.md leader/README.md leader/README.ko.md CHANGELOG.md docs/release/v0.19.0-provider-conformance-runbook.md docs/lessons/2026-07-19-issue-537-etcd-leader.md
git commit -m "Document safe etcd leader operations" \
  --trailer "Constraint: Local Session join and elapsed TTL do not prove remote deletion." \
  --trailer "Confidence: high" \
  --trailer "Scope-risk: moderate" \
  --trailer "Tested: compile-checked examples, TLS assertions, bilingual heading parity, runbook contract, git diff --check"
```

`TestReadmeParity` extracts ordered `##` headings from both provider README files 및 requires a
one-to-one section mapping covering every 단계 2 topic. `TestRunbookContract` requires executable
command/checklist blocks for 상태 plus linearizable roundtrip, campaign drain, separate-client
reconciliation, exact absence, symmetric rollback, dependency rollback (`git diff go.mod go.sum`,
provider registration removal, `go mod tidy`, `make ci`), quorum recovery, 및 observability
sampling cadence; a keyword-만 presence check is insufficient.

### 작업 9: 실행 Final 검증 및 준비 리뷰 증거

**파일:**
- 생성 later: `docs/superpowers/reviews/2026-07-19-issue-537-etcd-leader-step-6r-code-review.md`
- 생성 later: `docs/superpowers/reviews/2026-07-19-issue-537-etcd-leader-step-7r-pr-review.md`
- Conditionally modify if admission fails: `Makefile`
- Conditionally modify if admission fails: `.github/workf낮음s/ci.yml`
- Conditionally modify if admission fails: `.github/workf낮음s/nightly-tests.yml`
- 검증: every issue #537 file 및 dependency delta

- [ ] **단계 1: 검증 dependency shape**

```bash
go list -m go.etcd.io/etcd/client/v3 github.com/testcontainers/testcontainers-go/modules/etcd
go mod verify
go mod tidy
git diff --exit-code -- go.mod go.sum
```

예상: etcd client is v3.6.13, Testcontainers etcd is v0.42.0, module verification passes,
및 tidy leaves 없음 diff. 기록 gRPC, protobuf, Prometheus, `x/net`, `x/sys`, 및 zap graph changes
in the implementation review.

- [ ] **단계 2: 실행 targeted, race, 및 repository gates serially**

```bash
go test -p 1 -count=1 ./leader/leadertest ./leader/etcd
go test -race -p 1 -count=1 ./leader/leadertest ./leader/etcd
make fmt-check
make tidy-check
make vet
make lint
make ci
```

예상: every command PASS. 기록 targeted/race/full-CI durations. If normal targeted 테스트
exceed three minutes, race exceeds five minutes, combined exceeds eight minutes, 또는 projected full
CI exceeds 20 minutes/80% of the job timeout, move repeated Docker/race work to a separate CI lane
전에 PR review. Command timings are diagnostic 만; workf낮음 admission is based on the complete
target job, including checkout, Go setup/cache, Docker preflight, cold image pull, 기존
workloads, 및 every 테스트 step.

For that conditional branch, parameterize `Makefile` 테스트/coverage/race 패키지 lists so CI can run
모든 packages except `./leader/etcd` 변경하지 않고 local defaults. 추가 an `etcd-leader` job to
`.github/workf낮음s/ci.yml` 함께 `needs: ci`, `timeout-minutes: 15`, Docker preflight, 및 serial
`go test -p 1 -count=1` plus `go test -race -p 1 -count=1` for
`./leader/leadertest ./leader/etcd`; keep it a required PR check. Put the nightly repetition in a
dedicated `etcd-leader-soak` job 함께 `timeout-minutes: 30`; never append it to the 기존
Testcontainers workload. Before push, run a CI-equivalent sequence including Docker preflight,
immutable image pull, setup-equivalent module download, 및 every target job command. After push,
read each live job's `startedAt`/`completedAt` from GitHub Actions 및 require the complete PR job to
finish within 12 minutes 및 the complete soak job within 24 minutes (80% of their timeouts). If a
job exceeds the limit, increase its timeout 함께 explicit 20% headroom 또는 reduce 만 the nightly
repetition count to the largest measured value within the limit while retaining the ten-run manual
pre-release gate; rerun exact-head CI 전에 merge readiness.

Validate every local workf낮음 diff mandatorily 함께
`go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 .github/workf낮음s/*.yml`, then run both
separated local command sets 전에 accepting the split. 사용 `gh workf낮음 view` 만 후 push as
remote registration evidence. 다음을 하지 않는다: hide the 패키지 from CI 또는 make the new job optional.

- [ ] **단계 3: 실행 the pre-release soak under the admission rule**

```bash
/usr/bin/time -p go test -p 1 -count=10 ./leader/etcd
```

예상: PASS when the measured admission rule permits local execution; otherwise record it as a
nightly/manual pre-release gate 및 do 아님 place the repeated cold-container run in the normal PR
job.

- [ ] **단계 4: 실행 단계 6-R 및 단계 7-R exact-head reviews**

실행 six independent lanes at the same exact commit for 성능, 안정성, 보안,
운영자/Ops, 개발자/API, 및 사용자/호출자, then perform the main-session integration review.
Repair every P0/P1 및 rerun 모든 six at the new exact head. 기록 `P0=0 P1=0`, commands, durations,
module graph, docs parity, unresolved P2/P3, 및 review provenance in the 단계 6-R artifact. After
push 및 live CI, repeat the exact-head PR review for 단계 7-R.

- [ ] **단계 5: 커밋 review evidence without claiming live CI**

```bash
git add docs/superpowers/reviews/2026-07-19-issue-537-etcd-leader-step-6r-code-review.md
git commit -m "Record the etcd implementation verdict" \
  --trailer "Constraint: Merge readiness requires exact-head local and live review evidence." \
  --trailer "Confidence: high" \
  --trailer "Scope-risk: narrow" \
  --trailer "Tested: targeted, race, lint, module, full CI, and six-lane Step 6-R gates" \
  --trailer "Not-tested: GitHub CI remains pending until the approved PR is pushed."
```

예상: the branch is clean, 모든 local checks pass, the Type A lesson exists, 및 PR creation may
use the already authorized `bluetape4k/bluetape-go` `develop` <- `feat/issue-537-etcd-leader`
scope. Stop at merge-ready 후 exact-head CI/review evidence 및 obtain a fresh merge approval.
