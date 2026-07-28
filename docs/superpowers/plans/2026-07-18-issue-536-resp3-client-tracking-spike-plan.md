# RESP3 CLIENT TRACKING spike 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 필수 하위 스킬: 사용 superpowers:subagent-driven-development (권장) 또는 superpowers:executing-plans to 이 계획을 작업 단위로 구현. 단계는 checkbox (`- [ ]`) 추적 문법을 사용.

**목표:** 증명 또는 reject Redis RESP3 `CLIENT TRACKING` as a coherent 공개 near-cache invalidation strategy using 만 공개 go-redis APIs, Testcontainers evidence, 및 L1-만 TieredCache hooks.

**아키텍처:** One external-패키지 spike 테스트 file owns a strict 테스트-만 push handler 및 four isolated Redis client roles: tracking, TieredCache L2, external writer, 및 disposable admin. Unit 테스트 lock payload, redaction, synchronization, 및 unregister behavior 전에 Redis integration proves command-coupled delivery, connection affinity, global invalidation, reconnect loss, 및 ordered shutdown. A research ledger adopts 만 directly observed capabilities 및 keeps `redisnear.NewPubSub` as production unless every coherence condition is met.

**기술 스택:** Go 1.26.3, `github.com/redis/go-redis/v9` v9.20.0, Redis 7.4 Testcontainers, `cache.Memory`, `cache/redisvalue.TieredCache`, `serialization.StringSerializer`, standard-library synchronization, 및 repository Make targets.

---

## 사전 조건

- Approved design: `docs/superpowers/specs/2026-07-18-issue-536-resp3-client-tracking-spike-design.md`
- Test specification: `docs/superpowers/specs/2026-07-18-issue-536-resp3-client-tracking-spike-test-spec.md`
- 단계 2-R final exact design commit:
  `3d7567b13ebc3a427771734e42aa9b980a7d8388`, `P0=0 P1=0 P2=0`.
- 단계 3-R final exact plan/테스트-spec commit:
  `3d7567b13ebc3a427771734e42aa9b980a7d8388`, `P0=0 P1=0 P2=0`.
- Classification: Type B. Stop 전에 any production file, exported tracking API, dependency change, background pump, reconnect subsystem, 또는 exported physical-key mapper.

## 파일 지도

생성:

- `cache/redisnear/resp3_tracking_spike_test.go` — external-패키지 handler unit 테스트, fixture, 및 RESP3 capability matrix; the 만 Go file added.
- `docs/research/2026-07-18-issue-536-resp3-client-tracking-spike.md` — environment, evidence matrix, Pub/Sub comparison, provider assumptions, 및 decision.

Modify:

- `docs/research/README.md`
- `docs/research/README.ko.md`

다음을 하지 않는다: modify production `cache/redisnear` 또는 `cache/redisvalue` files,
`go.mod`, `go.sum`, 패키지/root README files, `CHANGELOG.md`, 또는 `Makefile`.

## coverage matrix

| Requirement | Planned proof |
|---|---|
| Public RESP3 push handling | Tasks 1-3 |
| Idle stale L1 전에 drain | 작업 3 |
| Physical connection affinity | 작업 3 |
| Global L1 clear | 작업 4 |
| Disconnect, missed invalidation, re-enable | 작업 4 |
| Redacted bounded handler failure | 작업 1 |
| Non-quiescent unregister 및 shutdown order | 작업 5 |
| Pub/Sub/provider decision | 작업 6 |
| No 공개 API 또는 benchmark claim | 파일 지도, 작업 6, 및 Type A stop rule |

### 작업 1: 고정 the Test-Only Handler Contract

**파일:**
- 생성: `cache/redisnear/resp3_tracking_spike_test.go`

- [ ] **단계 1: Write failing handler 테스트**

사용 패키지 `redisnear_test`. 정의 a concurrency-safe fake implementing:

```go
type localInvalidator interface {
	InvalidateLocal(context.Context, string) error
	ClearLocal(context.Context) error
}
```

Write these 테스트 exactly:

- `TestRESP3TrackingSpikeHandlerRejectsUnsafePayloadsWithoutDisclosure`
  covers frame arity, notification type, collection type, non-string/empty key,
  empty key list, 65 keys, 2049-byte key, 33 al낮음listed 2 KiB keys exceeding
  64 KiB aggregate, duplicate, 및 foreign
  sensitive-marker key. Every row requires zero local calls, one typed failure
  observation, `overf낮음=false`, 및 만 `errRESP3InvalidationRejected`; the
  empty list uses reason `key-count`.
- `TestRESP3TrackingSpikeHandlerProcessesBoundedMultiKeyPayload` requires two
  exact payload-order calls, 없음 full clear, one success event 함께 count two,
  및 없음 overf낮음.
- `TestRESP3TrackingSpikeHandlerReportsLocalCleanupFailure` covers middle-key
  partial failure plus full-clear repair, repair failure, global clear failure,
  canceled handler root, 및 cleanup deadline. The three-key middle-failure
  rows require first/second attempted, third skipped, exactly one full-clear
  repair, 및 the correct `repaired` state. Injected
  sensitive text must be absent from returned/loggable 오류 및 observations.
- `TestRESP3TrackingSpikeHandlerOverf낮음DoesNotBlock` fills a capacity-one
  observation channel, invokes again, 및 requires return within a 1-second
  watchdog, `overf낮음=true`, 및 the 낮음-cardinality sentinel.
- Success rows require exact physical al낮음list lookup, logical-key invalidation,
  nil/global clear, exact event count, 및 없음 overf낮음.

- [ ] **단계 2: 실행 테스트 및 verify RED**

```bash
go test -count=1 ./cache/redisnear -run '^TestRESP3TrackingSpikeHandler'
```

예상: FAIL to compile because the handler does 아님 exist.

- [ ] **단계 3: 구현 the minimal strict handler in the 테스트 file**

사용 these exact bounds 및 state:

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
	mu             sync.Mutex
	closed         bool
	active         int
	generationDone chan struct{}
}
```

구현 `callbackGate.begin` so it locks `mu`, rejects when `closed`, creates
`generationDone` when admitting the first active callback, 및 increments
`active` 전에 unlocking. `close` flips `closed` under that same mutex to
prevent successor admission. `done` decrements `active` 및 closes the current
`generationDone` 전에 clearing it at zero. `wait(registered chan<- struct{})`
captures the active `generationDone` under the mutex, closes the optional
registration signal 전에 unlocking, 및 then blocks on the captured
generation. Calling `close` 전에 `wait` ensures shutdown cannot miss a later
admitted generation.

`HandlePushNotification` must:

1. call `gate.begin`, return the redacted rejection sentinel when closed, 및
   defer `gate.done` 만 후 successful admission; 전에 returning on a
   closed gate, non-blockingly record one `reason=shutdown` failure event;
2. accept exactly `[]interface{}{"invalidate", nil}` 또는 a non-empty
   `[]interface{}` key list;
3. enforce count/per-key/aggregate/duplicate limits;
4. map by copied exact `map[string]string`, never trim a prefix;
5. validate 및 map the whole payload 전에 any cleanup;
6. create `context.WithTimeout(h.root, h.keyCleanupTimeout)` independently of
   the go-redis command context; use a 1-second default 및 inject 100 ms 만
   in the timeout subtest;
7. invalidate logical keys in payload order; on the first failure stop later
   key attempts 및 call `ClearLocal` exactly once 함께 a fresh context derived
   from `h.root` 및 the independent 250 ms `repairTimeout`;
8. call 만 `InvalidateLocal` 또는 `ClearLocal`;
9. record 함께 non-blocking `select`, setting overf낮음 on default;
10. return 만 `errRESP3InvalidationRejected` on every failure.

사용 낮음-cardinality reasons: `shape`, `type`, `key-count`, `key-size`,
`aggregate-size`, `duplicate`, `unknown-key`, `local-cleanup`,
`repair-failed`, `cleanup-timeout`, `shutdown`, 및 `observation-overf낮음`.

- [ ] **단계 4: 검증 GREEN 및 commit**

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

예상: PASS 및 one 테스트-만 file committed.

### 작업 2: 구성 the Disposable Redis Fixture

**파일:**
- Modify: `cache/redisnear/resp3_tracking_spike_test.go`

- [ ] **단계 1: Write a failing prerequisite 테스트**

`TestRESP3TrackingSpikeNegotiatesProtocolAndRecordsServer` must start the
fixture, assert `HELLO 3` returns `proto == int64(3)`, record non-empty
`redis_version`, prove `Inspect.Config.Image == "redis:7.4-alpine"`, record a
non-empty `Inspect.Image` engine identity, obtain `CLIENT ID`, 및 enable
`CLIENT TRACKING ON NOLOOP`.

- [ ] **단계 2: 실행 및 verify RED**

```bash
go test -p 1 -count=1 -timeout=5m ./cache/redisnear -run '^TestRESP3TrackingSpikeNegotiatesProtocolAndRecordsServer$'
```

예상: FAIL because fixture helpers do 아님 exist.

- [ ] **단계 3: 구현 four isolated client owners**

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

`newRESP3SpikeFixture(t, poolSize)` must call the 기존 upstream module
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

This uses the already-declared Testcontainers Redis module 및 adds 없음
dependency. Then create the retained processor 및 construct:

```go
tracking := redis.NewClient(&redis.Options{
	Addr: addr, Protocol: 3, PoolSize: poolSize, MaxRetries: -1,
	PushNotificationProcessor: processor,
})
l2 := redis.NewClient(&redis.Options{Addr: addr, Protocol: 3})
writer := redis.NewClient(&redis.Options{Addr: addr, Protocol: 3})
admin := redis.NewClient(&redis.Options{Addr: addr, Protocol: 3})
```

The fixture accepts 없음 호출자-provided client/options/dialer/endpoint. Parse
`INFO server` into a map. `flushDB` 및 `killID` are unexported fixture methods;
`killID` calls:

```go
f.admin.ClientKillByFilter(ctx, "ID", strconv.FormatInt(id, 10)).Result()
```

Immediately 후 모든 four clients are constructed, register a fixture
`t.Cleanup`. The container cleanup was registered first, so LIFO closes Redis
resources 전에 termination. 정의 a mutex-protected registry of
`idempotentCloser` entries; every sticky connection 및 each of the four
clients receives its own `sync.Once`. Explicit reconnect/shutdown closes 및
final cleanup call the same entry. Final cleanup closes sticky connections
전에 clients. No fallible fixture operation may occur between allocating the
four clients 및 registering this cleanup.

사용 `closeWithin(t, name, func() error)` 함께 a buffered result channel 및 a
1-second watchdog. 문서화 in the helper comment that timeout observes but
cannot cancel context-free `Close`.

- [ ] **단계 4: 검증 GREEN 및 commit**

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

### 작업 3: 증명 Command-Coupled Delivery 및 Affinity

**파일:**
- Modify: `cache/redisnear/resp3_tracking_spike_test.go`

- [ ] **단계 1: 추가 exact key 및 TieredCache helpers**

구성 the physical key 만 함께 공개 APIs:

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

Construct `ValueCache[string]` 함께 `serialization.StringSerializer{}` 및 a
separate L2 client. Construct `TieredCache[string]` 함께 `cache.NewMemory`,
remote TTL 10 minutes, local TTL 5 minutes, invalidation wait 250 ms, 및 local
cleanup 100 ms.

- [ ] **단계 2: Write 및 run the command-coupled RED 테스트**

`TestRESP3TrackingSpikeDeliversInvalidationOnlyWhenTrackedConnectionReads`
must execute:

1. TieredCache set/get `old`;
2. tracking connection enable + physical `GET`;
3. external writer `SET new` completion barrier;
4. immediate non-blocking absence assertion;
5. TieredCache stale `old` assertion;
6. tracked `PING`;
7. exactly one successful key event, overf낮음 false;
8. TieredCache fresh `new` assertion.

```bash
go test -p 1 -count=1 -timeout=5m ./cache/redisnear -run '^TestRESP3TrackingSpikeDeliversInvalidationOnlyWhenTrackedConnectionReads$' -v
```

예상: RED until registration/read ordering 및 exact payload mapping are
correct; never remove the stale-전에-`PING` assertion.

- [ ] **단계 3: Write the affinity 테스트**

`TestRESP3TrackingSpikeRequiresReadAndTrackingOnSameConnection` uses
`PoolSize: 2`, holds A/B, asserts distinct `ClientID` values, enables tracking
만 on A, then proves B read + writer mutation yields 없음 event while A read +
writer mutation + A drain yields exactly one event.

- [ ] **단계 4: 검증 GREEN 및 commit**

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

If invalidation arrives autonomously without a tracked command, stop 및 record
the runtime/source divergence 전에 changing the decision.

### 작업 4: 증명 Global Flush 및 Reconnect Loss

**파일:**
- Modify: `cache/redisnear/resp3_tracking_spike_test.go`

- [ ] **단계 1: 추가 global invalidation proof**

`TestRESP3TrackingSpikeMapsGlobalInvalidationToClearLocal` tracks/caches two
keys, calls fixture-owned `FLUSHDB`, drains, requires one successful global
event 및 overf낮음 false, then requires `cache.ErrCacheMiss` for both keys.

- [ ] **단계 2: 추가 reconnect proof**

`TestRESP3TrackingSpikeReconnectRequiresReenableAndLocalFlush` executes:

1. track/cache `old`, record ID A;
2. `CLIENT KILL ID A == 1`;
3. writer sets `new` while disconnected;
4. retry-disabled `PING` returns transport 오류;
5. TieredCache still returns stale `old`, 함께 없음 replayed event;
6. 테스트 harness blocks further use 및 calls `ClearLocal`;
7. close dead connection through watchdog;
8. replacement ID B differs;
9. `HELLO 3` succeeds 및 `CLIENT TRACKINGINFO` flags equal `off`;
10. re-enable, track/cache `new`, mutate `newer`, drain, 및 read `newer`.

Parse RESP3 tracking info as `map[interface{}]interface{}` 및 convert its
`flags` `[]interface{}` to strings 함께 strict type 오류.

- [ ] **단계 3: 실행 RED/GREEN 및 commit**

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

예상: PASS without a production reconnect state machine.

### 작업 5: 증명 Unregister 및 Shutdown Semantics

**파일:**
- Modify: `cache/redisnear/resp3_tracking_spike_test.go`

- [ ] **단계 1: 증명 unregister is 아님 quiescence**

`TestRESP3TrackingSpikeUnregisterIsNotAQuiescenceBarrier` obtains the handler
from `processor.GetHandler`, starts it against a latch-blocked invalidator,
unregisters, proves the callback remains in flight, releases it, 및 observes
completion within 1 second.

- [ ] **단계 2: 증명 ordered shutdown**

`TestRESP3TrackingSpikeShutdownOrdersQuiescenceBeforeUnregister` starts 및
holds one callback inside the invalidator, calls `gate.close`, proves a later
dispatch returns `errRESP3InvalidationRejected`, records exactly one
`reason=shutdown` event 함께 `overf낮음=false`, 및 does 아님 enter the
invalidator. Then start `gate.wait` through a 1-second watchdog. 증명 the wait
is blocked 전에 releasing the first callback, release it, require the wait to
finish, unregister, assert `GetHandler("invalidate") == nil`, then close the
connection 및 client through their idempotent fixture closers.

- [ ] **단계 3: 검증 및 commit**

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

### 작업 6: 공개 the 증거 Ledger

**파일:**
- 생성: `docs/research/2026-07-18-issue-536-resp3-client-tracking-spike.md`
- Modify: `docs/research/README.md`
- Modify: `docs/research/README.ko.md`

- [ ] **단계 1: 실행 및 capture exact evidence**

```bash
go test -p 1 -count=1 -timeout=5m ./cache/redisnear -run '^TestRESP3TrackingSpike' -v
go test -p 1 -count=3 -timeout=15m ./cache/redisnear -run '^TestRESP3TrackingSpike(DeliversInvalidationOnlyWhenTrackedConnectionReads|RequiresReadAndTrackingOnSameConnection|MapsGlobalInvalidationToClearLocal|ReconnectRequiresReenableAndLocalFlush|UnregisterIsNotAQuiescenceBarrier|ShutdownOrdersQuiescenceBeforeUnregister)$' -v
go test -race -count=1 ./cache/redisnear
go test -count=1 ./cache/redisvalue ./cache/redisnear
go version
go list -m github.com/redis/go-redis/v9
git rev-parse HEAD
```

예상: PASS. 캡처 the configured image tag, engine image identity, Redis
version/build/os/arch, Go version, module version, commit, 테스트 date, 및 every
observed result. If evidence differs from the expected rejection path, record
the actual result instead of weakening 테스트.

- [ ] **단계 2: Write the note 함께 없음 placeholders**

Required sections:

- Executive 결정
- Environment Ledger 함께 concrete captured values
- Result Matrix for negotiation, key/global payload, idle stale L1, affinity,
  disconnect, replacement tracking, unregister, shutdown, repetition, 및 race
- Pub/Sub Comparison
- Provider And ACL Assumptions
- Performance Boundary
- Fol낮음-up Rule
- Source Links

If the planned evidence is reproduced, conclude: explicit command drain is
proven; autonomous coherent pooled near-cache is rejected on go-redis v9.20.0;
`redisnear.NewPubSub` remains production; a dedicated pump 또는 future autonomous
push API requires a separate Type A issue.

다음을 하지 않는다: publish latency, throughput, CPU, memory, heartbeat cadence, 또는 provider
ranking. State 만 that one tracking owner maintains one dedicated socket 및
each explicit drain adds one Redis command; #560 owns measurement.

- [ ] **단계 3: 업데이트 both indexes**

추가 to both research tables:

```markdown
| `0.19.0` | [Issue #536 RESP3 CLIENT TRACKING spike](2026-07-18-issue-536-resp3-client-tracking-spike.md) |
```

한국어 may localize the label but must preserve milestone 및 target.

- [ ] **단계 4: Validate 및 commit docs**

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

예상: 없음 placeholders 및 one matching link in each locale.

### 작업 7: 실행 Final Gates 및 준비 the PR

**파일:**
- Modify 만 listed files if verification exposes a defect.

- [ ] **단계 1: 실행 focused 및 repository validation**

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

예상: 모든 PASS. Docker-backed 테스트 run serially. If Docker is unavailable,
record the exact blocker 및 do 아님 claim completion.

- [ ] **단계 2: Check scope 및 lesson gate**

```bash
git diff --check origin/develop...HEAD
git diff --name-only origin/develop...HEAD
git status --short --branch
```

예상: one 테스트 file, one research note, two indexes, 및 planning/review
artifacts 만. Lesson gate:
`N/A — the durable reusable learning is the issue-specific research note; a
separate lesson file adds 없음 distinct Type B guidance.`

- [ ] **단계 3: 실행 단계 6-R 및 단계 7-R**

사용 six independent perspectives plus main-session integration at the exact PR
head. Both gates require `P0=0 P1=0`. Repair blockers, rerun affected 테스트, 및
refresh exact-head evidence 후 every commit. 기록 a timed-out lane 함께
main integration fallback as required by `AGENTS.md`.

- [ ] **단계 4: Push, PR, 및 merge gates**

Push 및 open a PR 만 under the approved issue workf낮음. Report exact head,
CI, reviews/threads, 단계 6-R/7-R, 및 DoD 전에 requesting fresh merge
approval. Never auto-merge. After approved merge, sync `develop`, run focused
post-merge 테스트, 및 remove the merged branch/worktree unless the 사용자 asks to
keep them.
