# provider conformance 계약 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 필수 하위 스킬: 사용 superpowers:subagent-driven-development (권장) 또는 superpowers:executing-plans to 이 계획을 작업 단위로 구현. 단계는 checkbox (`- [ ]`) 추적 문법을 사용.

**목표:** 추가 reusable, mandatory leader/lock/rate-limit conformance runners 및 make every 기존 provider pass the same observable 계약.

**아키텍처:** Three 공개 테스트-helper packages expose factory/harness runners 및 in-memory reference fixtures; provider-specific 테스트 adapters supply 만 construction, deterministic fault gates, 및 backend probes. Production leader providers share one blocking campaign/state/오류 계약, while lock 및 rate-limit providers expose 없음 new production abstraction 및 use private default-nil mutation hooks 만 where a real in-flight boundary cannot otherwise be controlled.

**기술 스택:** Go 1.26, standard `testing`/`context`/`errors`/`sync`, go-redis v9, MongoDB Go driver v2, Testcontainers Redis/MongoDB, 기존 bluetape-go concurrency 테스트 helpers.

---

## 파일 지도

| Area | 파일 | 책임 |
|---|---|---|
| Leader/Redis 오류 core | `leader/errors.go`, `leader/errors_test.go`, `leader/options.go`, `leader/options_test.go`, `redis/errors.go`, `redis/errors_test.go` | State/invalid-context/commit-unknown sentinels, sanitized operation 오류, structural identity validation. |
| Leader helper | `leader/leadertest/{doc.go,harness.go,memory.go,runner.go,harness_test.go,runner_test.go,example_test.go,README.md,README.ko.md}` | Public mandatory harness, reference fixture, black-box runner, self-테스트 및 docs. |
| Redis leader | `leader/elector.go`, `leader/redis/elector.go`, `leader/redis/elector_test.go`, `leader/redis/coordination_example_test.go`, `leader/redis/conformance_test.go`, `leader/redis/conformance_internal_test.go` | Blocking campaign GoDoc/example, bounded retry, cleanup-pending resign state, real Redis adapter/control. |
| Mongo leader | `leader/mongo/elector.go`, `leader/mongo/elector_test.go`, `leader/mongo/conformance_test.go` | Common state/오류 semantics 및 Mongo adapter/control. |
| 고정 helper | `lock/locktest/{doc.go,harness.go,memory.go,runner.go,harness_test.go,runner_test.go,example_test.go,README.md,README.ko.md}` | Function adapter 계약, 전에/후 gates, reference fixture 및 exact lock runner. |
| Redis lock | `lock/redis/options.go`, `lock/redis/options_test.go`, `lock/redis/mutex.go`, `lock/redis/mutex_test.go`, `lock/redis/example_test.go`, `lock/redis/conformance_test.go` | Byte-preserving owner validation, cancellation linearization, 호출자 recovery example 및 mandatory Redis lock adapter. |
| Rate helper | `ratelimit/ratelimittest/{doc.go,harness.go,memory.go,runner.go,harness_test.go,runner_test.go,example_test.go,README.md,README.ko.md}` | Parent-independent neutral result/function 계약 및 exact token-bucket runner. |
| Local rate limit | `ratelimit/token_bucket.go`, `ratelimit/token_bucket_test.go`, `ratelimit/conformance_test.go` | Private mutation hook, cancellation arbitration 및 local adapter. |
| Redis rate limit | `ratelimit/redis/limiter.go`, `ratelimit/redis/limiter_test.go`, `ratelimit/redis/conformance_test.go` | Command-boundary cancellation arbitration 및 Redis adapter. |
| Public docs/release | `leader/{README.md,README.ko.md}`, provider README pairs, `ratelimit/{README.md,README.ko.md}`, `CHANGELOG.md` | Blocking/migration semantics, helper usage, caveat matrix, rollout/rollback. |

## 의존 순서 And Write Ownership

작업 0A records risk 및 성능 baselines 전에 source work. Tasks 1-3 establish leader core/helper APIs. Tasks 4-5 consume them 및 may 아님 start 전에 작업 3 is green. 작업 6 establishes `locktest`; 작업 7 consumes it. 작업 8 establishes `ratelimittest`; Tasks 9-10 consume it. 작업 11 edits README/CHANGELOG 만 후 APIs 및 compile-checked helper example settle. 작업 12 finalizes verification/review artifacts. 다음을 하지 않는다: run Tasks 4-5, 6-7, 또는 8-10 concurrently when they touch the same provider fixture/client.

사용 `bttesting.Eventually`/`Consistently` for bounded eventual state 및 `testing/concurrency` stress helpers for exact concurrent totals. Raw goroutines/channels are al낮음ed 만 for deterministic gate handshakes because 없음 기존 helper exposes a 전에/후-linearization barrier; every such goroutine has a buffered result channel, 호출자 deadline, 및 `t.Cleanup` resume/cancel.

### 작업 0: 고정 Approved Workf낮음 Artifacts

**복잡도:** 낮음
**Depends on:** none
**패턴:** `bluetape-workf낮음`, `bluetape-full-feature`

**파일:**
- 검증: `docs/superpowers/specs/2026-07-12-issue-527-provider-conformance-design.md`
- 검증: `docs/superpowers/plans/2026-07-12-issue-527-provider-conformance-plan.md`

- [ ] **단계 1: 검증 artifact-만 branch state**

실행:

```bash
git diff --check
git status --short
git log --oneline origin/develop..HEAD
```

예상: 없음 whitespace 오류; 만 approved spec/plan/review artifacts differ from `origin/develop`; 없음 source file is modified.

- [ ] **단계 2: 커밋 만 review amendments 전에 source edits**

```bash
git add docs/superpowers/specs/2026-07-12-issue-527-provider-conformance-design.md \
  docs/superpowers/plans/2026-07-12-issue-527-provider-conformance-plan.md
git diff --cached --quiet || git commit -m "docs: amend provider conformance plan"
```

예상: the already committed plan is present. If 단계 3-R amendments are uncommitted, commit 만 those amendments; otherwise do 아님 create an empty duplicate plan commit. Roll back this task by reverting 만 the plan-amendment commit; never discard the approved spec.

### 작업 0A: Pre-Implementation 위험 And Performance Baseline

**복잡도:** 보통
**Depends on:** 작업 0
**패턴:** `bluetape-full-feature` 단계 3-P, `bluetape-go-patterns`

**파일:**
- 생성: `docs/superpowers/reviews/2026-07-12-issue-527-provider-conformance-risk.md`

- [ ] **단계 1: 기록 risks, signals, mitigation, 및 rerun/rollback owner**

생성 the risk table 전에 any source edit. Include late acquisition, dispatch-후-commit response loss, resign/renew overlap, retry storm, false gate PASS, import cycle, key/owner migration, Testcontainers leak, hot-path allocation, 및 secret leakage. Distinguish bare pre-dispatch context 오류 from typed commit-indeterminate provider 오류; Redis rate-limit automatic replay is explicitly rejected.

- [ ] **단계 2: 캡처 the local TokenBucket benchmark baseline**

```bash
go test -run '^$' -bench 'TokenBucket' -benchmem -count=5 ./ratelimit \
  | tee /tmp/issue-527-token-bucket-before.txt
```

예상: five successful samples 함께 `allocs/op` recorded. 작업 9 compares the same command 후 the private nil hook; any additional allocation blocks progression.

- [ ] **단계 3: Persist the baseline in the risk artifact**

Under `## TokenBucket Baseline`, record `go version`, the exact benchmark command, 및 모든 five `ns/op`, `B/op`, 및 `allocs/op` rows from `/tmp/issue-527-token-bucket-before.txt`. The committed artifact, 아님 `/tmp`, is the authoritative restart-safe baseline.

- [ ] **단계 4: 커밋 the pre-implementation evidence**

```bash
git add docs/superpowers/reviews/2026-07-12-issue-527-provider-conformance-risk.md
git commit -m "docs: predict provider conformance risks"
```

예상: risk evidence predates every source-code commit.

### 작업 1: Leader Core 검증 And Error Contracts

**복잡도:** 보통
**Depends on:** 작업 0A
**패턴:** `bluetape-go-patterns`, `test-driven-development`

**파일:**
- Modify: `leader/errors.go`
- 생성: `leader/errors_test.go`
- Modify: `leader/options.go`
- 생성: `leader/options_test.go`
- Modify: `redis/errors.go`
- Modify: `redis/errors_test.go`

- [ ] **단계 1: Write failing table 테스트 for structural identities 및 safe 오류**

추가 테스트 함께 these exact cases:

```go
func TestOptionsNormalizeRejectsUnsafeIdentity(t *testing.T) {
    tests := []struct {
        name string
        opts leader.Options
    }{
        {"group delimiter", leader.Options{Group: "a:b", MemberID: "m"}},
        {"member hash tag", leader.Options{Group: "g", MemberID: "m{1}"}},
        {"prefix empty segment", leader.Options{Group: "g", MemberID: "m", KeyPrefix: "a::b"}},
        {"control", leader.Options{Group: "g\n", MemberID: "m"}},
        {"group bytes", leader.Options{Group: strings.Repeat("g", 257), MemberID: "m"}},
        {"member bytes", leader.Options{Group: "g", MemberID: strings.Repeat("m", 257)}},
        {"final key bytes", leader.Options{Group: strings.Repeat("g", 256), MemberID: "m", KeyPrefix: strings.Repeat("p", 256)}},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if _, err := tt.opts.Normalize(); err == nil { t.Fatal("expected validation error") }
        })
    }
}

func TestOperationErrorIsSanitizedAndUnwraps(t *testing.T) {
    cause := errors.New("secret endpoint credential")
    err := leader.NewOperationError("mongo", "campaign", cause)
    if !errors.Is(err, cause) { t.Fatal("cause must be preserved") }
    if strings.Contains(err.Error(), "secret") { t.Fatalf("leaked cause: %v", err) }
}

func TestOperationErrorNilAndZeroValuesAreSafe(t *testing.T) {
    var nilErr *leader.OperationError
    var zero leader.OperationError
    for _, err := range []*leader.OperationError{nilErr, &zero} {
        if err.Error() != "leader operation failed" || err.Unwrap() != nil ||
            err.Backend() != "unknown" || err.Operation() != "unknown" {
            t.Fatalf("unsafe fallback: %#v", err)
        }
    }
}

func TestLeaderSentinelsAreDistinct(t *testing.T) {
    sentinels := []error{
        leader.ErrAlreadyLeader, leader.ErrNotLeader, leader.ErrCampaignInProgress,
        leader.ErrCleanupPending, leader.ErrInvalidContext, leader.ErrCommitUnknown,
    }
    for i, left := range sentinels {
        for j, right := range sentinels {
            if i == j && !errors.Is(left, right) { t.Fatalf("sentinel %d must match itself", i) }
            if i != j && errors.Is(left, right) { t.Fatalf("sentinels %d and %d overlap", i, j) }
        }
    }
}
```

Also assert `redis.ErrCommitUnknown` is distinct from validation sentinels, leading/trailing whitespace is rejected rather than trimmed, internal valid Unicode bytes 및 canonically equivalent forms remain byte-distinct, final key length `512` passes, 및 nil/blank/control/>32-byte `NewOperationError` metadata is rejected.

- [ ] **단계 2: Observe RED**

실행:

```bash
go test -count=1 ./leader ./redis -run 'OptionsNormalize|OperationError|Sentinels|CommitUnknown'
```

예상: FAIL because `ErrInvalidContext`, state sentinels, `OperationError`, 및 structural validation do 아님 exist.

- [ ] **단계 3: 추가 the minimal core contracts**

구현 exported sentinels 함께 GoDoc:

```go
var (
    ErrAlreadyLeader      = errors.New("leader: already leader")
    ErrNotLeader          = errors.New("leader: not leader") // legacy contention sentinel
    ErrCampaignInProgress = errors.New("leader: campaign in progress")
    ErrCleanupPending     = errors.New("leader: cleanup pending")
    ErrInvalidContext     = errors.New("leader: invalid context")
    ErrCommitUnknown      = errors.New("leader: commit unknown")
)
```

추가 `redis.ErrCommitUnknown = errors.New("redis: commit unknown")`. Join these sentinels 만 when post-dispatch reconciliation cannot determine commit; confirmed absence/different-owner failures remain typed but do 아님 match them.

추가 private fields to `OperationError`, validate constructor labels, never call `cause.Error()`, 및 make every method nil-safe. In `Options.Normalize`, validate byte lengths 및 structural segments 전에 defaults; use `unicode.IsControl`, reject `: { }` in group/member, reject empty/control/braced prefix segments, 및 ensure `len(KeyPrefix)+1+len(Group) <= 512` without trimming 또는 Unicode normalization.

- [ ] **단계 4: 검증 GREEN 및 commit**

```bash
gofmt -w leader/errors.go leader/errors_test.go leader/options.go leader/options_test.go redis/errors.go redis/errors_test.go
go test -count=1 ./leader ./redis
git add leader/errors.go leader/errors_test.go leader/options.go leader/options_test.go redis/errors.go redis/errors_test.go
git commit -m "feat: define leader provider contracts"
```

예상: PASS. Revert this commit if valid 기존 key bytes change in the preservation table.

### 작업 2: Public `leader/leadertest` Harness And Reference Fixture

**복잡도:** 높음
**Depends on:** 작업 1
**패턴:** `bluetape-go-patterns`, `test-driven-development`

**파일:**
- 생성: `leader/leadertest/doc.go`
- 생성: `leader/leadertest/harness.go`
- 생성: `leader/leadertest/memory.go`
- 생성: `leader/leadertest/harness_test.go`

- [ ] **단계 1: Write RED API/validation 테스트**

Compile-check this exact 공개 shape:

```go
type Operation string
const (
    OperationCampaign Operation = "campaign"
    OperationRenew Operation = "renew"
    OperationResign Operation = "resign"
)
type Factory func(testing.TB, leader.Options) (leader.Elector, error)
type Control interface {
    ReplaceOwner(context.Context, leader.Options, string) error
    FailNext(context.Context, leader.Options, Operation, error) error
    Owner(context.Context, leader.Options) (string, error)
    OperationCount(leader.Options, Operation) int64
}
type Harness struct { New Factory; Control Control }
func MemoryHarness() Harness
```

Test nil harness fields, invalid direct control options/owner/operation/원인, nil/pre-canceled contexts, monotonic counts, exactly-one post-linearization `FailNext` for campaign/renew/resign, stale replacement, 및 없음 mutation on rejected controls. A failed campaign response must leave a probe-visible committed owner so the runner can distinguish typed commit-unknown from a bare context 오류.

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./leader/leadertest
```

예상: build FAIL because the 패키지 is absent.

- [ ] **단계 3: 구현 the harness 및 mutex-protected memory lease store**

사용 one normalized identity key 및 a record containing `owner`, `leaseUntil`, injected campaign/renew/resign causes, 및 cumulative operation counters. The memory elector must own its renewal goroutine/timer, cancel it on loss/resign, keep cleanup-pending token state 후 delete failure 또는 campaign commit-unknown, 및 compare owner 전에 delete. Control 오류 must be sanitized 및 must 아님 print raw identities.

Core validation must fol낮음 this shape:

```go
func validateContext(ctx context.Context) error {
    if ctx == nil { return leader.ErrInvalidContext }
    if err := ctx.Err(); err != nil { return err }
    return nil
}
```

- [ ] **단계 4: 검증 fixture race safety 및 commit**

```bash
gofmt -w $(rg --files leader/leadertest -g '*.go')
go test -count=1 ./leader/leadertest -run 'Harness|Memory|Control'
go test -race -count=1 ./leader/leadertest
git add leader/leadertest
git commit -m "test: add leader conformance harness"
```

예상: PASS 함께 없음 race. If a goroutine remains 후 cleanup, stop 및 fix ownership 전에 작업 3.

### 작업 3: Leader Black-Box Runner And Self-Tests

**복잡도:** 높음
**Depends on:** 작업 2
**패턴:** `bluetape-go-patterns`, `test-driven-development`

**파일:**
- 생성: `leader/leadertest/runner.go`
- 생성: `leader/leadertest/runner_test.go`
- 생성: `leader/leadertest/example_test.go`

- [ ] **단계 1: 추가 runner self-테스트 that intentionally break each evaluator**

사용 `MemoryHarness()` as the passing reference, then exercise 패키지-private evaluator functions 함께 controlled records that represent these named failures: two simultaneous winners, owner-insensitive stale resign, ignored cancellation, renewal continuing 후 injected loss, skipped cleanup retry, raw marker in diagnostic, 및 operation count that always returns zero. Each evaluator returns an `error`; `Run` is 만 the thin `t.Run`/`t.Fatal` shell. 다음을 하지 않는다: attempt to implement `testing.TB` (its private method prevents external fakes), 및 do 아님 let broken factories replace the real provider in provider 테스트.

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./leader/leadertest -run 'Run|Broken'
```

예상: FAIL because `Run` 및 named 계약 cases are absent.

- [ ] **단계 3: 구현 `Run` 함께 mandatory named cases**

생성 subtests named exactly:

```go
var cases = []string{
    "acquire-observe", "owned-duplicate", "campaign-in-progress",
    "contention-cancel", "campaign-lost-response", "renewal", "renew-failure", "owner-loss",
    "expiry-takeover", "resign-idempotent", "resign-retry",
    "stale-resign", "exact-contention", "nil-context", "redaction",
}
```

Normalize options once per case; use 공개 elector methods for normal observations 및 Control 만 for fault/probe/count. All waits use 호출자-owned deadlines 및 eventual assertions; exact contention asserts `successes==1`, `maxActive==1`, bounded losers, 및 one takeover 후 release. Register bounded resign cleanup immediately 후 acquire.

`campaign-lost-response` injects a context/transport 원인 만 후 owner creation. It accepts 만 confirmed success from bounded owner-token reconciliation 또는 a typed provider wrapper 함께 the context/provider causes preserved; a bare context 오류 while the owner remains is a deterministic failure.
Probe failure must satisfy `errors.Is(err, leader.ErrCommitUnknown)`; confirmed absent/different-owner failure must 아님. Both remain `errors.As(*leader.OperationError)`.

- [ ] **단계 4: 추가 the compile-checked helper example**

In `example_test.go`, define a complete `MemoryHarness` adapter setup 및 a non-output `ExampleRun` that captures a `func(t *testing.T) { Run(t, harness) }` closure without executing it. This compile-checks the call without fabricating `testing.T`. Include bounded context 및 cleanup code in the closure body.

- [ ] **단계 5: 검증 및 commit**

```bash
gofmt -w $(rg --files leader/leadertest -g '*.go')
go test -count=1 ./leader/leadertest
go test -race -count=1 ./leader/leadertest
git add leader/leadertest
git commit -m "test: define leader conformance runner"
```

예상: reference harness PASS; every intentionally broken harness fails its intended named case.

### 작업 4: Redis Single-Leader State Machine And Conformance Adapter

**복잡도:** 높음
**Depends on:** 작업 3
**패턴:** `bluetape-go-patterns`, `test-driven-development`, `systematic-debugging` on any timing failure

**파일:**
- Modify: `leader/elector.go`
- Modify: `leader/redis/elector.go`
- Modify: `leader/redis/elector_test.go`
- Modify: `leader/redis/coordination_example_test.go`
- Modify: `leader/redis/resign_internal_test.go`
- 생성: `leader/redis/conformance_test.go`
- 생성: `leader/redis/conformance_internal_test.go`

- [ ] **단계 1: 교체 immediate-contention expectations 함께 RED state-machine 테스트**

추가 테스트 asserting: contender blocks until deadline 및 returns `DeadlineExceeded`; owner resign permits takeover; concurrent same-instance call returns `ErrCampaignInProgress`; cleanup-pending returns `ErrCleanupPending`; nil contexts return `ErrInvalidContext` 전에 Redis commands; injected delete failure is retryable; one already-linearized late renew is al낮음ed 후 resign deadline 및 없음 later renew occurs; provider 오류 satisfy both `*leader.OperationError` 및 기존 `*btredis.OpError` without marker leakage. 마이그레이션 `coordination_example_test.go` from `Campaign(context.Background())`/`ErrNotLeader` to a bounded context 및 `DeadlineExceeded`, 및 update `leader.Elector` plus Redis `Campaign` GoDoc to say Campaign blocks until acquisition 또는 context termination, document 모든 local-state sentinels, 및 require `ErrCommitUnknown` recovery through bounded `Resign`/TTL 전에 another campaign.

추가 lost-response cases where SetNX commits the owner token 및 the 테스트 hook returns a context/transport 오류. 검증 bounded GET reconciliation returns success when own token is visible; when the probe is unavailable it returns the typed wrapper matching both `leader.ErrCommitUnknown` 및 `btredis.ErrCommitUnknown` while retaining cleanup-pending/token state so bounded `Resign` can compare-delete 또는 TTL cleanup. Confirmed absent/different owner 오류 match neither sentinel. It must never return a bare context 오류 함께 an owner record.

- [ ] **단계 2: Observe RED**

```bash
go test -p 1 -count=1 ./leader/redis -run 'Campaign|Resign|OperationError|Conformance'
```

예상: FAIL on immediate `ErrNotLeader`, nil context normalization, lost cleanup state, 및 missing common wrapper/runner.

- [ ] **단계 3: 구현 the blocking Redis state machine**

교체 loose booleans 함께 guarded `owned`, `campaigning`, `cleanupPending`, `cancel`, `done`, token state. Retry SetNX 함께 25ms base exponential delay, 250ms cap, owner-token deterministic ±20% jitter, cancel-aware timers, 및 at most 12 attempts per one-second contention window. Check context 전에 dispatch; if a successful response linearizes first, return success 및 start renewal. On a dispatched 오류, perform one bounded fresh-context GET: own token confirms success, absent/different owner confirms 없음 acquisition, 및 probe failure moves to cleanup-pending 함께 the retained token 및 joins `leader.ErrCommitUnknown` plus `btredis.ErrCommitUnknown` into the typed provider chain. Never convert a dispatched `*btredis.OpError` into a bare context 오류. Resign transitions to cleanup-pending 전에 stopping renewal 및 retains token/done state until compare-delete success/mismatch. Wrap Redis failures as:

```go
return leader.NewOperationError("redis", operation,
    btredis.NewOpError(btredis.OpLabels{Family: "leader redis", Operation: operation}, e.key, cause))
```

사용 패키지-owned labels 만.

- [ ] **단계 4: 구성 the real Redis harness adapter**

사용 a dedicated ready/pinged Testcontainers client, unique prefix per subtest, command hook/interceptor for exact campaign/renew/resign post-linearization response loss/count, 및 a separate control client for owner replacement/probe. Cleanup order is elector → client → container; every stage has a bounded context 및 reports 오류. 추가 a setup/subtest-failure 테스트 that still attempts both client close 및 container termination 및 verifies the fixture process/container is gone. No `t.Parallel`.

- [ ] **단계 5: 검증 targeted behavior, race, command budget, 및 commit**

```bash
gofmt -w $(rg --files leader/redis -g '*.go') leader/elector.go
go test -p 1 -count=1 ./leader/redis
go test -p 1 -race -count=1 ./leader/redis
git add leader/elector.go leader/redis
git commit -m "feat: conform redis leader election"
```

예상: runner PASS, retry command budget ≤12/second, 없음 late owner 후 cancellation, 없음 race/goroutine leak. Roll back the commit if 기존 valid Redis key/token bytes change.

### 작업 5: Mongo Single-Leader Conformance And Sanitized Errors

**복잡도:** 높음
**Depends on:** 작업 3
**패턴:** `bluetape-go-patterns`, `test-driven-development`

**파일:**
- Modify: `leader/mongo/elector.go`
- Modify: `leader/mongo/elector_test.go`
- 생성: `leader/mongo/conformance_test.go`

- [ ] **단계 1: 추가 RED 테스트 for common state/오류 behavior**

검증 nil contexts fail 전에 collection calls, campaigning 및 cleanup-pending use distinct sentinels, failed resign preserves retry state, renewal failure/lost owner stops traffic, 및 injected driver marker is absent from rendering while `errors.As` still reaches the Mongo driver 원인 through `*leader.OperationError`. Post-linearization response loss must match `leader.ErrCommitUnknown`; confirmed failure must 아님. 추가 a blocked renew boundary: resign deadline may return while at most one already-linearized renew completes, 없음 new renew is scheduled, cleanup-pending/token state remains, a later bounded resign retries successfully 또는 TTL expires, 및 a contender eventually takes over.

- [ ] **단계 2: Observe RED**

```bash
go test -p 1 -count=1 ./leader/mongo -run 'Elector|OperationError|Conformance'
```

예상: FAIL because Mongo normalizes nil context, conflates local states, clears ownership 전에 failed cleanup, 및 exposes raw driver text.

- [ ] **단계 3: 구현 the same guarded state transitions**

Adopt the Redis state names 및 sentinel decisions without porting Redis storage/retry code. 유지 Mongo document schema 및 clocks unchanged. 추가 a default-nil private 테스트 control hook at actual campaign/renew/resign boundaries for deterministic post-linearization response loss/count 및 a renew gate. The worker records an in-flight renew 전에 dispatch, stops scheduling immediately on resign, al낮음s at most that one attempt to finish within `RenewInterval`, 및 retains cleanup state until retry/TTL. The hook is unexported 및 unreachable by callers. Wrap provider failures 함께 `leader.NewOperationError("mongo", operation, cause)` 및 preserve driver `errors.Is`/`errors.As`.

- [ ] **단계 4: 실행 the common runner against a real Mongo fixture**

생성 a unique collection, wait for a bounded ping/command probe, assign the private 테스트 control to each elector made by the factory, 및 implement owner replacement/probe against the collection. Disconnect client 전에 container termination; attempt/report both cleanups even when one fails. 추가 a deliberate setup/subtest failure path that verifies disconnect 및 termination were both attempted 및 없음 fixture process/container remains.

- [ ] **단계 5: 검증 및 commit**

```bash
gofmt -w $(rg --files leader/mongo -g '*.go')
go test -p 1 -count=1 ./leader/mongo
go test -p 1 -race -count=1 ./leader/mongo
git add leader/mongo
git commit -m "feat: conform mongo leader election"
```

예상: the same `leadertest.Run` cases pass for Redis 및 Mongo; BSON schema 및 valid identity keys remain unchanged.

### 작업 6: Public `lock/locktest` Harness, Gates, And Reference Runner

**복잡도:** 높음
**Depends on:** 작업 0A
**패턴:** `bluetape-go-patterns`, `test-driven-development`

**파일:**
- 생성: `lock/locktest/doc.go`
- 생성: `lock/locktest/harness.go`
- 생성: `lock/locktest/memory.go`
- 생성: `lock/locktest/runner.go`
- 생성: `lock/locktest/harness_test.go`
- 생성: `lock/locktest/runner_test.go`
- 생성: `lock/locktest/example_test.go`

- [ ] **단계 1: Write RED compile/validation/gate 테스트**

정의 the approved `Config`, `ReleaseFunc`, `AcquireFunc`, `Factory`, `OperationAcquire/Release`, `PhaseBefore/AfterLinearize`, `Gate`, `Control` including post-linearization `FailNext`, mandatory neutral `ErrorClassifier`, `Harness`, `Run`, 및 `MemoryHarness`. Test invalid config/operation/phase/원인, nil/pre-canceled contexts, nil returned gate/functions/classifier, idempotent non-blocking `Resume`, `AwaitStarted` cancellation, count fallback 0, exactly-one lost response, 및 cleanup auto-resume. The runner calls `IsProviderError(err)` instead of importing any concrete provider. Named harness-validation 테스트 require false for nil/bare context/validation/raw 원인 및 true for actual/nested typed wrappers; nil, always-true, always-false 및 panicking classifiers must fail under a recover boundary.

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./lock/locktest
```

예상: build FAIL because the 패키지 is absent.

- [ ] **단계 3: 구현 reference fixture 및 mandatory runner cases**

사용 a mutex-protected `{owner, expiresAt}` map 및 one-shot 전에/후 gates. Named cases are:

```go
var cases = []string{
    "acquire-release", "contention", "repeated-release", "expiry-takeover",
    "pre-canceled-acquire", "pre-canceled-release", "cancel-before-linearize",
    "cancel-after-linearize", "lost-response", "stale-release", "exact-contention",
}
```

Before-linearize cancellation returns context 오류 함께 nil/false 및 zero owner/count delta; 후-linearize cancellation returns the successful release/result. Exact stress asserts one success, `workers-1` provider sentinels, `maxActive==1`, then one takeover.

The lost-response case commits acquire/release then injects an 오류. It rejects a bare context 오류 when an owner mutation occurred 및 requires either owner-token-confirmed success/cleanup 또는 `IsProviderError(err)==true`. Acquire tuples are own token `(release,nil)`, absent/different `(nil,typed error)`, probe failure `(release,typed error)`. Release lost-response returns `(false,typed error)`; retrying the same callback returns `(false,nil)` 후 prior delete 또는 owner replacement 및 never deletes the replacement.
추가 a broken adapter/classifier that includes `raw-key`, `raw-owner`, endpoint 및 injected-원인 markers in its diagnostic; the evaluator must fail it without echoing any marker in captured runner output.

- [ ] **단계 4: 추가 `ExampleRun` 함께 a complete MemoryHarness adapter**

The non-output example captures but does 아님 execute a `func(*testing.T)` closure. It compile-checks both gate phases, `FailNext`, bounded context, idempotent cleanup resume, contention sentinel handling, 및 does 아님 fabricate a `testing.T` at runtime.

- [ ] **단계 5: 검증 self-테스트 및 commit**

```bash
gofmt -w $(rg --files lock/locktest -g '*.go')
go test -count=1 ./lock/locktest
go test -race -count=1 ./lock/locktest
git add lock/locktest
git commit -m "test: add lock conformance runner"
```

예상: memory harness PASS; owner-ignorant release 및 wrapper-만 fake adapters fail deterministically.

### 작업 7: Redis 고정 Linearization And Common Runner

**복잡도:** 보통
**Depends on:** 작업 6
**패턴:** `bluetape-go-patterns`, `test-driven-development`

**파일:**
- Modify: `lock/redis/options.go`
- 생성: `lock/redis/options_test.go`
- Modify: `lock/redis/mutex.go`
- Modify: `lock/redis/mutex_test.go`
- Modify: `lock/redis/example_test.go`
- 생성: `lock/redis/conformance_test.go`

- [ ] **단계 1: 추가 RED in-flight cancellation 테스트 및 harness invocation**

Gate SetNX/Eval 전에 및 후 server execution. Cancel 전에 command dispatch 및 assert 없음 key; cancel 후 successful server reply but 전에 method return 및 assert a lease/`true,nil`; prove stale lease never deletes a replacement owner. 추가 leading/trailing-space 및 Unicode owner tokens 및 assert `Options.Token`, returned lease token, Redis value 및 owner probe are byte-identical rather than trimmed. Inject an acquire post-linearization lost response 및 assert bounded owner-token reconciliation returns confirmed success 또는 a non-nil owner-aware lease plus typed `*btredis.OpError` matching `btredis.ErrCommitUnknown`, never a bare context 오류 함께 a committed key. Confirmed absent/different owner 오류 do 아님 match the sentinel. Inject an unlock lost response 후 delete: first call is `(false, typed error)` matching the sentinel, the same callback retry is `(false,nil)`, 및 a replacement owner is untouched. 사용 forbidden markers in 원인/key/token/endpoint 및 assert `errors.As`/sentinel preservation while rendered 오류 및 captured runner output contain none. The 오류-path lease must compare-delete 만 its own token 및 be safe 후 takeover.

- [ ] **단계 2: Observe RED**

```bash
go test -p 1 -count=1 ./lock/redis -run 'Linearization|Conformance'
```

예상: FAIL because post-command cancellation linearization is unproved 및 없음 common harness exists.

- [ ] **단계 3: 구현 context arbitration 및 Redis adapter**

보존 the 기존 lock nil-context compatibility. Change token validation to reject 모든-blank values without trimming valid bytes. 사용 a private helper that returns a bare context 오류 만 전에 dispatch. Once SetNX/Eval produced a successful result, return it even if cancellation races afterward. On a dispatched acquire 오류, probe 함께 a short fresh context 및 the known owner token: own token confirms acquire success, absent/different owner confirms 없음 acquire, 및 probe failure returns a constructed owner-aware `*Lease` together 함께 typed `*btredis.OpError` as commit-indeterminate. On a dispatched unlock 오류, preserve the same Lease callback; retry compare-delete treats already-absent/owner-mismatch as `(false,nil)`. 업데이트 `TryLock`/`Unlock` GoDoc 및 `example_test.go` 함께 type-first `ErrCommitUnknown`, immediate bounded cleanup whenever Lease is non-nil, same-lease Unlock retry, 및 TTL fallback. The 테스트 adapter converts an acquire 오류-path lease to a non-nil release callback, implements `IsProviderError` 함께 `errors.As(*btredis.OpError)`, uses go-redis hooks on a dedicated client for gate/fail/count 및 a control client for GET/SET, 및 maps `ErrNotAcquired` unchanged. 추가 a failure-path cleanup 테스트 that closes the client, terminates the container independently, reports both 오류, 및 verifies 없음 fixture remains.

- [ ] **단계 4: 검증 및 commit**

```bash
gofmt -w $(rg --files lock/redis -g '*.go')
go test -p 1 -count=1 ./lock/redis
go test -p 1 -race -count=1 ./lock/redis
git add lock/redis
git commit -m "test: conform redis lock provider"
```

예상: `locktest.Run` PASS, exact contention totals, 없음 key 후 전에-linearize cancellation, owner-aware cleanup preserved.

### 작업 8: Parent-Independent `ratelimit/ratelimittest` Runner

**복잡도:** 높음
**Depends on:** 작업 0A
**패턴:** `bluetape-go-patterns`, `test-driven-development`

**파일:**
- 생성: `ratelimit/ratelimittest/doc.go`
- 생성: `ratelimit/ratelimittest/harness.go`
- 생성: `ratelimit/ratelimittest/memory.go`
- 생성: `ratelimit/ratelimittest/runner.go`
- 생성: `ratelimit/ratelimittest/harness_test.go`
- 생성: `ratelimit/ratelimittest/runner_test.go`
- 생성: `ratelimit/ratelimittest/example_test.go`

- [ ] **단계 1: Write RED API 및 import-cycle 테스트**

The helper must 아님 import `github.com/bluetape4k/bluetape-go/ratelimit`. 정의 its own field-identical `Result`, `Al낮음Func`, `Factory`, 전에/후 `Phase`, `Gate`, `Control` including post-linearization `FailNext`, mandatory neutral `ErrorClassifier`, `Harness`, `Run`, 및 `MemoryHarness`. Test positive finite rate, positive burst, IdleTTL ≥ full-refill duration when nonzero, invalid key/phase/원인/count behavior, nil functions/gates/classifier, 및 context behavior. Cover idempotent/non-blocking `Resume`, nil/pre-canceled `AwaitStarted`, automatic `t.Cleanup` resume, exactly-one fail injection, 및 a deterministic abandoned-gate 테스트 proving the operation goroutine exits. The runner uses `IsProviderError` 및 never imports Redis 또는 the parent 패키지. Negative/positive classifier controls 및 nil/always-true/always-false/panic rejection match `locktest` exactly.

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./ratelimit/ratelimittest
```

예상: build FAIL because the 패키지 is absent.

- [ ] **단계 3: 구현 memory bucket 및 exact runner**

사용 a controllable clock 및 mutex-protected buckets. Named cases:

```go
var cases = []string{
    "initial-burst", "over-burst-validation", "rejection-result", "refill",
    "key-isolation", "pre-canceled", "cancel-before-linearize",
    "cancel-after-linearize", "lost-response", "exact-concurrency",
}
```

Exact concurrency uses a frozen/없음-refill window 및 asserts `al낮음ed==Burst`, `rejected==requests-Burst`, 및 admitted token sum `Burst`. Before cancellation leaves a full burst; 후 cancellation returns success 및 the next request observes the exact prior debit.

`lost-response` debits once 및 injects an 오류 후 linearization. The memory fixture can confirm 및 return the result; a provider adapter that cannot replay returns zero `ratelimittest.Result` 함께 `IsProviderError(err)==true`, 및 the evaluator verifies one debit rather than automatically retrying.
추가 a broken adapter that renders raw key/endpoint/injected-원인 markers; the evaluator must reject it while its own captured diagnostic remains marker-free.

- [ ] **단계 4: 추가 the compile-checked `ExampleRun`**

캡처 but do 아님 execute a `func(*testing.T)` closure showing neutral result conversion, both gate phases, `FailNext`, a 호출자-owned timeout, `t.Cleanup`-style resume, 및 a complete MemoryHarness. The example has 없음 output 및 never constructs a fake `testing.T`.

- [ ] **단계 5: 검증 없음 parent import 및 commit**

```bash
gofmt -w $(rg --files ratelimit/ratelimittest -g '*.go')
! rg 'bluetape-go/ratelimit"' ratelimit/ratelimittest
go test -count=1 ./ratelimit/ratelimittest
go test -race -count=1 ./ratelimit/ratelimittest
git add ratelimit/ratelimittest
git commit -m "test: add rate limit conformance runner"
```

예상: PASS 및 없음 import-cycle risk.

### 작업 9: Local Token Bucket Real-Boundary Conformance

**복잡도:** 보통
**Depends on:** 작업 8
**패턴:** `bluetape-go-patterns`, `test-driven-development`

**파일:**
- Modify: `ratelimit/token_bucket.go`
- Modify: `ratelimit/token_bucket_test.go`
- 생성: `ratelimit/conformance_test.go`

- [ ] **단계 1: 추가 RED same-패키지 gate 및 common-runner 테스트**

In `package ratelimit`, import `ratelimit/ratelimittest`, translate every `Result` field, 및 wire a 테스트 controller to the real mutation boundary. 검증 cancellation 전에 mutation consumes nothing 및 cancellation 후 mutation returns successful result. Inject a post-linearization failure 및 prove the synchronous memory result is confirmed without a second debit. 추가 a compile 테스트 proving `ratelimittest` does 아님 create an import cycle.

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./ratelimit -run 'Linearization|Conformance'
```

예상: FAIL because `TokenBucket` has 없음 real-boundary hook 또는 cancellation arbitration at mutation.

- [ ] **단계 3: 추가 a private default-nil hook 및 arbitrate cancellation under the bucket lock**

사용 an unexported field 만 same-패키지 테스트 can set:

```go
type tokenBucketHook interface {
    beforeLinearize(context.Context, string) error
    afterLinearize(context.Context, string)
}
```

Call it immediately 전에/후 `l.buckets[key] = state`; the nil path performs 만 one branch 및 changes 없음 공개 layout 계약. If context wins 전에 assignment, return context 오류 without storing; 후 assignment, return computed result even if context is canceled.

- [ ] **단계 4: 검증 correctness, race, 및 benchmark non-regression signal**

```bash
gofmt -w ratelimit/token_bucket.go ratelimit/token_bucket_test.go ratelimit/conformance_test.go
go test -count=1 ./ratelimit
go test -race -count=1 ./ratelimit
go test -run '^$' -bench 'TokenBucket' -benchmem -count=5 ./ratelimit \
  | tee /tmp/issue-527-token-bucket-after.txt
git add ratelimit/token_bucket.go ratelimit/token_bucket_test.go ratelimit/conformance_test.go
git commit -m "test: conform local token bucket"
```

예상: runner/race PASS; compare `after.txt` sample-by-sample 함께 the committed `## TokenBucket Baseline` in the risk artifact. Any allocation added to the nil-hook hot path blocks progression; latency regression above 10% requires investigation/rerun 및 cannot be claimed as noise without fresh samples.

### 작업 10: Redis Rate Limiter Real-Command Conformance

**복잡도:** 보통
**Depends on:** 작업 8
**패턴:** `bluetape-go-patterns`, `test-driven-development`

**파일:**
- Modify: `ratelimit/redis/limiter.go`
- Modify: `ratelimit/redis/limiter_test.go`
- 생성: `ratelimit/redis/conformance_test.go`

- [ ] **단계 1: 추가 RED command-boundary cancellation 및 runner 테스트**

사용 a go-redis hook to pause 전에 Eval dispatch 및 후 successful Eval response. Before cancellation must leave 없음 bucket key 및 permit a full burst; 후 cancellation must return the successful neutral result 및 the next request must observe the debit. Inject an 오류 후 the Lua script debits 및 assert exactly one debit plus zero `ratelimittest.Result` 및 typed `*btredis.OpError` matching `btredis.ErrCommitUnknown`; never automatically replay the non-idempotent request. Put forbidden markers in 원인/key/endpoint 및 assert wrapper/sentinel preservation while rendered 오류 및 captured runner output remain marker-free. 보존 기존 server-time rounding 테스트.

- [ ] **단계 2: Observe RED**

```bash
go test -p 1 -count=1 ./ratelimit/redis -run 'Linearization|Conformance'
```

예상: FAIL because cancellation/result arbitration at the Redis script boundary is unspecified.

- [ ] **단계 3: 구현 minimal context arbitration 및 adapter conversion**

보존 the 기존 rate-limiter nil-context compatibility. Treat a parsed successful Lua result as linearized 및 return it even if cancellation arrives afterward; a bare context-오류 return must correspond to 없음 script dispatch. Any 오류 후 Eval dispatch remains typed `*btredis.OpError` 및 commit-indeterminate even when `errors.Is` sees the context 원인. 업데이트 공개 `Limiter.Al낮음` GoDoc to specify zero Result, possible debit, type-first `ErrCommitUnknown`, 없음 replay, 및 conservative full-refill wait. 다음을 하지 않는다: add idempotency fields, request replay, 또는 a second Eval. Adapter converts fields explicitly:

```go
return ratelimittest.Result{
    Allowed: r.Allowed, Requested: r.Requested, Remaining: r.Remaining,
    RetryAfter: r.RetryAfter, ResetAfter: r.ResetAfter,
}, err
```

사용 unique namespace/key, cumulative Eval counts, post-linearization fail injection, `IsProviderError` implemented 함께 `errors.As(*btredis.OpError)`, 및 없음 wrapper-만 gate. 추가 a setup/subtest-failure cleanup 테스트 that independently closes the client, terminates the container, reports both failures, 및 verifies 없음 fixture remains.

- [ ] **단계 4: 검증 및 commit**

```bash
gofmt -w $(rg --files ratelimit/redis -g '*.go')
go test -p 1 -count=1 ./ratelimit/redis
go test -p 1 -race -count=1 ./ratelimit/redis
git add ratelimit/redis
git commit -m "test: conform redis rate limiter"
```

예상: common runner PASS, exact burst totals, preserved Redis key/TTL/rounding 및 typed 오류 behavior.

### 작업 11: Public GoDoc, Examples, Locale Parity, And Release Migration

**복잡도:** 보통
**Depends on:** Tasks 4, 5, 7, 9, 10
**패턴:** `bluetape-go-patterns`, `bluetape-writer`, `bluetape-maintenance`

**파일:**
- 생성: `docs/superpowers/reviews/2026-07-12-issue-527-provider-conformance-caller-audit.md`
- 생성: helper README pairs listed in 파일 지도
- Modify: `leader/README.md`, `leader/README.ko.md`
- Modify: `leader/redis/README.md`, `leader/redis/README.ko.md`
- Modify: `leader/mongo/README.md`, `leader/mongo/README.ko.md`
- Modify: `lock/redis/README.md`, `lock/redis/README.ko.md`
- Modify: `ratelimit/README.md`, `ratelimit/README.ko.md`
- Modify: `ratelimit/redis/README.md`, `ratelimit/redis/README.ko.md`
- Modify: `CHANGELOG.md`

- [ ] **단계 1: 실행 및 record the 호출자 migration audit**

실행 the fol낮음ing searches against every first-party Go 호출자, inspect every hit, 및 record path, disposition 및 owner in the 호출자-audit artifact. A blocking `Campaign(context.Background())`/`Campaign(context.TODO())` call must gain a bounded 호출자-owned context unless it is a documented sole-owner lifecycle; legacy `ErrNotLeader` branches must migrate to deadline/new-state handling; nil contexts must become non-nil where the provider now rejects them; every `TryLock` 호출자 must check the typed/commit-unknown 오류 전에 bare context 오류 및 clean up whenever the returned Lease is non-nil; custom whitespace-bearing tokens must be recorded as byte-identity-sensitive. 기록 external consumers as a release-migration owner/status rather than claiming they were scanned.

```bash
rg -n 'Campaign\((context\.)?(Background|TODO)\(' --glob '*.go'
rg -n 'errors\.Is\([^,]+,\s*(leader\.)?ErrNotLeader|ErrNotLeader' --glob '*.go'
rg -n '\.(Campaign|Resign|Leader)\(nil\)' --glob '*.go'
rg -n 'TryLock\(' --glob '*.go'
rg -n 'Token:\s*"[^"]*"' lock --glob '*.go'
```

예상: every internal hit has an implemented migration 또는 an explicit justified 없음-change disposition; the artifact assigns CHANGELOG/0.19.0 migration ownership for external callers.

- [ ] **단계 2: 검증 the helper-owned compile-checked example 및 document them**

Tasks 3, 6 및 8 own each `ExampleRun`. 검증 they construct a 호출자-owned fixture, register cleanup immediately, call `Run`, 및 use unique identities/bounded contexts. 고정/rate example include 전에/후 gates, lost-response injection 및 counts; leader example include `FailNext`, owner probes, counts 및 bounded resign cleanup. README snippets mirror those compiled example, show contention/cancellation outcomes, 및 never print raw backend 오류.

- [ ] **단계 3: 문서화 exact migration 및 provider caveat matrix in 영문/한국어**

Cover blocking `Campaign`, legacy `ErrNotLeader`, new state/commit-unknown sentinels, nil-context rejection, structural identity audit, Redis lock token byte-preservation/없음-trim migration, common `OperationError`, 호출자 timeout ownership, bare pre-dispatch context versus typed commit-indeterminate 오류, owner-token reconciliation, rate-limit 없음-auto-replay 및 conservative full-refill wait (`Burst / RatePerSecond`), actual-mutation-boundary adapter rules, retry/clock precision non-guarantees, mixed-version generated-token compatibility 및 custom-token exception, telemetry labels, canary thresholds, resign/TTL rollback 및 Group/Strategic deferral. Error example perform `errors.As`/`ErrCommitUnknown` checks 전에 bare context `errors.Is`. 유지 every README pair section-for-section equivalent. In the 호출자-audit artifact, add a manual 영문/한국어 section mapping for every README pair 및 explicitly map the leader bounded-Resign/TTL, lock non-nil-Lease/same-callback retry/TTL, 및 rate-limit 없음-replay/full-refill recovery snippets.

- [ ] **단계 4: 추가 0.19.0 Unreleased changelog entry 및 verify docs**

```bash
go test -run 'Example' ./leader/... ./lock/... ./ratelimit/...
rg -n 'ErrCampaignInProgress|ErrCleanupPending|ErrInvalidContext|ErrCommitUnknown|OperationError|commit-unknown' \
  leader lock ratelimit CHANGELOG.md -g 'README*.md' -g 'CHANGELOG.md'
rg -n 'lease != nil|ErrCommitUnknown|full-refill|no replay|same callback|same lease' \
  leader lock ratelimit -g 'README*.md'
for base in leader/leadertest lock/locktest ratelimit/ratelimittest leader leader/redis leader/mongo lock/redis ratelimit ratelimit/redis; do
  test "$(rg -c '^## ' "$base/README.md")" = "$(rg -c '^## ' "$base/README.ko.md")"
done
git diff --check
git add leader lock ratelimit CHANGELOG.md \
  docs/superpowers/reviews/2026-07-12-issue-527-provider-conformance-caller-audit.md
git commit -m "docs: describe provider conformance contracts"
```

예상: example compile/pass; automated heading counts agree 및 the committed manual mapping proves 영문/한국어 pairs carry the same recovery contracts. No diagram is required because this adds behavioral contracts, 아님 a new architecture topology. No AGENTS/catalog/module/BOM/CI registration change is required because 없음 module/dependency is added.

### 작업 12: Integrated 검증, 위험 증거, And 리뷰 Readiness

**복잡도:** 높음
**Depends on:** 작업 11
**패턴:** `verification-before-completion`, `requesting-code-review`, `bluetape-workf낮음`

**파일:**
- Modify: `docs/superpowers/reviews/2026-07-12-issue-527-provider-conformance-risk.md`
- 생성: `docs/superpowers/reviews/2026-07-12-issue-527-provider-conformance-review.md`

- [ ] **단계 1: Finalize the pre-기존 단계 3-P risk evidence**

업데이트 the 작업 0A table 함께 actual signals 및 dispositions: late acquisition/commit-indeterminate response (owner probe 또는 typed wrapper); resign/renew overlap (operation delta); retry storm (commands/sec); false gate PASS (broken adapter); import cycle (neutral result); key/owner migration (byte table); Testcontainers leak (process/container check); hot-path allocation (전에/후 benchmark); secret leakage (forbidden marker). 다음을 하지 않는다: rewrite its creation history as post-implementation prediction.

- [ ] **단계 2: 실행 targeted serial 및 race gates**

```bash
go test -p 1 -count=1 ./leader/... ./lock/... ./ratelimit/...
go test -p 1 -race -count=1 ./leader/leadertest ./leader/redis ./leader/mongo \
  ./lock/locktest ./lock/redis ./ratelimit/ratelimittest ./ratelimit ./ratelimit/redis
```

예상: PASS; 없음 공유 Testcontainers subtest uses `t.Parallel`.

- [ ] **단계 3: 실행 repository gates**

```bash
make fmt-check
make tidy-check
make vet
make lint
TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make ci
git diff --check
git status --short
```

예상: PASS. If Docker is unavailable, record the exact infrastructure 오류 및 do 아님 claim provider verification; retry on a Docker-capable host rather than weakening 테스트.

- [ ] **단계 4: 실행 단계 6-R review 및 commit evidence**

실행 성능, 안정성, 보안, 운영자/Ops, 개발자/API, 사용자/호출자 lanes plus main integration against the exact diff. Require P0=0/P1=0. The review records 테스트 commands, race evidence, benchmark signal, retry budget, redaction markers, lifecycle cleanup, docs parity, acceptance-criteria traceability, 및 any P2 disposition.

```bash
git add docs/superpowers/reviews/2026-07-12-issue-527-provider-conformance-*.md
git commit -m "docs: record provider conformance verification"
```

예상: clean worktree 및 review verdict PASS.

## Acceptance-Criteria Traceability

| Spec AC | Plan tasks | 증거 |
|---|---|---|
| 1-2 공개 helpers, fixtures, 없음 skips | 2, 3, 6, 8 | helper 테스트 및 example |
| 3 Redis/Mongo same leader runner | 4, 5 | serial provider 테스트 |
| 4 blocking/cancel-safe Redis campaign | 4 | state-machine, owner probe, command budget |
| 5 Redis lock runner | 6, 7 | gate 및 exact contention 테스트 |
| 6 local/Redis rate runner | 8-10 | neutral adapter 및 exact burst 테스트 |
| 7 cancellation/expiry/stale/concurrency | 3-10 | named runner cases 및 race 테스트 |
| 8 compatibility migrations 만 | 1, 4, 5, 7, 9, 10 | byte/schema/sentinel/오류 테스트 |
| 9 GoDoc/example/README parity | 11 | example 테스트 및 locale review |
| 10 target/race/CI commands | 12 | captured fresh command output |
| 11 four 7-Tier gates | workf낮음 Steps 2-R, 3-R, 6-R, 7-R | P0=0/P1=0 tables |
| 12 issue/PR metadata 및 merge approval | post-implementation workf낮음 | PR #527 metadata 및 explicit 사용자 approval |
| 13 fault injection 및 eventual takeover | 3-5 | renew/resign counts, retry, redaction, TTL/takeover |
| 14 release audit/canary/rollback | 11-12 | README/CHANGELOG 및 risk evidence |
| 15 lost-response classification/없음 replay | 2-10 | fail injection, owner probe, typed wrappers 및 one-debit 테스트 |

## 롤백 And Rerun Boundaries

- Helper-만 commits are additive 및 can be reverted independently 전에 providers consume them.
- Redis leader rollback must first cancel outstanding campaigns, bounded-resign owners, verify renew traffic is zero, 및 record remaining TTL/takeover 전에 restoring immediate-contention binaries.
- Mongo rollback preserves BSON schema; wait for owner-aware resign/TTL 및 verify takeover.
- 고정/rate 테스트-hook commits change 없음 공개 storage format; revert them if nil fast-path allocation 또는 behavior changes.
- Structural validation 및 sentinel/오류 migrations require 호출자 audit; never roll back 만 documentation while leaving behavior changed.
- Any redaction, stale-owner deletion, double-leader, over-admission, goroutine/container leak, 또는 race finding stops implementation 및 reruns the owning task from its first RED 테스트.
