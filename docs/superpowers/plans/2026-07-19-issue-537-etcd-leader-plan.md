# etcd Leader Elector Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a caller-client-owned etcd implementation of `leader.Elector` that passes all 15 mandatory provider-conformance cases without weakening their semantics.

**Architecture:** `leader/etcd` creates one explicitly granted etcd lease, official `concurrency.Session`, and official `concurrency.Election` per Campaign generation. A mutex-protected generation record owns cancellation, publication, exact-key watch, single-flight Proclaim renewal, Session join, monitor join, and proof-based cleanup; `leader/leadertest.RunWithConfig` supplies etcd-compatible timing plus fail-stop timeout containment while preserving `Harness` and `Run` source compatibility.

**Tech Stack:** Go 1.26, etcd client/v3 v3.6.13, Testcontainers etcd v0.42.0, `leader/leadertest`, `internal/testcleanup`, standard Go race/lint/CI gates.

---

## File Map

| Area | Files | Responsibility |
|---|---|---|
| Conformance timing | `leader/leadertest/timing.go`, `leader/leadertest/runner.go`, `leader/leadertest/{timing_test.go,runner_test.go}` | Source-compatible timing normalization, cancel/abort/join containment, unchanged 15-case table. |
| Public etcd API | `leader/etcd/doc.go`, `leader/etcd/elector.go`, `leader/etcd/options.go` | Constructor, key encoding, rounded TTL, caller-owned client contract, `EffectiveTTL`. |
| Generation lifecycle | `leader/etcd/generation.go`, `leader/etcd/campaign.go`, `leader/etcd/monitor.go`, `leader/etcd/cleanup.go`, `leader/etcd/observe.go` | Campaign publication, Session/Election ownership, exact watch, Proclaim renewal, resign/reconciliation, redacted observation. |
| Unit tests | `leader/etcd/{options_test.go,generation_test.go,campaign_test.go,monitor_test.go,cleanup_test.go,errors_test.go}` | Deterministic state, cancellation, nil-session, join, reconciliation, redaction, and concurrency contracts. |
| Real etcd tests | `leader/etcd/{etcd_test_fixture_test.go,integration_test.go,conformance_test.go,security_test.go,resource_test.go}` | Official server fixture, real election semantics, fault control, mandatory conformance, trust-boundary and resource evidence. |
| Examples/docs | `leader/etcd/{example_test.go,README.md,README.ko.md}`, provider indexes, `CHANGELOG.md`, release runbook | Compile-checked usage, shutdown, migration, security, no-fencing, discoverability, release notes. |
| Dependency/review evidence | `go.mod`, `go.sum`, `docs/superpowers/reviews/*issue-537*`, `docs/lessons/*issue-537*` | Pinned dependency graph, risk prediction, Step 6-R/7-R reviews, Type A reusable lesson. |

## Dependency Order and Execution Constraints

Task 0 freezes reviewed artifacts and records baseline evidence. Task 1 must land before provider
tests because etcd cannot represent the current 300ms harness lease. Tasks 2-5 build the provider
in TDD order: value/key contract, Campaign, monitoring, then cleanup/observation. Task 6 adds the
real-server adapter and mandatory conformance. Task 7 hardens faults, trust boundaries, and
resource containment. Task 8 updates public documentation only after the API and tests settle.
Task 9 is the final repository gate.

Do not run Docker-backed packages in parallel. Do not change `leader.Elector`, `leader.Options`,
or the 15 conformance cases. Do not add capability flags, skips, relaxed assertions, a generic etcd
wrapper, a public Testcontainers wrapper, an elector-owned client, or a detached Campaign wrapper
goroutine. Any required deviation returns to Step 2 design review.

### Task 0: Freeze Artifacts, Baseline CI, and Predict Risks

**Files:**
- Verify: `docs/superpowers/specs/2026-07-19-issue-537-etcd-leader-design.md`
- Verify: `docs/superpowers/reviews/2026-07-19-issue-537-etcd-leader-step-2r-spec-review.md`
- Create: `docs/superpowers/reviews/2026-07-19-issue-537-etcd-leader-step-3p-risk-prediction.md`

- [ ] **Step 1: Verify the approved artifact-only branch**

Run:

```bash
git status --short
git log --oneline origin/develop..HEAD
git diff --check origin/develop...HEAD
```

Expected: only issue #537 design, review, and plan artifacts are ahead of `develop`; no
`leader/etcd` source or dependency change exists.

- [ ] **Step 2: Record dependency and CI baselines**

Run serially and paste exact duration, module, Go, platform, and Docker availability into the risk
artifact:

```bash
go version
go env GOOS GOARCH
go list -m github.com/testcontainers/testcontainers-go
/usr/bin/time -p make ci
```

Expected: the pre-change full-CI baseline passes and is below the current 25-minute job timeout.
If Docker is unavailable, record that gap without substituting a mock-server claim.

- [ ] **Step 3: Write the pre-implementation risk ledger**

Create a table with `Risk`, `Trigger`, `Signal`, `Prevention`, `Recovery`, and `Owner`. Include:
integer-TTL drift, Campaign cleanup blocked on `client.Ctx`, cancellation/publication race,
nil Session after Grant/NewSession failure, monitor created before publication failure,
watch-created handshake timeout, compaction, mismatched PUT, overlapping key ranges,
Proclaim overlap, stale monitor ABA, nil official Resign without delete, stale revision cleanup,
lease-level cross-principal revoke/keepalive, shared-client hard stop, Testcontainers leak,
dependency graph churn, 32-contender leak, and rapid reacquisition without caller fencing.

- [ ] **Step 4: Commit risk evidence before source work**

```bash
git add docs/superpowers/reviews/2026-07-19-issue-537-etcd-leader-step-3p-risk-prediction.md
git commit -m "Predict etcd election delivery risks" \
  --trailer "Constraint: Official Campaign cleanup can outlive the caller deadline." \
  --trailer "Confidence: high" \
  --trailer "Scope-risk: narrow" \
  --trailer "Tested: pre-change make ci baseline and artifact diff checks"
```

Expected: the risk commit predates every source or module commit.

### Task 1: Add Source-Compatible Conformance Timing and Containment

**Files:**
- Create: `leader/leadertest/timing.go`
- Create: `leader/leadertest/timing_test.go`
- Modify: `leader/leadertest/runner.go`
- Modify: `leader/leadertest/runner_test.go`
- Test: `leader/leadertest/example_test.go`

- [ ] **Step 1: Write RED normalization and compatibility tests**

Put the source-compatibility assertion in `package leadertest_test` so it proves the external API:

```go
var _ = leadertest.Harness{nil, nil}
```

Add internal runtime assertions for normalization:

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

Add a runner test whose injected private evaluator waits on its received case context. Cancel the
case root and assert the evaluator observes cancellation and joins before `Abort` can run. Add a
subprocess test whose elector remains blocked after root cancellation; with a successful `Abort`,
assert event order `cancel, abort, joined`. With nil/failing Abort and an unjoined call, assert the
subprocess reaches the outer `go test -timeout` fail-stop instead of returning to a later case.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./leader/leadertest -run 'TestNormalizeTiming|TestRunWithConfig|TestRunContainment'
```

Expected: build FAIL because `Timing`, `Config`, `AbortFunc`, `RunWithConfig`, and normalization do
not exist.

- [ ] **Step 3: Implement the timing API and containment formulas**

Add Go doc comments for each exported type, field, and function. Keep the blank fields and this
exact public signature:

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

Resolve zero fields independently to `300ms`, `50ms`, `5s`, `2s`, and `250ms`; reject negatives,
`RenewInterval >= Lease`, and profiles that fail either inequality. Avoid duration addition so
caller-controlled near-`MaxInt64` values cannot wrap:

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

Make `Run` exactly `RunWithConfig(t, harness, Config{})`. Move the existing 15-case table unchanged
into `RunWithConfig`, but change the private table function shape to:

```go
type evaluator func(context.Context, *testing.T, Harness, leader.Options, Timing) error
```

Each case creates one cancelable root and passes it plus normalized Timing to its evaluator.
Campaign, Leader, Control, wait, and contention-worker contexts derive from that root; `waitFor`
accepts the root and exits on `ctx.Done`. Bounded Resign cleanup uses a fresh context so root
cancellation cannot skip release. Preserve all 15 names, order, and assertions. On timeout: cancel
the root, wait `joinGrace`, call Abort with a fresh `abortBudget` context whether already joined or
still running, then join. If the call
remains unjoined, block so the outer test timeout fails the process rather than leaking work into
the next subtest.

- [ ] **Step 4: Verify existing and custom profiles**

```bash
go test -count=1 ./leader/leadertest
go test -race -count=1 ./leader/leadertest
```

Expected: existing `Run(t, MemoryHarness())`, redaction subprocesses, broken-provider detection,
custom timing, abort ordering, and fail-stop subprocess tests PASS.

- [ ] **Step 5: Commit the harness amendment**

```bash
git add leader/leadertest
git commit -m "Contain provider conformance timeouts" \
  --trailer "Constraint: etcd leases use integer-second TTLs and Campaign cleanup may block." \
  --trailer "Rejected: Provider capability skips | every mandatory case remains unchanged." \
  --trailer "Confidence: high" \
  --trailer "Scope-risk: moderate" \
  --trailer "Tested: go test and go test -race ./leader/leadertest"
```

### Task 2: Pin Dependencies and Define the etcd Value/Key Contract

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `leader/etcd/doc.go`
- Create: `leader/etcd/elector.go`
- Create: `leader/etcd/generation.go`
- Create: `leader/etcd/options.go`
- Create: `leader/etcd/options_test.go`

- [ ] **Step 1: Add RED constructor, TTL, key, and no-I/O tests**

Test nil client, invalid options, `RenewInterval < 100ms`, `RenewInterval >= Lease`, exact-second TTL, fractional round-up,
overflow, token uniqueness, slash-containing input, encoded sibling isolation, and no backend I/O.
Use these exact expectations:

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

- [ ] **Step 2: Observe RED and add pinned modules**

```bash
go test -count=1 ./leader/etcd -run 'TestNew|TestEncode|TestRequestedTTL'
go get go.etcd.io/etcd/client/v3@v3.6.13
go mod tidy
```

Expected: the first command fails on the deliberately missing etcd module and implementation
symbols (`clientv3`, `New`, `electionPaths`, and `requestedTTL`), not because the already-created
`leader/etcd` test package is absent. The module commands add only the selected etcd production
client plus its required transitive graph. The Testcontainers etcd module is deferred until Task 6
creates its importing fixture so `go mod tidy` cannot remove it.

- [ ] **Step 3: Implement the constructor-only API**

Use `base64.RawURLEncoding` for both segments and this state shape:

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

Keep the package name `etcdleader` and add Go doc comments for `Elector`, `New`, and `EffectiveTTL`.
The docs must state that the caller owns the client, `New` performs no network I/O, the `Elector`
zero value is unusable, and callers must construct it with `New`. Defer
`var _ leader.Elector = (*Elector)(nil)` until Task 5 has implemented the complete interface; do not
add temporary public method stubs merely to satisfy an early assertion.

Task 2's initial `generation` contains only `ttl time.Duration` and `published bool`; Task 3 expands
the same type with lifecycle ownership fields. This keeps `current *generation` compilable without
inventing a second state type.

`New` performs only normalization, duration math, key encoding, and a 128-bit `crypto/rand` token.
It never calls the client, creates a lease/session, or closes caller resources. `EffectiveTTL`
reads current published TTL, otherwise last published TTL, otherwise requested rounded TTL under
the state lock. Add synchronized transition tests proving requested -> published current ->
last-published behavior under concurrent readers and `go test -race`; an in-progress Grant and any
invalid/overflow server-granted TTL remain invisible and cannot replace the last valid value.

- [ ] **Step 4: Verify and commit**

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

### Task 3: Implement Generation-Safe Campaign and Local Session Join

**Files:**
- Modify: `leader/etcd/generation.go`
- Create: `leader/etcd/campaign.go`
- Create: `leader/etcd/monitor.go`
- Create: `leader/etcd/generation_test.go`
- Create: `leader/etcd/campaign_test.go`
- Modify: `leader/etcd/elector.go`

- [ ] **Step 1: Write RED lifecycle tests around injectable boundaries**

Keep production on official etcd types but give each elector its own unexported `etcdOps` bundle
for Grant, NewSession, Campaign, Proclaim, `snapshotElection`, watch-created, revoke, Get,
`sessionDone`, `orphanSession`, and ticker construction. `snapshotElection` returns an
`electionSnapshot{key string, createRev, headerRev int64}`; the production adapter reads the
official `Election.Key()`, `Election.Rev()`, and `Election.Header().Revision`, rejects a nil Header,
and validates the key plus both revisions before publication. Per-elector seams prevent parallel
tests from racing package globals while the official `*concurrency.Session` remains available to
`NewElection`. Tests must prove:

Task 3 adds `ops etcdOps` to `Elector`; production `New` installs real operations and tests replace
the bundle on an individual elector. Copy the bundle by value into each generation before any
remote dispatch. Once Campaign starts, neither production nor tests mutate that generation's copy.

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

Add cases for Grant failure, NewSession failure after a known lease, Campaign failure,
Proclaim failure, created-notify timeout, mismatched created PUT, callback join, failure after a
monitor handle exists, and Created acknowledgement immediately followed by DELETE or mismatched
PUT. For every successfully created Session, assert `Session.Done()` is closed before Campaign
returns. For every pre-publication monitor failure, assert its handle joins too.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./leader/etcd -run 'TestShutdownGeneration|TestCampaign|TestPublication'
```

Expected: FAIL on the deliberately missing generation lifecycle, operations bundle, and Campaign
symbols.

- [ ] **Step 3: Implement the generation record and one shutdown owner**

Use this minimum ownership record:

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

`shutdown` is nil-session-safe. One caller runs cancel then the per-elector `orphanSession`
operation when Session exists; all callers wait for `shutdownDone`. Production operations call
`Session.Orphan` and return `Session.Done`. A successfully created Session must have a closed Done
channel before shutdown completes. Unit tests inject a controllable Done channel and assert exact
orphan-before-close/join ordering on every exit. This proves local goroutine termination only and
must not clear remote cleanup inventory.

- [ ] **Step 4: Implement synchronous Campaign publication**

Implement the eight spec steps in order: reconcile pending cleanup; explicit Grant; NewSession
with `WithContext` and `WithLease`; `NewElection(session, electionBase)`; synchronous Campaign;
bounded Proclaim; call the injected `snapshotElection` adapter and validate key/revision/header;
create the exact-key watch from `headerRev+1` with `WithCreatedNotify`, hand that WatchChan plus the
injected `sessionDone` signal to a joinable monitor and receive its created acknowledgement; then
serialize caller cancellation against publication under `e.mu`. Publication is forbidden until
the monitor owns both observation inputs and has acknowledged watch creation. Task 3's
`monitor.go` owns this minimal start/Created/terminal/join primitive. Under `e.mu`, publication must
recheck both generation cancellation and the monitor's terminal state so a Created-then-loss event
cannot publish stale leadership.

Precompute the deterministic key as `candidateRoot + fmt.Sprintf("%x", leaseID)` and retain the
known lease even when NewSession fails. Any post-dispatch failure cancels and joins the generation,
attempts bounded revoke, joins any created monitor, and clears state only after revoke or exact
linearizable absence/replacement proof. Wrap public failures with
`leader.NewOperationError("etcd", "campaign", cause)` and join `leader.ErrCommitUnknown` when proof
is unavailable.

- [ ] **Step 5: Verify races and commit**

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

### Task 4: Add Exact-Key Monitoring and Single-Flight Proclaim

**Files:**
- Modify: `leader/etcd/monitor.go`
- Create: `leader/etcd/monitor_test.go`
- Modify: `leader/etcd/generation.go`
- Modify: `leader/etcd/campaign.go`

- [ ] **Step 1: Write RED monitor tests**

Use synchronized fake watch and Proclaim boundaries to cover Session loss, exact DELETE, mismatched
PUT token/revision/lease, compaction, watch cancellation/error, Proclaim failure, stale generation,
concurrent Resign, slow Proclaim, and operation counts. Assert:

```go
wantMax := int64(math.Ceil(float64(elapsed)/float64(opts.RenewInterval))) + 1
if got := control.OperationCount(opts, leadertest.OperationRenew); got > wantMax {
    t.Fatalf("renew count = %d, want <= %d", got, wantMax)
}
if maxInFlight.Load() != 1 { t.Fatalf("max in-flight Proclaim = %d", maxInFlight.Load()) }
```

Every loss must clear `IsLeader` before protected work can continue, invoke the same generation
shutdown helper, preserve cleanup when exact absence is unproved, and close the monitor done handle.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./leader/etcd -run 'TestMonitor|TestProclaim'
```

Expected: FAIL because monitor and renewal code are absent.

- [ ] **Step 3: Implement the monitor loop**

Select on `Session.Done`, the exact-key watch, and a `RenewInterval` ticker. Validate every PUT by
key, token, creation revision, and lease. Treat DELETE, mismatched PUT, compaction, watch error,
Session loss, and Proclaim failure as terminal loss. Proclaim uses a fresh generation-derived
context bounded by `min(RenewInterval, grantedTTL/4, 1s)` and never overlaps or queues. No mutex is
held during RPC, wait, shutdown, or join. A generation ID check prevents an old monitor clearing a
new owner. Construct the ticker with `time.NewTicker`, defer `ticker.Stop`, and add a terminal-path
test using an injected ticker factory whose stop counter must equal one for every monitor exit.

- [ ] **Step 4: Verify and commit**

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

### Task 5: Implement Proof-Based Resign, Reconciliation, and Leader Lookup

**Files:**
- Create: `leader/etcd/cleanup.go`
- Create: `leader/etcd/observe.go`
- Create: `leader/etcd/cleanup_test.go`
- Create: `leader/etcd/errors_test.go`
- Create: `leader/etcd/elector_contract_test.go`
- Modify: `leader/etcd/elector.go`

- [ ] **Step 1: Write RED cleanup and redaction tests**

Cover nil/canceled context, idempotent no-owner Resign, known/unknown revision, official nil Resign
with surviving key, revoke success, revoke ambiguity, exact absence, exact replacement, stale
revision, expired TTL without proof, retry on the same elector, concurrent Resign, caller-client
usability, linearizable oldest-candidate lookup, and forbidden markers. Require:

```go
if err == nil || !errors.Is(err, leader.ErrCommitUnknown) { t.Fatalf("resign error = %v", err) }
if !errors.Is(elector.Campaign(context.Background()), leader.ErrCleanupPending) {
    t.Fatal("Campaign did not preserve unresolved cleanup")
}
for _, marker := range []string{endpoint, username, password, token, encodedGroup, leaseID, certPath, rawCause} {
    if strings.Contains(err.Error(), marker) { t.Fatalf("error leaked %q", marker) }
}
```

Add a table for each synchronous public operation (`Campaign`, `Resign`, and `Leader`) proving nil
context returns bare `leader.ErrInvalidContext`, pre-dispatch cancellation/deadline returns the bare
context error, post-dispatch failure declares `var operationErr *leader.OperationError` and requires
`errors.As(err, &operationErr)`, preserves causes/sentinels through `errors.Is`, and exposes only
backend `etcd` plus operation `campaign`, `resign`, or `lookup`. Test asynchronous `renew` failure
separately: it must fail closed by clearing leadership, preserve unresolved cleanup inventory, and
emit no raw diagnostic through repository-owned observability. Feed the same forbidden markers
through every repository-owned test logger, example diagnostic helper, and telemetry stub; assert
none renders an unwrapped cause or raw marker.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./leader/etcd -run 'TestResign|TestReconcile|TestLeader|TestRedaction'
```

Expected: FAIL because cleanup and observation boundaries are absent.

- [ ] **Step 3: Implement shared cleanup**

Resign first clears local leadership and cancels the generation. Run/join generation shutdown,
then join the exact monitor. With a known revision, call
`concurrency.ResumeElection(session, candidateRoot, key, createRev)` and
official Resign. With unknown revision, reconcile deterministic key first. Every dispatched
official Resign result, nil or non-nil, is followed by bounded lease revoke when budget remains and
a default-linearizable exact-key Get; a non-nil response may still follow a committed delete. Clear state only
after successful revoke or proof of exact absence/replacement; compare key, creation revision,
token, and lease wherever known. Elapsed TTL and `Session.Done` never clear inventory.

If proof fails, return a redacted resign operation error joined with `leader.ErrCommitUnknown` and
retain the generation for a same-elector retry. Concurrent calls share the shutdown result and only
one may clear the generation.

- [ ] **Step 4: Implement observation and public error labels**

`Leader` uses one default-linearizable Get over `[candidateRoot, rangeEnd)`. Assemble options as
`opts := append([]clientv3.OpOption{clientv3.WithRange(rangeEnd)}, clientv3.WithFirstCreate()...)`
because `WithFirstCreate` returns a slice. Return empty on no candidate and the oldest candidate value otherwise.
Expose only `campaign`, `renew`, `resign`, and `lookup` as operation labels; map Grant, Session,
watch, Proclaim, revoke, and reconciliation phases to the owning public operation.
At this point add `var _ leader.Elector = (*Elector)(nil)` beside the implementation. Add an
external-package zero-value contract test: `IsLeader` is false, Campaign and Leader fail
deterministically for valid contexts without panic or backend dispatch, and Resign with a valid
context is a safe no-op; nil contexts still return `leader.ErrInvalidContext`. The test does not make
the zero value supported; it enforces the documented constructor-only failure boundary.

- [ ] **Step 5: Verify and commit**

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

### Task 6: Build the Real etcd Fixture and Mandatory Conformance Adapter

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `leader/etcd/etcd_test_fixture_test.go`
- Create: `leader/etcd/integration_test.go`
- Create: `leader/etcd/conformance_test.go`

- [ ] **Step 1: Write the serial real-server fixture**

Import the Testcontainers etcd module in the fixture first, then add it so tidy retains the direct
test dependency:

```bash
go get github.com/testcontainers/testcontainers-go/modules/etcd@v0.42.0
go mod tidy
go mod verify
```

Resolve the Docker container target platform, never the Go host OS, and map it to the approved
digest:

```go
var etcdDigest = map[string]string{
    "linux/amd64": "sha256:946dfbae58b1dec56af786a23e7322484b58281547bef1e848321f6beeb388d5",
    "linux/arm64": "sha256:23c14fbdf70105a54146cf5ed3a81613b99a973c60d5907851a251ca15664e96",
}
```

`containerPlatform(ctx)` first honors a non-empty `DOCKER_DEFAULT_PLATFORM`; otherwise it opens the
Testcontainers Docker client, reads daemon `Info`, and derives `OSType/Architecture`. Normalize only
the architecture aliases `x86_64 -> amd64` and `aarch64 -> arm64`, require Linux, and accept only
`linux/amd64` or `linux/arm64`. The fixture may import `github.com/moby/moby/client` for
`InfoOptions`; `go mod tidy` may promote that already-present Testcontainers dependency to direct,
but must not change its selected version.

Reject every platform absent from the map. Pass both the immutable reference
`"gcr.io/etcd-development/etcd@" + etcdDigest[platform]` and
`testcontainers.WithImagePlatform(platform)` directly to `etcd.Run`; keep `v3.6.13` only as
recorded version metadata. Add table tests for environment override, daemon fallback, aliases,
unsupported OS/architecture, and env-over-daemon precedence. Do not launch the mutable tag.

Readiness requires bounded `Status` member/leader evidence and one linearizable Put/Get/Delete
roundtrip. Register client and partially created container cleanup with `internal/testcleanup`; log
tag, digest, platform, and endpoint count without credentials.

- [ ] **Step 2: Add RED real-election tests and control**

Add acquire/observe, 16-contender exact winner, canceled waiter with no late candidate, keepalive
beyond requested lease, external revoke, key loss, watch interruption, idempotent resign,
lost-response retry, stale revision, caller-client reuse, and stable single-node restart cases.

Implement a concurrency-safe test control whose `ReplaceOwner` revokes the current leader lease,
creates a control lease/session, Campaigns the replacement, immediately calls `Session.Orphan`, and
retains its lease ID for teardown. `FailNext` runs the real Campaign/Proclaim/Resign first and loses
only the response. `OperationCount` is key-and-operation-specific and monotonic.

- [ ] **Step 3: Observe RED then run the mandatory suite with exact timing**

```bash
go test -p 1 -count=1 ./leader/etcd -run 'TestEtcdIntegration|TestEtcdElectorConformance'
```

Use:

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
first creation registers one cleanup; later factories cannot replace or leak the entry. `closeFor`
atomically detaches the entry and best-effort closes it even when the Abort context is already
ended, returning `errors.Join(ctx.Err(), closeErr)` as applicable. Thus Abort closes every shared
user in that case and never a client used by a different case. Expected: all 15 existing named
cases PASS with no skips or relaxed assertions.

- [ ] **Step 4: Verify and commit**

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

### Task 7: Harden Trust Boundaries, Faults, and Resource Containment

**Files:**
- Create: `leader/etcd/security_test.go`
- Create: `leader/etcd/resource_test.go`
- Create: `leader/etcd/shutdown_test.go`
- Modify: `leader/etcd/integration_test.go`
- Modify: `leader/etcd/conformance_test.go`

- [ ] **Step 1: Add authenticated boundary tests**

Use a separate serial container that no unauthenticated test shares. Bootstrap the root user and
root role before `AuthEnable`, reconnect with root credentials, then create principals/roles A and
B. Grant each role exactly its encoded `[candidateRoot, rangeEnd)` through
`RoleGrantPermission`; create separate authenticated clients and register bounded teardown that
disables auth and terminates the isolated container. Assert A can Put/Get/Delete/Watch its own
range and every same operation is denied on B's sibling range, with symmetric assertions for B.

On v3.6.13, attach A's candidate key to A's lease and assert B's authenticated cross-principal
Revoke and `KeepAliveOnce` are denied because server authorization checks every attached key. Also
prove B can revoke an unattached lease, then loses that ability after an A-range key is attached.
Record these pinned results without treating lease IDs as creator-owned or prefix-scoped
capabilities. Principals with the same range remain mutually trusted, mutually untrusted tenants
always use separate clusters, and every server-version change reruns both denial tests. Mark all
plaintext fixtures test-only.

- [ ] **Step 2: Add deterministic lifecycle interleavings**

Run barrier-controlled tests for cancellation versus publication, Resign versus monitor loss,
watch-created response followed immediately by mismatched PUT/DELETE, compaction, Proclaim delay,
post-success lost responses, cleanup reconciliation ABA, and rapid reacquisition. The rapid
reacquisition test must stop and join prior protected work before a new Campaign or use an explicit
test fencing generation.

- [ ] **Step 3: Prove the blocked-Campaign hard-stop branch**

Inject a Campaign blocked in official cleanup on `client.Ctx`. Cancel the case root, wait the join
grace, coordinate every user of that case-dedicated client, close it, and assert the Campaign plus
Session/monitor handles join. Preserve cleanup inventory, open a separate healthy diagnostic
client, and require linearizable exact-range absence/replacement proof before clearing the test's
restart gate. A timeout invokes the subprocess fail-stop instead of leaking the blocked call.

- [ ] **Step 4: Add the 32-contender resource test**

Before starting contenders, capture `Lease.Leases` count, scrape the server
`etcd_debugging_mvcc_watcher_total` metric, and read package-private atomic counters for live
Sessions, published monitors, and in-flight Proclaims. Start 32 contenders in one group; subtract
the baseline and poll with the configured wait deadline. Assert at most 32 live leases, 32 live
Sessions, 32 server watchers, one published monitor, and one in-flight Proclaim. Cancel,
Resign/reconcile, close case clients, and poll until every baseline delta is exactly zero. Do not
assert the process-global goroutine count.

- [ ] **Step 5: Run normal and race gates and measure admission inputs**

```bash
/usr/bin/time -p go test -p 1 -count=1 ./leader/etcd
/usr/bin/time -p go test -race -p 1 -count=1 ./leader/leadertest ./leader/etcd
```

Expected: PASS with one in-flight Proclaim, no late candidate, all local Session/monitor handles
joined, no forbidden diagnostic marker, and fixture resources back at baseline. Record durations;
the ten-run soak remains Task 9's admission-gated pre-release command.

- [ ] **Step 6: Commit hardening evidence**

```bash
git add leader/etcd
git commit -m "Harden etcd election failure boundaries" \
  --trailer "Constraint: etcd KV prefixes do not automatically isolate lease revocation." \
  --trailer "Confidence: high" \
  --trailer "Scope-risk: broad" \
  --trailer "Tested: real-server suite, race suite, auth, hard-stop, and 32-contender tests"
```

### Task 8: Publish Bilingual Usage, Shutdown, Migration, and Lesson Guidance

**Files:**
- Create: `leader/etcd/README.md`
- Create: `leader/etcd/README.ko.md`
- Create: `leader/etcd/example_test.go`
- Create: `leader/etcd/readme_test.go`
- Create: `docs/lessons/2026-07-19-issue-537-etcd-leader.md`
- Modify: `leader/elector.go`
- Modify: `leader/README.md`
- Modify: `leader/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`
- Modify: `docs/release/v0.19.0-provider-conformance-runbook.md`

- [ ] **Step 1: Add compile-checked examples before prose**

Add one acquire/work/resign example and one shutdown-supervisor example. Both must inspect
`campaignCtx.Err()` even after nil Campaign return, stop and join protected work when `IsLeader`
clears, preserve initiating plus cleanup errors, retry Resign on the same elector, use
`EffectiveTTL` only to schedule another reconciliation, and never infer cleanup from elapsed time.
The supervisor sequence is: cancel campaigns, bounded join grace, same-elector Resign while the
client is healthy, coordinate shared-client users, close the caller client only for blocked calls,
join them, persist unresolved inventory, then require a separate healthy client to prove exact
range absence before restart.
Extend the compile-checked supervisor through symmetric rollback: stop protected work and every
etcd contender, perform bounded same-elector cleanup, prove exact range absence with a healthy
diagnostic client, re-enable the previous provider, and verify zero etcd contenders.

Add a compile-checked caller-owned production client example that loads a CA pool, sets a non-empty
`ServerName`, leaves `InsecureSkipVerify=false`, supplies scoped authentication, and passes the
resulting `*clientv3.Client` to `New`. A focused test rejects an example config with an empty root
pool, empty ServerName, invalid credentials, or `InsecureSkipVerify=true`; no production TLS
ownership moves into the provider.

- [ ] **Step 2: Write section-for-section English/Korean provider docs**

Include install/client ownership, the constructor-only contract and unusable `Elector` zero value,
encoded ranges, integer/server-granted TTL, Proclaim versus keepalive, cancellation limitation,
exact local Session join, cleanup pending, RBAC lease-level trust, TLS, quorum/compaction, no
fencing, observability sampling, shutdown, stop-the-world cutover, symmetric rollback, rapid
reacquisition rule, tested server version, and unsupported scope. State in both languages that
callers must use `New` and that `errors.Unwrap` exposes diagnostic-sensitive raw etcd causes which
must not be logged or emitted to telemetry without sanitization.

- [ ] **Step 3: Update indexes, changelog, and release runbook**

Register `leader/etcd` in both leader and root README pairs. Add the v0.19.0 change with the
caller-owned client and no-fencing caveat. Extend the provider runbook with preflight Status plus
linearizable roundtrip, exact-range RBAC, campaign drain, unresolved inventory, cutover/rollback,
quorum recovery, and dependency rollback gates.

Update the backend-neutral `leader.Elector.Campaign` Go doc so it no longer promises TTL expiry as
a universal cleanup fallback: callers retry bounded Resign on the same elector and then follow the
provider-specific proof/expiry contract; etcd requires successful revoke or linearizable exact-key
reconciliation.

- [ ] **Step 4: Record the Type A lesson**

Capture four reusable findings with concrete evidence: official Campaign uses long-lived client
context for cleanup, server-granted TTL controls budgets, Session plus exact watch plus Proclaim are
distinct ownership signals, and provider timing profiles must preserve cases while containing
timed-out goroutines.

- [ ] **Step 5: Verify parity and commit**

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

`TestReadmeParity` extracts ordered `##` headings from both provider README files and requires a
one-to-one section mapping covering every Step 2 topic. `TestRunbookContract` requires executable
command/checklist blocks for Status plus linearizable roundtrip, campaign drain, separate-client
reconciliation, exact absence, symmetric rollback, dependency rollback (`git diff go.mod go.sum`,
provider registration removal, `go mod tidy`, `make ci`), quorum recovery, and observability
sampling cadence; a keyword-only presence check is insufficient.

### Task 9: Run Final Verification and Prepare Review Evidence

**Files:**
- Create later: `docs/superpowers/reviews/2026-07-19-issue-537-etcd-leader-step-6r-code-review.md`
- Create later: `docs/superpowers/reviews/2026-07-19-issue-537-etcd-leader-step-7r-pr-review.md`
- Conditionally modify if admission fails: `Makefile`
- Conditionally modify if admission fails: `.github/workflows/ci.yml`
- Conditionally modify if admission fails: `.github/workflows/nightly-tests.yml`
- Verify: every issue #537 file and dependency delta

- [ ] **Step 1: Verify dependency shape**

```bash
go list -m go.etcd.io/etcd/client/v3 github.com/testcontainers/testcontainers-go/modules/etcd
go mod verify
go mod tidy
git diff --exit-code -- go.mod go.sum
```

Expected: etcd client is v3.6.13, Testcontainers etcd is v0.42.0, module verification passes,
and tidy leaves no diff. Record gRPC, protobuf, Prometheus, `x/net`, `x/sys`, and zap graph changes
in the implementation review.

- [ ] **Step 2: Run targeted, race, and repository gates serially**

```bash
go test -p 1 -count=1 ./leader/leadertest ./leader/etcd
go test -race -p 1 -count=1 ./leader/leadertest ./leader/etcd
make fmt-check
make tidy-check
make vet
make lint
make ci
```

Expected: every command PASS. Record targeted/race/full-CI durations. If normal targeted tests
exceed three minutes, race exceeds five minutes, combined exceeds eight minutes, or projected full
CI exceeds 20 minutes/80% of the job timeout, move repeated Docker/race work to a separate CI lane
before PR review. Command timings are diagnostic only; workflow admission is based on the complete
target job, including checkout, Go setup/cache, Docker preflight, cold image pull, existing
workloads, and every test step.

For that conditional branch, parameterize `Makefile` test/coverage/race package lists so CI can run
all packages except `./leader/etcd` without changing local defaults. Add an `etcd-leader` job to
`.github/workflows/ci.yml` with `needs: ci`, `timeout-minutes: 15`, Docker preflight, and serial
`go test -p 1 -count=1` plus `go test -race -p 1 -count=1` for
`./leader/leadertest ./leader/etcd`; keep it a required PR check. Put the nightly repetition in a
dedicated `etcd-leader-soak` job with `timeout-minutes: 30`; never append it to the existing
Testcontainers workload. Before push, run a CI-equivalent sequence including Docker preflight,
immutable image pull, setup-equivalent module download, and every target job command. After push,
read each live job's `startedAt`/`completedAt` from GitHub Actions and require the complete PR job to
finish within 12 minutes and the complete soak job within 24 minutes (80% of their timeouts). If a
job exceeds the limit, increase its timeout with explicit 20% headroom or reduce only the nightly
repetition count to the largest measured value within the limit while retaining the ten-run manual
pre-release gate; rerun exact-head CI before merge readiness.

Validate every local workflow diff mandatorily with
`go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 .github/workflows/*.yml`, then run both
separated local command sets before accepting the split. Use `gh workflow view` only after push as
remote registration evidence. Do not hide the package from CI or make the new job optional.

- [ ] **Step 3: Run the pre-release soak under the admission rule**

```bash
/usr/bin/time -p go test -p 1 -count=10 ./leader/etcd
```

Expected: PASS when the measured admission rule permits local execution; otherwise record it as a
nightly/manual pre-release gate and do not place the repeated cold-container run in the normal PR
job.

- [ ] **Step 4: Run Step 6-R and Step 7-R exact-head reviews**

Run six independent lanes at the same exact commit for performance, stability, security,
operator/Ops, developer/API, and user/caller, then perform the main-session integration review.
Repair every P0/P1 and rerun all six at the new exact head. Record `P0=0 P1=0`, commands, durations,
module graph, docs parity, unresolved P2/P3, and review provenance in the Step 6-R artifact. After
push and live CI, repeat the exact-head PR review for Step 7-R.

- [ ] **Step 5: Commit review evidence without claiming live CI**

```bash
git add docs/superpowers/reviews/2026-07-19-issue-537-etcd-leader-step-6r-code-review.md
git commit -m "Record the etcd implementation verdict" \
  --trailer "Constraint: Merge readiness requires exact-head local and live review evidence." \
  --trailer "Confidence: high" \
  --trailer "Scope-risk: narrow" \
  --trailer "Tested: targeted, race, lint, module, full CI, and six-lane Step 6-R gates" \
  --trailer "Not-tested: GitHub CI remains pending until the approved PR is pushed."
```

Expected: the branch is clean, all local checks pass, the Type A lesson exists, and PR creation may
use the already authorized `bluetape4k/bluetape-go` `develop` <- `feat/issue-537-etcd-leader`
scope. Stop at merge-ready after exact-head CI/review evidence and obtain a fresh merge approval.
