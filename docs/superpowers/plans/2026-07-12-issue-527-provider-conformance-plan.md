# Provider Conformance Contracts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add reusable, mandatory leader/lock/rate-limit conformance runners and make every existing provider pass the same observable contract.

**Architecture:** Three public test-helper packages expose factory/harness runners and in-memory reference fixtures; provider-specific test adapters supply only construction, deterministic fault gates, and backend probes. Production leader providers share one blocking campaign/state/error contract, while lock and rate-limit providers expose no new production abstraction and use private default-nil mutation hooks only where a real in-flight boundary cannot otherwise be controlled.

**Tech Stack:** Go 1.26, standard `testing`/`context`/`errors`/`sync`, go-redis v9, MongoDB Go driver v2, Testcontainers Redis/MongoDB, existing bluetape-go concurrency test helpers.

---

## File Map

| Area | Files | Responsibility |
|---|---|---|
| Leader core | `leader/errors.go`, `leader/errors_test.go`, `leader/options.go`, `leader/options_test.go` | State sentinels, invalid-context sentinel, sanitized operation errors, structural identity validation. |
| Leader helper | `leader/leadertest/{doc.go,harness.go,memory.go,runner.go,harness_test.go,runner_test.go,example_test.go,README.md,README.ko.md}` | Public mandatory harness, reference fixture, black-box runner, self-tests and docs. |
| Redis leader | `leader/redis/elector.go`, `leader/redis/elector_test.go`, `leader/redis/conformance_test.go`, `leader/redis/conformance_internal_test.go` | Blocking campaign, bounded retry, cleanup-pending resign state, real Redis adapter/control. |
| Mongo leader | `leader/mongo/elector.go`, `leader/mongo/elector_test.go`, `leader/mongo/conformance_test.go` | Common state/error semantics and Mongo adapter/control. |
| Lock helper | `lock/locktest/{doc.go,harness.go,memory.go,runner.go,harness_test.go,runner_test.go,example_test.go,README.md,README.ko.md}` | Function adapter contract, before/after gates, reference fixture and exact lock runner. |
| Redis lock | `lock/redis/mutex.go`, `lock/redis/mutex_test.go`, `lock/redis/conformance_test.go` | Cancellation linearization and mandatory Redis lock adapter. |
| Rate helper | `ratelimit/ratelimittest/{doc.go,harness.go,memory.go,runner.go,harness_test.go,runner_test.go,example_test.go,README.md,README.ko.md}` | Parent-independent neutral result/function contract and exact token-bucket runner. |
| Local rate limit | `ratelimit/token_bucket.go`, `ratelimit/token_bucket_test.go`, `ratelimit/conformance_test.go` | Private mutation hook, cancellation arbitration and local adapter. |
| Redis rate limit | `ratelimit/redis/limiter.go`, `ratelimit/redis/limiter_test.go`, `ratelimit/redis/conformance_test.go` | Command-boundary cancellation arbitration and Redis adapter. |
| Public docs/release | `leader/{README.md,README.ko.md}`, provider README pairs, `ratelimit/{README.md,README.ko.md}`, `CHANGELOG.md` | Blocking/migration semantics, helper usage, caveat matrix, rollout/rollback. |

## Dependency Order And Write Ownership

Tasks 1-3 establish leader core/helper APIs. Tasks 4-5 consume them and may not start before Task 3 is green. Task 6 establishes `locktest`; Task 7 consumes it. Task 8 establishes `ratelimittest`; Tasks 9-10 consume it. Task 11 edits only documentation after APIs settle. Task 12 owns only verification/review artifacts. Do not run Tasks 4-5, 6-7, or 8-10 concurrently when they touch the same provider fixture/client.

Use `bttesting.Eventually`/`Consistently` for bounded eventual state and `testing/concurrency` stress helpers for exact concurrent totals. Raw goroutines/channels are allowed only for deterministic gate handshakes because no existing helper exposes a before/after-linearization barrier; every such goroutine has a buffered result channel, caller deadline, and `t.Cleanup` resume/cancel.

### Task 0: Freeze Approved Workflow Artifacts

**Complexity:** low  
**Depends on:** none  
**Patterns:** `bluetape-workflow`, `bluetape-full-feature`

**Files:**
- Verify: `docs/superpowers/specs/2026-07-12-issue-527-provider-conformance-design.md`
- Verify: `docs/superpowers/plans/2026-07-12-issue-527-provider-conformance-plan.md`

- [ ] **Step 1: Verify artifact-only branch state**

Run:

```bash
git diff --check
git status --short
git log --oneline origin/develop..HEAD
```

Expected: no whitespace errors; only approved spec/plan/review artifacts differ from `origin/develop`; no source file is modified.

- [ ] **Step 2: Commit the reviewed plan before source edits**

```bash
git add docs/superpowers/specs/2026-07-12-issue-527-provider-conformance-design.md \
  docs/superpowers/plans/2026-07-12-issue-527-provider-conformance-plan.md
git commit -m "docs: plan provider conformance implementation"
```

Expected: clean worktree. Roll back this task by reverting only the plan commit; never discard the approved spec.

### Task 1: Leader Core Validation And Error Contracts

**Complexity:** medium  
**Depends on:** Task 0  
**Patterns:** `bluetape-go-patterns`, `test-driven-development`

**Files:**
- Modify: `leader/errors.go`
- Create: `leader/errors_test.go`
- Modify: `leader/options.go`
- Create: `leader/options_test.go`

- [ ] **Step 1: Write failing table tests for structural identities and safe errors**

Add tests with these exact cases:

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
```

Also assert leading/trailing whitespace is rejected rather than trimmed, internal valid Unicode bytes and canonically equivalent forms remain byte-distinct, final key length `512` passes, and nil/blank/control/>32-byte `NewOperationError` metadata is rejected.

- [ ] **Step 2: Observe RED**

Run:

```bash
go test -count=1 ./leader -run 'OptionsNormalize|OperationError'
```

Expected: FAIL because `ErrInvalidContext`, state sentinels, `OperationError`, and structural validation do not exist.

- [ ] **Step 3: Add the minimal core contracts**

Implement exported sentinels with GoDoc:

```go
var (
    ErrAlreadyLeader      = errors.New("leader: already leader")
    ErrNotLeader          = errors.New("leader: not leader") // legacy contention sentinel
    ErrCampaignInProgress = errors.New("leader: campaign in progress")
    ErrCleanupPending     = errors.New("leader: cleanup pending")
    ErrInvalidContext     = errors.New("leader: invalid context")
)
```

Add private fields to `OperationError`, validate constructor labels, never call `cause.Error()`, and make every method nil-safe. In `Options.Normalize`, validate byte lengths and structural segments before defaults; use `unicode.IsControl`, reject `: { }` in group/member, reject empty/control/braced prefix segments, and ensure `len(KeyPrefix)+1+len(Group) <= 512` without trimming or Unicode normalization.

- [ ] **Step 4: Verify GREEN and commit**

```bash
gofmt -w leader/errors.go leader/errors_test.go leader/options.go leader/options_test.go
go test -count=1 ./leader
git add leader/errors.go leader/errors_test.go leader/options.go leader/options_test.go
git commit -m "feat: define leader provider contracts"
```

Expected: PASS. Revert this commit if valid existing key bytes change in the preservation table.

### Task 2: Public `leader/leadertest` Harness And Reference Fixture

**Complexity:** high  
**Depends on:** Task 1  
**Patterns:** `bluetape-go-patterns`, `test-driven-development`

**Files:**
- Create: `leader/leadertest/doc.go`
- Create: `leader/leadertest/harness.go`
- Create: `leader/leadertest/memory.go`
- Create: `leader/leadertest/harness_test.go`

- [ ] **Step 1: Write RED API/validation tests**

Compile-check this exact public shape:

```go
type Operation string
const (
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

Test nil harness fields, invalid direct control options/owner/operation/cause, nil/pre-canceled contexts, monotonic counts, exactly-one `FailNext`, stale replacement, and no mutation on rejected controls.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./leader/leadertest
```

Expected: build FAIL because the package is absent.

- [ ] **Step 3: Implement the harness and mutex-protected memory lease store**

Use one normalized identity key and a record containing `owner`, `leaseUntil`, injected renew/resign causes, and cumulative operation counters. The memory elector must own its renewal goroutine/timer, cancel it on loss/resign, keep cleanup-pending token state after delete failure, and compare owner before delete. Control errors must be sanitized and must not print raw identities.

Core validation must follow this shape:

```go
func validateContext(ctx context.Context) error {
    if ctx == nil { return leader.ErrInvalidContext }
    if err := ctx.Err(); err != nil { return err }
    return nil
}
```

- [ ] **Step 4: Verify fixture race safety and commit**

```bash
gofmt -w leader/leadertest
go test -count=1 ./leader/leadertest -run 'Harness|Memory|Control'
go test -race -count=1 ./leader/leadertest
git add leader/leadertest
git commit -m "test: add leader conformance harness"
```

Expected: PASS with no race. If a goroutine remains after cleanup, stop and fix ownership before Task 3.

### Task 3: Leader Black-Box Runner And Self-Tests

**Complexity:** high  
**Depends on:** Task 2  
**Patterns:** `bluetape-go-patterns`, `test-driven-development`

**Files:**
- Create: `leader/leadertest/runner.go`
- Create: `leader/leadertest/runner_test.go`
- Create: `leader/leadertest/example_test.go`

- [ ] **Step 1: Add runner self-tests that intentionally break each evaluator**

Use `MemoryHarness()` as the passing reference, then exercise package-private evaluator functions with controlled records that represent these named failures: two simultaneous winners, owner-insensitive stale resign, ignored cancellation, renewal continuing after injected loss, skipped cleanup retry, raw marker in diagnostic, and operation count that always returns zero. Each evaluator returns an `error`; `Run` is only the thin `t.Run`/`t.Fatal` shell. Do not attempt to implement `testing.TB` (its private method prevents external fakes), and do not let broken factories replace the real provider in provider tests.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./leader/leadertest -run 'Run|Broken'
```

Expected: FAIL because `Run` and named contract cases are absent.

- [ ] **Step 3: Implement `Run` with mandatory named cases**

Create subtests named exactly:

```go
var cases = []string{
    "acquire-observe", "owned-duplicate", "campaign-in-progress",
    "contention-cancel", "renewal", "renew-failure", "owner-loss",
    "expiry-takeover", "resign-idempotent", "resign-retry",
    "stale-resign", "exact-contention", "nil-context", "redaction",
}
```

Normalize options once per case; use public elector methods for normal observations and Control only for fault/probe/count. All waits use caller-owned deadlines and eventual assertions; exact contention asserts `successes==1`, `maxActive==1`, bounded losers, and one takeover after release. Register bounded resign cleanup immediately after acquire.

- [ ] **Step 4: Verify and commit**

```bash
gofmt -w leader/leadertest
go test -count=1 ./leader/leadertest
go test -race -count=1 ./leader/leadertest
git add leader/leadertest
git commit -m "test: define leader conformance runner"
```

Expected: reference harness PASS; every intentionally broken harness fails its intended named case.

### Task 4: Redis Single-Leader State Machine And Conformance Adapter

**Complexity:** high  
**Depends on:** Task 3  
**Patterns:** `bluetape-go-patterns`, `test-driven-development`, `systematic-debugging` on any timing failure

**Files:**
- Modify: `leader/redis/elector.go`
- Modify: `leader/redis/elector_test.go`
- Modify: `leader/redis/resign_internal_test.go`
- Create: `leader/redis/conformance_test.go`
- Create: `leader/redis/conformance_internal_test.go`

- [ ] **Step 1: Replace immediate-contention expectations with RED state-machine tests**

Add tests asserting: contender blocks until deadline and returns `DeadlineExceeded`; owner resign permits takeover; concurrent same-instance call returns `ErrCampaignInProgress`; cleanup-pending returns `ErrCleanupPending`; nil contexts return `ErrInvalidContext` before Redis commands; injected delete failure is retryable; one already-linearized late renew is allowed after resign deadline and no later renew occurs; provider errors satisfy both `*leader.OperationError` and existing `*btredis.OpError` without marker leakage.

- [ ] **Step 2: Observe RED**

```bash
go test -p 1 -count=1 ./leader/redis -run 'Campaign|Resign|OperationError|Conformance'
```

Expected: FAIL on immediate `ErrNotLeader`, nil context normalization, lost cleanup state, and missing common wrapper/runner.

- [ ] **Step 3: Implement the blocking Redis state machine**

Replace loose booleans with guarded `owned`, `campaigning`, `cleanupPending`, `cancel`, `done`, token state. Retry SetNX with 25ms base exponential delay, 250ms cap, owner-token deterministic ±20% jitter, cancel-aware timers, and at most 12 attempts per one-second contention window. Check context before and after each acquire attempt; if acquire linearized first, return success and start renewal. Resign transitions to cleanup-pending before stopping renewal and retains token/done state until compare-delete success/mismatch. Wrap Redis failures as:

```go
return leader.NewOperationError("redis", operation,
    btredis.NewOpError(btredis.OpLabels{Family: "leader redis", Operation: operation}, e.key, cause))
```

Use package-owned labels only.

- [ ] **Step 4: Build the real Redis harness adapter**

Use a dedicated ready/pinged Testcontainers client, unique prefix per subtest, command hook/interceptor for exact renew/resign failure/count, and a separate control client for owner replacement/probe. Cleanup order is elector → client → container; every stage has a bounded context and reports errors. No `t.Parallel`.

- [ ] **Step 5: Verify targeted behavior, race, command budget, and commit**

```bash
gofmt -w leader/redis
go test -p 1 -count=1 ./leader/redis
go test -p 1 -race -count=1 ./leader/redis
git add leader/redis
git commit -m "feat: conform redis leader election"
```

Expected: runner PASS, retry command budget ≤12/second, no late owner after cancellation, no race/goroutine leak. Roll back the commit if existing valid Redis key/token bytes change.

### Task 5: Mongo Single-Leader Conformance And Sanitized Errors

**Complexity:** high  
**Depends on:** Task 3  
**Patterns:** `bluetape-go-patterns`, `test-driven-development`

**Files:**
- Modify: `leader/mongo/elector.go`
- Modify: `leader/mongo/elector_test.go`
- Create: `leader/mongo/conformance_test.go`

- [ ] **Step 1: Add RED tests for common state/error behavior**

Assert nil contexts fail before collection calls, campaigning and cleanup-pending use distinct sentinels, failed resign preserves retry state, renewal failure/lost owner stops traffic, and injected driver marker is absent from rendering while `errors.As` still reaches the Mongo driver cause through `*leader.OperationError`.

- [ ] **Step 2: Observe RED**

```bash
go test -p 1 -count=1 ./leader/mongo -run 'Elector|OperationError|Conformance'
```

Expected: FAIL because Mongo normalizes nil context, conflates local states, clears ownership before failed cleanup, and exposes raw driver text.

- [ ] **Step 3: Implement the same guarded state transitions**

Adopt the Redis state names and sentinel decisions without porting Redis storage/retry code. Keep Mongo document schema and clocks unchanged. Add a default-nil private test control hook at actual renew/resign boundaries for deterministic failure/count; it is unexported and unreachable by callers. Wrap provider failures with `leader.NewOperationError("mongo", operation, cause)` and preserve driver `errors.Is`/`errors.As`.

- [ ] **Step 4: Run the common runner against a real Mongo fixture**

Create a unique collection, wait for a bounded ping/command probe, assign the private test control to each elector made by the factory, and implement owner replacement/probe against the collection. Disconnect client before container termination; attempt/report both cleanups even when one fails.

- [ ] **Step 5: Verify and commit**

```bash
gofmt -w leader/mongo
go test -p 1 -count=1 ./leader/mongo
go test -p 1 -race -count=1 ./leader/mongo
git add leader/mongo
git commit -m "feat: conform mongo leader election"
```

Expected: the same `leadertest.Run` cases pass for Redis and Mongo; BSON schema and valid identity keys remain unchanged.

### Task 6: Public `lock/locktest` Harness, Gates, And Reference Runner

**Complexity:** high  
**Depends on:** Task 0  
**Patterns:** `bluetape-go-patterns`, `test-driven-development`

**Files:**
- Create: `lock/locktest/doc.go`
- Create: `lock/locktest/harness.go`
- Create: `lock/locktest/memory.go`
- Create: `lock/locktest/runner.go`
- Create: `lock/locktest/harness_test.go`
- Create: `lock/locktest/runner_test.go`
- Create: `lock/locktest/example_test.go`

- [ ] **Step 1: Write RED compile/validation/gate tests**

Define the approved `Config`, `ReleaseFunc`, `AcquireFunc`, `Factory`, `OperationAcquire/Release`, `PhaseBefore/AfterLinearize`, `Gate`, `Control`, `Harness`, `Run`, and `MemoryHarness`. Test invalid config/operation/phase, nil/pre-canceled contexts, nil returned gate/functions, idempotent non-blocking `Resume`, `AwaitStarted` cancellation, count fallback 0, and cleanup auto-resume.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./lock/locktest
```

Expected: build FAIL because the package is absent.

- [ ] **Step 3: Implement reference fixture and mandatory runner cases**

Use a mutex-protected `{owner, expiresAt}` map and one-shot before/after gates. Named cases are:

```go
var cases = []string{
    "acquire-release", "contention", "repeated-release", "expiry-takeover",
    "pre-canceled-acquire", "pre-canceled-release", "cancel-before-linearize",
    "cancel-after-linearize", "stale-release", "exact-contention",
}
```

Before-linearize cancellation returns context error with nil/false and zero owner/count delta; after-linearize cancellation returns the successful release/result. Exact stress asserts one success, `workers-1` provider sentinels, `maxActive==1`, then one takeover.

- [ ] **Step 4: Verify self-tests and commit**

```bash
gofmt -w lock/locktest
go test -count=1 ./lock/locktest
go test -race -count=1 ./lock/locktest
git add lock/locktest
git commit -m "test: add lock conformance runner"
```

Expected: memory harness PASS; owner-ignorant release and wrapper-only fake adapters fail deterministically.

### Task 7: Redis Lock Linearization And Common Runner

**Complexity:** medium  
**Depends on:** Task 6  
**Patterns:** `bluetape-go-patterns`, `test-driven-development`

**Files:**
- Modify: `lock/redis/mutex.go`
- Modify: `lock/redis/mutex_test.go`
- Create: `lock/redis/conformance_test.go`

- [ ] **Step 1: Add RED in-flight cancellation tests and harness invocation**

Gate SetNX/Eval before and after server execution. Cancel before command dispatch and assert no key; cancel after successful server reply but before method return and assert a lease/`true,nil`; prove stale lease never deletes a replacement owner.

- [ ] **Step 2: Observe RED**

```bash
go test -p 1 -count=1 ./lock/redis -run 'Linearization|Conformance'
```

Expected: FAIL because post-command cancellation linearization is unproved and no common harness exists.

- [ ] **Step 3: Implement context arbitration and Redis adapter**

Preserve the existing lock nil-context compatibility. Use a private helper that returns a context error only when the command did not linearize; once SetNX/Eval produced a successful result, return it even if cancellation races afterward. The test adapter uses go-redis hooks on a dedicated client for gate/count and a control client for GET/SET; it maps `ErrNotAcquired` unchanged.

- [ ] **Step 4: Verify and commit**

```bash
gofmt -w lock/redis
go test -p 1 -count=1 ./lock/redis
go test -p 1 -race -count=1 ./lock/redis
git add lock/redis
git commit -m "test: conform redis lock provider"
```

Expected: `locktest.Run` PASS, exact contention totals, no key after before-linearize cancellation, owner-aware cleanup preserved.

### Task 8: Parent-Independent `ratelimit/ratelimittest` Runner

**Complexity:** high  
**Depends on:** Task 0  
**Patterns:** `bluetape-go-patterns`, `test-driven-development`

**Files:**
- Create: `ratelimit/ratelimittest/doc.go`
- Create: `ratelimit/ratelimittest/harness.go`
- Create: `ratelimit/ratelimittest/memory.go`
- Create: `ratelimit/ratelimittest/runner.go`
- Create: `ratelimit/ratelimittest/harness_test.go`
- Create: `ratelimit/ratelimittest/runner_test.go`
- Create: `ratelimit/ratelimittest/example_test.go`

- [ ] **Step 1: Write RED API and import-cycle tests**

The helper must not import `github.com/bluetape4k/bluetape-go/ratelimit`. Define its own field-identical `Result`, `AllowFunc`, `Factory`, before/after `Phase`, `Gate`, `Control`, `Harness`, `Run`, and `MemoryHarness`. Test positive finite rate, positive burst, IdleTTL ≥ full-refill duration when nonzero, invalid key/phase/count behavior, nil functions/gates, and context behavior.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./ratelimit/ratelimittest
```

Expected: build FAIL because the package is absent.

- [ ] **Step 3: Implement memory bucket and exact runner**

Use a controllable clock and mutex-protected buckets. Named cases:

```go
var cases = []string{
    "initial-burst", "over-burst-validation", "rejection-result", "refill",
    "key-isolation", "pre-canceled", "cancel-before-linearize",
    "cancel-after-linearize", "exact-concurrency",
}
```

Exact concurrency uses a frozen/no-refill window and asserts `allowed==Burst`, `rejected==requests-Burst`, and admitted token sum `Burst`. Before cancellation leaves a full burst; after cancellation returns success and the next request observes the exact prior debit.

- [ ] **Step 4: Verify no parent import and commit**

```bash
gofmt -w ratelimit/ratelimittest
! rg 'bluetape-go/ratelimit"' ratelimit/ratelimittest
go test -count=1 ./ratelimit/ratelimittest
go test -race -count=1 ./ratelimit/ratelimittest
git add ratelimit/ratelimittest
git commit -m "test: add rate limit conformance runner"
```

Expected: PASS and no import-cycle risk.

### Task 9: Local Token Bucket Real-Boundary Conformance

**Complexity:** medium  
**Depends on:** Task 8  
**Patterns:** `bluetape-go-patterns`, `test-driven-development`

**Files:**
- Modify: `ratelimit/token_bucket.go`
- Modify: `ratelimit/token_bucket_test.go`
- Create: `ratelimit/conformance_test.go`

- [ ] **Step 1: Add RED same-package gate and common-runner tests**

In `package ratelimit`, import `ratelimit/ratelimittest`, translate every `Result` field, and wire a test controller to the real mutation boundary. Assert cancellation before mutation consumes nothing and cancellation after mutation returns successful result. Add a compile test proving `ratelimittest` does not create an import cycle.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./ratelimit -run 'Linearization|Conformance'
```

Expected: FAIL because `TokenBucket` has no real-boundary hook or cancellation arbitration at mutation.

- [ ] **Step 3: Add a private default-nil hook and arbitrate cancellation under the bucket lock**

Use an unexported field only same-package tests can set:

```go
type tokenBucketHook interface {
    beforeLinearize(context.Context, string) error
    afterLinearize(context.Context, string)
}
```

Call it immediately before/after `l.buckets[key] = state`; the nil path performs only one branch and changes no public layout contract. If context wins before assignment, return context error without storing; after assignment, return computed result even if context is canceled.

- [ ] **Step 4: Verify correctness, race, and benchmark non-regression signal**

```bash
gofmt -w ratelimit/token_bucket.go ratelimit/token_bucket_test.go ratelimit/conformance_test.go
go test -count=1 ./ratelimit
go test -race -count=1 ./ratelimit
go test -run '^$' -bench 'TokenBucket' -benchmem ./ratelimit
git add ratelimit/token_bucket.go ratelimit/token_bucket_test.go ratelimit/conformance_test.go
git commit -m "test: conform local token bucket"
```

Expected: runner/race PASS; benchmark is recorded as a signal, not a claimed improvement. Any allocation added to the nil-hook hot path blocks progression.

### Task 10: Redis Rate Limiter Real-Command Conformance

**Complexity:** medium  
**Depends on:** Task 8  
**Patterns:** `bluetape-go-patterns`, `test-driven-development`

**Files:**
- Modify: `ratelimit/redis/limiter.go`
- Modify: `ratelimit/redis/limiter_test.go`
- Create: `ratelimit/redis/conformance_test.go`

- [ ] **Step 1: Add RED command-boundary cancellation and runner tests**

Use a go-redis hook to pause before Eval dispatch and after successful Eval response. Before cancellation must leave no bucket key and permit a full burst; after cancellation must return the successful neutral result and the next request must observe the debit. Preserve existing server-time rounding tests.

- [ ] **Step 2: Observe RED**

```bash
go test -p 1 -count=1 ./ratelimit/redis -run 'Linearization|Conformance'
```

Expected: FAIL because cancellation/result arbitration at the Redis script boundary is unspecified.

- [ ] **Step 3: Implement minimal context arbitration and adapter conversion**

Preserve the existing rate-limiter nil-context compatibility. Treat a parsed successful Lua result as linearized and return it even if cancellation arrives afterward; a context-error return must correspond to no script mutation. Adapter converts fields explicitly:

```go
return ratelimittest.Result{
    Allowed: r.Allowed, Requested: r.Requested, Remaining: r.Remaining,
    RetryAfter: r.RetryAfter, ResetAfter: r.ResetAfter,
}, err
```

Use unique namespace/key, cumulative Eval counts, and no wrapper-only gate.

- [ ] **Step 4: Verify and commit**

```bash
gofmt -w ratelimit/redis
go test -p 1 -count=1 ./ratelimit/redis
go test -p 1 -race -count=1 ./ratelimit/redis
git add ratelimit/redis
git commit -m "test: conform redis rate limiter"
```

Expected: common runner PASS, exact burst totals, preserved Redis key/TTL/rounding and typed error behavior.

### Task 11: Public GoDoc, Examples, Locale Parity, And Release Migration

**Complexity:** medium  
**Depends on:** Tasks 4, 5, 7, 9, 10  
**Patterns:** `bluetape-go-patterns`, `bluetape-writer`, `bluetape-maintenance`

**Files:**
- Create: helper README pairs listed in File Map
- Modify: `leader/README.md`, `leader/README.ko.md`
- Modify: `leader/redis/README.md`, `leader/redis/README.ko.md`
- Modify: `leader/mongo/README.md`, `leader/mongo/README.ko.md`
- Modify: `lock/redis/README.md`, `lock/redis/README.ko.md`
- Modify: `ratelimit/README.md`, `ratelimit/README.ko.md`
- Modify: `ratelimit/redis/README.md`, `ratelimit/redis/README.ko.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Add compile-checked helper examples**

Each `ExampleRun` constructs a caller-owned fixture, registers cleanup immediately, builds the full mandatory Harness including before/after gates and counts, calls `Run`, and uses unique identities/bounded contexts. Examples must show contention/cancellation outcomes and never print raw backend errors.

- [ ] **Step 2: Document exact migration and provider caveat matrix in English/Korean**

Cover blocking `Campaign`, legacy `ErrNotLeader`, new state sentinels, nil-context rejection, structural identity audit, common `OperationError`, caller timeout ownership, actual-mutation-boundary adapter rules, retry/clock precision non-guarantees, mixed-version behavior, telemetry labels, canary thresholds, resign/TTL rollback and Group/Strategic deferral. Keep every README pair section-for-section equivalent.

- [ ] **Step 3: Add 0.19.0 Unreleased changelog entry and verify docs**

```bash
go test -run 'Example' ./leader/... ./lock/... ./ratelimit/...
rg -n 'ErrCampaignInProgress|ErrCleanupPending|ErrInvalidContext|OperationError' \
  leader/**/README*.md CHANGELOG.md
git diff --check
git add leader lock ratelimit CHANGELOG.md
git commit -m "docs: describe provider conformance contracts"
```

Expected: examples compile/pass; English/Korean pairs contain the same contract headings. No diagram is required because this adds behavioral contracts, not a new architecture topology. No AGENTS/catalog/module/BOM/CI registration change is required because no module/dependency is added.

### Task 12: Integrated Verification, Risk Evidence, And Review Readiness

**Complexity:** high  
**Depends on:** Task 11  
**Patterns:** `verification-before-completion`, `requesting-code-review`, `bluetape-workflow`

**Files:**
- Create: `docs/superpowers/reviews/2026-07-12-issue-527-provider-conformance-risk.md`
- Create: `docs/superpowers/reviews/2026-07-12-issue-527-provider-conformance-review.md`

- [ ] **Step 1: Record Step 3-P risks before implementation claims**

Write a table containing at least these signals/mitigations/rerun points: late acquisition after cancellation (owner probe; re-run leader/lock linearization); resign/renew overlap (operation delta; re-run race); retry storm (commands/sec; revert cadence); false gate PASS (broken adapter self-test; fix boundary); import cycle (dependency build; keep neutral result); key migration (byte table; revert validation); Testcontainers leak (process/container check; bounded teardown); hot-path allocation (benchmark/race; remove hook allocation); secret leakage (forbidden marker; stop rollout).

- [ ] **Step 2: Run targeted serial and race gates**

```bash
go test -p 1 -count=1 ./leader/... ./lock/... ./ratelimit/...
go test -p 1 -race -count=1 ./leader/leadertest ./leader/redis ./leader/mongo \
  ./lock/locktest ./lock/redis ./ratelimit/ratelimittest ./ratelimit ./ratelimit/redis
```

Expected: PASS; no shared Testcontainers subtest uses `t.Parallel`.

- [ ] **Step 3: Run repository gates**

```bash
make fmt-check
make tidy-check
make vet
make lint
TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make ci
git diff --check
git status --short
```

Expected: PASS. If Docker is unavailable, record the exact infrastructure error and do not claim provider verification; retry on a Docker-capable host rather than weakening tests.

- [ ] **Step 4: Run Step 6-R review and commit evidence**

Run performance, stability, security, operator/Ops, developer/API, user/caller lanes plus main integration against the exact diff. Require P0=0/P1=0. The review records test commands, race evidence, benchmark signal, retry budget, redaction markers, lifecycle cleanup, docs parity, acceptance-criteria traceability, and any P2 disposition.

```bash
git add docs/superpowers/reviews/2026-07-12-issue-527-provider-conformance-*.md
git commit -m "docs: record provider conformance verification"
```

Expected: clean worktree and review verdict PASS.

## Acceptance-Criteria Traceability

| Spec AC | Plan tasks | Evidence |
|---|---|---|
| 1-2 public helpers, fixtures, no skips | 2, 3, 6, 8 | helper tests and examples |
| 3 Redis/Mongo same leader runner | 4, 5 | serial provider tests |
| 4 blocking/cancel-safe Redis campaign | 4 | state-machine, owner probe, command budget |
| 5 Redis lock runner | 6, 7 | gate and exact contention tests |
| 6 local/Redis rate runner | 8-10 | neutral adapter and exact burst tests |
| 7 cancellation/expiry/stale/concurrency | 3-10 | named runner cases and race tests |
| 8 compatibility migrations only | 1, 4, 5, 7, 9, 10 | byte/schema/sentinel/error tests |
| 9 GoDoc/examples/README parity | 11 | example tests and locale review |
| 10 target/race/CI commands | 12 | captured fresh command output |
| 11 four 7-Tier gates | workflow Steps 2-R, 3-R, 6-R, 7-R | P0=0/P1=0 tables |
| 12 issue/PR metadata and merge approval | post-implementation workflow | PR #527 metadata and explicit user approval |
| 13 fault injection and eventual takeover | 3-5 | renew/resign counts, retry, redaction, TTL/takeover |
| 14 release audit/canary/rollback | 11-12 | README/CHANGELOG and risk evidence |

## Rollback And Rerun Boundaries

- Helper-only commits are additive and can be reverted independently before providers consume them.
- Redis leader rollback must first cancel outstanding campaigns, bounded-resign owners, verify renew traffic is zero, and record remaining TTL/takeover before restoring immediate-contention binaries.
- Mongo rollback preserves BSON schema; wait for owner-aware resign/TTL and verify takeover.
- Lock/rate test-hook commits change no public storage format; revert them if nil fast-path allocation or behavior changes.
- Structural validation and sentinel/error migrations require caller audit; never roll back only documentation while leaving behavior changed.
- Any redaction, stale-owner deletion, double-leader, over-admission, goroutine/container leak, or race finding stops implementation and reruns the owning task from its first RED test.
