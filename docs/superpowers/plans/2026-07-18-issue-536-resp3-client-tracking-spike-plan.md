# RESP3 CLIENT TRACKING Spike Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove or reject Redis RESP3 `CLIENT TRACKING` as a coherent public near-cache invalidation strategy using only public go-redis APIs, Testcontainers evidence, and L1-only TieredCache hooks.

**Architecture:** One external-package spike test file owns a strict test-only push handler and four isolated Redis client roles: tracking, TieredCache L2, external writer, and disposable admin. Unit tests lock payload, redaction, synchronization, and unregister behavior before Redis integration proves command-coupled delivery, connection affinity, global invalidation, reconnect loss, and ordered shutdown. A research ledger adopts only directly observed capabilities and keeps `redisnear.NewPubSub` as production unless every coherence condition is met.

**Tech Stack:** Go 1.26.3, `github.com/redis/go-redis/v9` v9.20.0, Redis 7.4 Testcontainers, `cache.Memory`, `cache/redisvalue.TieredCache`, `serialization.StringSerializer`, standard-library synchronization, and repository Make targets.

---

## Preconditions

- Approved design: `docs/superpowers/specs/2026-07-18-issue-536-resp3-client-tracking-spike-design.md`
- Test specification: `docs/superpowers/specs/2026-07-18-issue-536-resp3-client-tracking-spike-test-spec.md`
- Step 2-R baseline: exact design commit
  `38364100af92d6da616ff89101109fc768e639a4`, `P0=0 P1=0 P2=0`; the Step 3-R
  repair candidate requires a fresh exact-commit Step 2-R delta review.
- Classification: Type B. Stop before any production file, exported tracking API, dependency change, background pump, reconnect subsystem, or exported physical-key mapper.

## File Map

Create:

- `cache/redisnear/resp3_tracking_spike_test.go` — external-package handler unit tests, fixture, and RESP3 capability matrix; the only Go file added.
- `docs/research/2026-07-18-issue-536-resp3-client-tracking-spike.md` — environment, evidence matrix, Pub/Sub comparison, provider assumptions, and decision.

Modify:

- `docs/research/README.md`
- `docs/research/README.ko.md`

Do not modify production `cache/redisnear` or `cache/redisvalue` files,
`go.mod`, `go.sum`, package/root README files, `CHANGELOG.md`, or `Makefile`.

## Coverage Matrix

| Requirement | Planned proof |
|---|---|
| Public RESP3 push handling | Tasks 1-3 |
| Idle stale L1 before drain | Task 3 |
| Physical connection affinity | Task 3 |
| Global L1 clear | Task 4 |
| Disconnect, missed invalidation, re-enable | Task 4 |
| Redacted bounded handler failure | Task 1 |
| Non-quiescent unregister and shutdown order | Task 5 |
| Pub/Sub/provider decision | Task 6 |
| No public API or benchmark claim | File map, Task 6, and Type A stop rule |

### Task 1: Lock the Test-Only Handler Contract

**Files:**
- Create: `cache/redisnear/resp3_tracking_spike_test.go`

- [ ] **Step 1: Write failing handler tests**

Use package `redisnear_test`. Define a concurrency-safe fake implementing:

```go
type localInvalidator interface {
	InvalidateLocal(context.Context, string) error
	ClearLocal(context.Context) error
}
```

Write these tests exactly:

- `TestRESP3TrackingSpikeHandlerRejectsUnsafePayloadsWithoutDisclosure`
  covers frame arity, notification type, collection type, non-string/empty key,
  empty key list, 65 keys, 2049-byte key, 33 allowlisted 2 KiB keys exceeding
  64 KiB aggregate, duplicate, and foreign
  sensitive-marker key. Every row requires zero local calls, one typed failure
  observation, `overflow=false`, and only `errRESP3InvalidationRejected`; the
  empty list uses reason `key-count`.
- `TestRESP3TrackingSpikeHandlerProcessesBoundedMultiKeyPayload` requires two
  exact payload-order calls, no full clear, one success event with count two,
  and no overflow.
- `TestRESP3TrackingSpikeHandlerReportsLocalCleanupFailure` covers middle-key
  partial failure plus full-clear repair, repair failure, global clear failure,
  canceled handler root, and cleanup deadline. The three-key middle-failure
  rows require first/second attempted, third skipped, exactly one full-clear
  repair, and the correct `repaired` state. Injected
  sensitive text must be absent from returned/loggable errors and observations.
- `TestRESP3TrackingSpikeHandlerOverflowDoesNotBlock` fills a capacity-one
  observation channel, invokes again, and requires return within a 1-second
  watchdog, `overflow=true`, and the low-cardinality sentinel.
- Success rows require exact physical allowlist lookup, logical-key invalidation,
  nil/global clear, exact event count, and no overflow.

- [ ] **Step 2: Run tests and verify RED**

```bash
go test -count=1 ./cache/redisnear -run '^TestRESP3TrackingSpikeHandler'
```

Expected: FAIL to compile because the handler does not exist.

- [ ] **Step 3: Implement the minimal strict handler in the test file**

Use these exact bounds and state:

```go
const (
	maxSpikeInvalidationKeys  = 64
	maxSpikePhysicalKeyBytes  = 2 << 10
	maxSpikeAggregateKeyBytes = 64 << 10
)

var errRESP3InvalidationRejected = errors.New("resp3 invalidation rejected")

type invalidationObservation struct {
	success  bool
	global   bool
	count    int
	reason   observationReason
	repaired bool
}

type spikeHandler struct {
	root              context.Context
	local             localInvalidator
	allowed           map[string]string
	keyCleanupTimeout time.Duration
	repairTimeout     time.Duration
	events            chan invalidationObservation
	overflow          atomic.Bool
	gate              callbackGate
}

type callbackGate struct {
	mu     sync.Mutex
	closed bool
	wg     sync.WaitGroup
}
```

Implement `callbackGate.begin` so it locks `mu`, rejects when `closed`, or
calls `wg.Add(1)` before unlocking. `close` flips `closed` under that same
mutex. `done` delegates to `wg.Done`; `wait` delegates to `wg.Wait`. Calling
`close` before `wait` makes late `Add` impossible.

`HandlePushNotification` must:

1. call `gate.begin`, return the redacted rejection sentinel when closed, and
   defer `gate.done` only after successful admission; before returning on a
   closed gate, non-blockingly record one `reason=shutdown` failure event;
2. accept exactly `[]interface{}{"invalidate", nil}` or a non-empty
   `[]interface{}` key list;
3. enforce count/per-key/aggregate/duplicate limits;
4. map by copied exact `map[string]string`, never trim a prefix;
5. validate and map the whole payload before any cleanup;
6. create `context.WithTimeout(h.root, h.keyCleanupTimeout)` independently of
   the go-redis command context; use a 1-second default and inject 100 ms only
   in the timeout subtest;
7. invalidate logical keys in payload order; on the first failure stop later
   key attempts and call `ClearLocal` exactly once with a fresh context derived
   from `h.root` and the independent 250 ms `repairTimeout`;
8. call only `InvalidateLocal` or `ClearLocal`;
9. record with non-blocking `select`, setting overflow on default;
10. return only `errRESP3InvalidationRejected` on every failure.

Use low-cardinality reasons: `shape`, `type`, `key-count`, `key-size`,
`aggregate-size`, `duplicate`, `unknown-key`, `local-cleanup`,
`repair-failed`, `cleanup-timeout`, `shutdown`, and `observation-overflow`.

- [ ] **Step 4: Verify GREEN and commit**

```bash
gofmt -w cache/redisnear/resp3_tracking_spike_test.go
go test -count=1 ./cache/redisnear -run '^TestRESP3TrackingSpikeHandler'
git add cache/redisnear/resp3_tracking_spike_test.go
git commit -m "Constrain the RESP3 spike callback boundary" \
  -m "Constraint: go-redis logs and swallows notification handler failures" \
  -m "Rejected: Raw callback errors | they can disclose invalidation payloads" \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Directive: Keep the handler test-only, bounded, allowlisted, and L1-only" \
  -m "Tested: focused RESP3 handler unit tests"
```

Expected: PASS and one test-only file committed.

### Task 2: Build the Disposable Redis Fixture

**Files:**
- Modify: `cache/redisnear/resp3_tracking_spike_test.go`

- [ ] **Step 1: Write a failing prerequisite test**

`TestRESP3TrackingSpikeNegotiatesProtocolAndRecordsServer` must start the
fixture, assert `HELLO 3` returns `proto == int64(3)`, record non-empty
`redis_version`, prove `Inspect.Config.Image == "redis:7.4-alpine"`, record a
non-empty `Inspect.Image` engine identity, obtain `CLIENT ID`, and enable
`CLIENT TRACKING ON NOLOOP`.

- [ ] **Step 2: Run and verify RED**

```bash
go test -p 1 -count=1 -timeout=5m ./cache/redisnear -run '^TestRESP3TrackingSpikeNegotiatesProtocolAndRecordsServer$'
```

Expected: FAIL because fixture helpers do not exist.

- [ ] **Step 3: Implement four isolated client owners**

```go
type resp3SpikeFixture struct {
	addr          string
	configuredImg string
	engineImageID string
	processor     push.NotificationProcessor
	tracking      *redis.Client
	l2            *redis.Client
	writer        *redis.Client
	admin         *redis.Client
	serverInfo    map[string]string
}
```

`newRESP3SpikeFixture(t, poolSize)` must call the existing upstream module
directly so actual container identity remains inspectable:

```go
container, err := tcredis.Run(ctx, "redis:7.4-alpine")
if err != nil {
	t.Fatal(testcleanup.FormatStartError("redis", "redis:7.4-alpine", err))
}
testcleanup.Register(ctx, t, "resp3 redis", container)
inspect, err := container.Inspect(ctx)
if err != nil {
	t.Fatal(err)
}
if inspect.Config == nil {
	t.Fatal("missing container image config")
}
if inspect.Config.Image != "redis:7.4-alpine" {
	got := inspect.Config.Image
	if len(got) > 128 {
		got = got[:128]
	}
	t.Fatalf("configured image = %q", got)
}
if inspect.Image == "" {
	t.Fatal("empty engine image identity")
}
addr, err := container.PortEndpoint(ctx, "6379/tcp", "")
if err != nil {
	t.Fatal(err)
}
```

This uses the already-declared Testcontainers Redis module and adds no
dependency. Then create the retained processor and construct:

```go
tracking := redis.NewClient(&redis.Options{
	Addr: addr, Protocol: 3, PoolSize: poolSize, MaxRetries: -1,
	PushNotificationProcessor: processor,
})
l2 := redis.NewClient(&redis.Options{Addr: addr, Protocol: 3})
writer := redis.NewClient(&redis.Options{Addr: addr, Protocol: 3})
admin := redis.NewClient(&redis.Options{Addr: addr, Protocol: 3})
```

The fixture accepts no caller-provided client/options/dialer/endpoint. Parse
`INFO server` into a map. `flushDB` and `killID` are unexported fixture methods;
`killID` calls:

```go
f.admin.ClientKillByFilter(ctx, "ID", strconv.FormatInt(id, 10)).Result()
```

Immediately after all four clients are constructed, register a fixture
`t.Cleanup`. The container cleanup was registered first, so LIFO closes Redis
resources before termination. Define a mutex-protected registry of
`idempotentCloser` entries; every sticky connection and each of the four
clients receives its own `sync.Once`. Explicit reconnect/shutdown closes and
final cleanup call the same entry. Final cleanup closes sticky connections
before clients. No fallible fixture operation may occur between allocating the
four clients and registering this cleanup.

Use `closeWithin(t, name, func() error)` with a buffered result channel and a
1-second watchdog. Document in the helper comment that timeout observes but
cannot cancel context-free `Close`.

- [ ] **Step 4: Verify GREEN and commit**

```bash
gofmt -w cache/redisnear/resp3_tracking_spike_test.go
go test -p 1 -count=1 -timeout=5m ./cache/redisnear -run '^TestRESP3TrackingSpike(Negotiates|Handler)'
git add cache/redisnear/resp3_tracking_spike_test.go
git commit -m "Isolate the RESP3 spike connection owners" \
  -m "Constraint: Sticky tracking and destructive admin work need separate owners" \
  -m "Rejected: Caller-supplied admin | a custom dialer can bypass address checks" \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Tested: Redis 7.4 negotiation and server identity proof"
```

### Task 3: Prove Command-Coupled Delivery and Affinity

**Files:**
- Modify: `cache/redisnear/resp3_tracking_spike_test.go`

- [ ] **Step 1: Add exact key and TieredCache helpers**

Build the physical key only with public APIs:

```go
builder, err := btredis.NewKeyBuilder("bluetape:cache:value")
if err != nil {
	t.Fatal(err)
}
builder, err = builder.Structural(namespace)
if err != nil {
	t.Fatal(err)
}
key, err := builder.LogicalKey(logical)
if err != nil {
	t.Fatal(err)
}
physical := key.Value
```

Construct `ValueCache[string]` with `serialization.StringSerializer{}` and a
separate L2 client. Construct `TieredCache[string]` with `cache.NewMemory`,
remote TTL 10 minutes, local TTL 5 minutes, invalidation wait 250 ms, and local
cleanup 100 ms.

- [ ] **Step 2: Write and run the command-coupled RED test**

`TestRESP3TrackingSpikeDeliversInvalidationOnlyWhenTrackedConnectionReads`
must execute:

1. TieredCache set/get `old`;
2. tracking connection enable + physical `GET`;
3. external writer `SET new` completion barrier;
4. immediate non-blocking absence assertion;
5. TieredCache stale `old` assertion;
6. tracked `PING`;
7. exactly one successful key event, overflow false;
8. TieredCache fresh `new` assertion.

```bash
go test -p 1 -count=1 -timeout=5m ./cache/redisnear -run '^TestRESP3TrackingSpikeDeliversInvalidationOnlyWhenTrackedConnectionReads$' -v
```

Expected: RED until registration/read ordering and exact payload mapping are
correct; never remove the stale-before-`PING` assertion.

- [ ] **Step 3: Write the affinity test**

`TestRESP3TrackingSpikeRequiresReadAndTrackingOnSameConnection` uses
`PoolSize: 2`, holds A/B, asserts distinct `ClientID` values, enables tracking
only on A, then proves B read + writer mutation yields no event while A read +
writer mutation + A drain yields exactly one event.

- [ ] **Step 4: Verify GREEN and commit**

```bash
gofmt -w cache/redisnear/resp3_tracking_spike_test.go
go test -p 1 -count=1 -timeout=5m ./cache/redisnear -run '^TestRESP3TrackingSpike(DeliversInvalidationOnlyWhenTrackedConnectionReads|RequiresReadAndTrackingOnSameConnection)$' -v
git add cache/redisnear/resp3_tracking_spike_test.go
git commit -m "Prove RESP3 invalidation is connection-coupled" \
  -m "Constraint: L1 hits issue no Redis command and pools hide physical affinity" \
  -m "Rejected: Background heartbeat | it would create an unreviewed subsystem" \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Tested: command-coupled delivery and two-connection affinity"
```

If invalidation arrives autonomously without a tracked command, stop and record
the runtime/source divergence before changing the decision.

### Task 4: Prove Global Flush and Reconnect Loss

**Files:**
- Modify: `cache/redisnear/resp3_tracking_spike_test.go`

- [ ] **Step 1: Add global invalidation proof**

`TestRESP3TrackingSpikeMapsGlobalInvalidationToClearLocal` tracks/caches two
keys, calls fixture-owned `FLUSHDB`, drains, requires one successful global
event and overflow false, then requires `cache.ErrCacheMiss` for both keys.

- [ ] **Step 2: Add reconnect proof**

`TestRESP3TrackingSpikeReconnectRequiresReenableAndLocalFlush` executes:

1. track/cache `old`, record ID A;
2. `CLIENT KILL ID A == 1`;
3. writer sets `new` while disconnected;
4. retry-disabled `PING` returns transport error;
5. TieredCache still returns stale `old`, with no replayed event;
6. test harness blocks further use and calls `ClearLocal`;
7. close dead connection through watchdog;
8. replacement ID B differs;
9. `HELLO 3` succeeds and `CLIENT TRACKINGINFO` flags equal `off`;
10. re-enable, track/cache `new`, mutate `newer`, drain, and read `newer`.

Parse RESP3 tracking info as `map[interface{}]interface{}` and convert its
`flags` `[]interface{}` to strings with strict type errors.

- [ ] **Step 3: Run RED/GREEN and commit**

```bash
gofmt -w cache/redisnear/resp3_tracking_spike_test.go
go test -p 1 -count=1 -timeout=5m ./cache/redisnear -run '^TestRESP3TrackingSpike(MapsGlobalInvalidationToClearLocal|ReconnectRequiresReenableAndLocalFlush)$' -v
git add cache/redisnear/resp3_tracking_spike_test.go
git commit -m "Expose the RESP3 reconnect coherence gap" \
  -m "Constraint: Connection-local tracking cannot replay invalidations missed during loss" \
  -m "Rejected: OnConnect-only recovery | it can retain stale L1 state" \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Directive: Preserve clear-before-reenable ordering" \
  -m "Tested: global invalidation and reconnect cases"
```

Expected: PASS without a production reconnect state machine.

### Task 5: Prove Unregister and Shutdown Semantics

**Files:**
- Modify: `cache/redisnear/resp3_tracking_spike_test.go`

- [ ] **Step 1: Prove unregister is not quiescence**

`TestRESP3TrackingSpikeUnregisterIsNotAQuiescenceBarrier` obtains the handler
from `processor.GetHandler`, starts it against a latch-blocked invalidator,
unregisters, proves the callback remains in flight, releases it, and observes
completion within 1 second.

- [ ] **Step 2: Prove ordered shutdown**

`TestRESP3TrackingSpikeShutdownOrdersQuiescenceBeforeUnregister` starts and
holds one callback inside the invalidator, calls `gate.close`, proves a later
dispatch returns `errRESP3InvalidationRejected`, records exactly one
`reason=shutdown` event with `overflow=false`, and does not enter the
invalidator. Then start `gate.wait` through a 1-second watchdog. Prove the wait
is blocked before releasing the first callback, release it, require the wait to
finish, unregister, assert `GetHandler("invalidate") == nil`, then close the
connection and client through their idempotent fixture closers.

- [ ] **Step 3: Verify and commit**

```bash
go test -count=1 ./cache/redisnear -run '^TestRESP3TrackingSpike(UnregisterIsNotAQuiescenceBarrier|ShutdownOrdersQuiescenceBeforeUnregister)$' -v
go test -race -count=1 ./cache/redisnear -run '^TestRESP3TrackingSpike(Handler|Unregister|Shutdown)'
git add cache/redisnear/resp3_tracking_spike_test.go
git commit -m "Make RESP3 handler shutdown limits explicit" \
  -m "Constraint: Unregister does not quiesce an already selected callback" \
  -m "Rejected: Close-before-unregister | handler ownership stays ambiguous" \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Tested: unregister ordering, close watchdog, and handler race tests"
```

### Task 6: Publish the Evidence Ledger

**Files:**
- Create: `docs/research/2026-07-18-issue-536-resp3-client-tracking-spike.md`
- Modify: `docs/research/README.md`
- Modify: `docs/research/README.ko.md`

- [ ] **Step 1: Run and capture exact evidence**

```bash
go test -p 1 -count=1 -timeout=5m ./cache/redisnear -run '^TestRESP3TrackingSpike' -v
go test -p 1 -count=3 -timeout=15m ./cache/redisnear -run '^TestRESP3TrackingSpike(DeliversInvalidationOnlyWhenTrackedConnectionReads|RequiresReadAndTrackingOnSameConnection|MapsGlobalInvalidationToClearLocal|ReconnectRequiresReenableAndLocalFlush|UnregisterIsNotAQuiescenceBarrier|ShutdownOrdersQuiescenceBeforeUnregister)$' -v
go test -race -count=1 ./cache/redisnear
go test -count=1 ./cache/redisvalue ./cache/redisnear
go version
go list -m github.com/redis/go-redis/v9
git rev-parse HEAD
```

Expected: PASS. Capture the configured image tag, engine image identity, Redis
version/build/os/arch, Go version, module version, commit, test date, and every
observed result. If evidence differs from the expected rejection path, record
the actual result instead of weakening tests.

- [ ] **Step 2: Write the note with no placeholders**

Required sections:

- Executive Decision
- Environment Ledger with concrete captured values
- Result Matrix for negotiation, key/global payload, idle stale L1, affinity,
  disconnect, replacement tracking, unregister, shutdown, repetition, and race
- Pub/Sub Comparison
- Provider And ACL Assumptions
- Performance Boundary
- Follow-up Rule
- Source Links

If the planned evidence is reproduced, conclude: explicit command drain is
proven; autonomous coherent pooled near-cache is rejected on go-redis v9.20.0;
`redisnear.NewPubSub` remains production; a dedicated pump or future autonomous
push API requires a separate Type A issue.

Do not publish latency, throughput, CPU, memory, heartbeat cadence, or provider
ranking. State only that one tracking owner maintains one dedicated socket and
each explicit drain adds one Redis command; #560 owns measurement.

- [ ] **Step 3: Update both indexes**

Add to both research tables:

```markdown
| `0.19.0` | [Issue #536 RESP3 CLIENT TRACKING spike](2026-07-18-issue-536-resp3-client-tracking-spike.md) |
```

Korean may localize the label but must preserve milestone and target.

- [ ] **Step 4: Validate and commit docs**

```bash
git diff --check
rg -n '0\.19\.0.*issue-536-resp3-client-tracking-spike' docs/research/README.md docs/research/README.ko.md
rg -n 'T''BD|T''ODO' docs/research/2026-07-18-issue-536-resp3-client-tracking-spike.md
git add docs/research
git commit -m "Record why RESP3 tracking is not yet coherent" \
  -m "Constraint: Frame delivery is weaker than autonomous invalidation" \
  -m "Rejected: NewTracking after one PING | it overstates readiness" \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Directive: Revisit only with autonomous push or approved connection ownership" \
  -m "Tested: repeated Testcontainers matrix, race detector, and index parity"
```

Expected: no placeholders and one matching link in each locale.

### Task 7: Run Final Gates and Prepare the PR

**Files:**
- Modify only listed files if verification exposes a defect.

- [ ] **Step 1: Run focused and repository validation**

```bash
gofmt -w cache/redisnear/resp3_tracking_spike_test.go
go test -p 1 -count=1 -timeout=5m ./cache/redisnear -run '^TestRESP3TrackingSpike' -v
go test -p 1 -count=3 -timeout=15m ./cache/redisnear -run '^TestRESP3TrackingSpike(DeliversInvalidationOnlyWhenTrackedConnectionReads|RequiresReadAndTrackingOnSameConnection|MapsGlobalInvalidationToClearLocal|ReconnectRequiresReenableAndLocalFlush|UnregisterIsNotAQuiescenceBarrier|ShutdownOrdersQuiescenceBeforeUnregister)$'
go test -race -count=1 ./cache/redisnear
go test -count=1 ./cache/redisvalue ./cache/redisnear
make fmt-check
make tidy-check
make vet
make lint
make test
```

Expected: all PASS. Docker-backed tests run serially. If Docker is unavailable,
record the exact blocker and do not claim completion.

- [ ] **Step 2: Check scope and lesson gate**

```bash
git diff --check origin/develop...HEAD
git diff --name-only origin/develop...HEAD
git status --short --branch
```

Expected: one test file, one research note, two indexes, and planning/review
artifacts only. Lesson gate:
`N/A — the durable reusable learning is the issue-specific research note; a
separate lesson file adds no distinct Type B guidance.`

- [ ] **Step 3: Run Step 6-R and Step 7-R**

Use six independent perspectives plus main-session integration at the exact PR
head. Both gates require `P0=0 P1=0`. Repair blockers, rerun affected tests, and
refresh exact-head evidence after every commit. Record a timed-out lane with
main integration fallback as required by `AGENTS.md`.

- [ ] **Step 4: Push, PR, and merge gates**

Push and open a PR only under the approved issue workflow. Report exact head,
CI, reviews/threads, Step 6-R/7-R, and DoD before requesting fresh merge
approval. Never auto-merge. After approved merge, sync `develop`, run focused
post-merge tests, and remove the merged branch/worktree unless the user asks to
keep them.
