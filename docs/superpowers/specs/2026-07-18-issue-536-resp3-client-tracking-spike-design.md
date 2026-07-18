# Issue #536 RESP3 CLIENT TRACKING Spike Design

Status: Step 2-R and Step 3-R converged at `3d7567b`
Issue: [#536](https://github.com/bluetape4k/bluetape-go/issues/536)
Predecessor: [#110](https://github.com/bluetape4k/bluetape-go/issues/110)
Tiered cache seam: [#535](https://github.com/bluetape4k/bluetape-go/issues/535)
Target package: `cache/redisnear` tests only

## Problem

`cache/redisnear.NewPubSub`는 전용 subscription receive loop로 invalidation을
수신하고, receive error가 나면 L1 전체를 clear하며, terminal failure 뒤에는 새
instance를 만들도록 요구한다. 이 방식은 bluetape-go protocol을 publish하는 writer만
관찰한다. Redis 밖의 writer가 같은 key를 변경하면 L1을 invalidate할 수 없다.

Redis RESP3 `CLIENT TRACKING`은 Redis가 실제 read connection별 key 접근을 추적하고
외부 mutation을 push invalidation으로 전달하므로 이 빈틈을 메울 후보이다. 그러나
go-redis/v9 v9.20.0의 공개 push handler는 별도 background reader가 아니다. 해당
physical connection이 다음 command를 처리하면서 socket을 읽을 때 pending push를
동기적으로 drain한다.

따라서 RESP3 frame을 parse할 수 있다는 사실만으로 coherent near-cache를 주장할 수
없다. L1 hit는 Redis command를 실행하지 않으므로 invalidation이 socket에 남은 채
stale L1 value가 무기한 제공될 수 있다. Pooled `*redis.Client`는 tracking을 활성화한
connection, cacheable read를 수행한 connection, push를 drain하는 connection의
affinity도 공개 API로 보장하지 않는다.

#536은 production API를 구현하지 않는다. public go-redis surface로 가능한 정확한
경계를 Testcontainers에서 재현하고, production strategy를 채택하거나 기각할 근거를
남기는 Type B spike다.

## Goals

1. Redis 7.4와 go-redis/v9 v9.20.0에서 RESP3 tracking invalidation payload를 공개
   API만으로 관찰할 수 있음을 증명한다.
2. Push 처리 시점이 tracked connection의 command/read path에 종속됨을 검증한다.
3. Tracking과 cacheable read가 동일 physical connection에 있어야 하는 affinity를
   검증한다.
4. Disconnect window에서 missed invalidation이 발생하며, reconnect 전에 L1 전체
   clear와 tracking 재활성화가 필요함을 검증한다.
5. Handler 등록과 해제를 caller가 명시적으로 소유해야 deterministic shutdown이
   가능함을 검증한다.
6. `TieredCache.InvalidateLocal`과 `TieredCache.ClearLocal`만 사용해 RESP3 callback이
   L2를 다시 mutate하지 않는 경계를 확인한다.
7. 기존 Pub/Sub 방식과 failure behavior를 비교하고 provider/proxy compatibility
   assumptions를 기록한다.
8. 결과가 positive여도 production 공개 API는 별도 Type A issue와 승인된 설계 없이는
   추가하지 않는다.

## Non-goals

- `redisnear.NewTracking`, `TrackingOptions`, strategy enum 또는 다른 공개 API 추가
- 기존 `redisnear.NewPubSub` 교체 또는 의미 변경
- `redisvalue.ValueOptions.Client`를 pinned connection abstraction으로 변경
- production background pump, reconnect state machine, queue 또는 backpressure 구현
- package-global Redis connection, processor, cache registry 또는 mutable defaults 추가
- go-redis upgrade/replacement 또는 새 dependency 추가
- RESP3 wire parser, raw socket reader 또는 Redis proxy 구현
- Redis `REDIRECT`를 production strategy로 채택
- benchmark 또는 provider 성능 순위 작성; issue #560이 benchmark matrix를 소유한다
- 모든 managed Redis provider와 proxy를 실제 통합 테스트하는 것

## Current Evidence

### Repository seams

- `cache/redisnear.NewPubSub`는 전용 `PubSub.ReceiveMessage` loop를 소유하며 receive
  error에서 L1을 clear한다. Close는 bounded하고 idempotent하다.
- `cache/redisnear/failure_injection_test.go`는 Redis outage, local clear, recreate를
  Testcontainers로 검증한다.
- `cache/redisvalue.TieredCache.InvalidateLocal`은 해당 logical key의 L1만 제거하고
  Redis를 호출하지 않는다.
- `cache/redisvalue.TieredCache.ClearLocal`은 전체 L1을 clear하며 blocked local state를
  repair할 수 있는 유일한 명시적 operation이다.
- `cache/redisvalue.ValueOptions.Client`는 concrete `*redis.Client`다. 현재 production
  L2 command path에는 pinned `*redis.Conn`을 주입할 수 없다.
- Redis physical value key는 package-private builder가 생성한다. Spike는 public
  `redis.KeyBuilder`로 동일 구조를 test fixture 안에서 구성하되, production mapper를
  export하지 않는다.

### go-redis/v9 v9.20.0

- `redis.Options.Protocol = 3`은 RESP3 negotiation을 요청하고 RESP3 push processor를
  활성화한다. Option만 확인하지 않고 `HELLO 3` 결과를 test evidence로 남긴다.
- `redis.Options.MaxRetries`의 zero value는 세 번 재시도한다. Connection-loss proof는
  `MaxRetries: -1`로 command retry를 끄고 첫 transport failure를 관찰해야 한다.
- `redis.NewPushNotificationProcessor`와
  `Client.RegisterPushNotificationHandler`는 공개 API다.
- `push.NotificationProcessor.UnregisterHandler`는 공개되지만 `Client`에는 unregister
  method가 없다. Deterministic shutdown에는 생성 시 주입한 processor를 보관해야 한다.
- `CLIENT TRACKING` typed command는 없다. Spike는
  `Conn.Do(ctx, "CLIENT", "TRACKING", "ON", "NOLOOP")`를 사용한다.
- `Client.Conn()`은 한 physical connection을 계속 사용하는 `*redis.Conn`을 반환하지만
  goroutine-safe하지 않다. 모든 command는 한 test goroutine에서 직렬화한다.
- Pending push는 ordinary command 전후의 socket read 경로에서 처리된다. 별도
  background receive loop가 없다.
- Handler는 command processing goroutine에서 동기 실행되고 handler error는 go-redis
  global logger에 `%v`로 기록된 뒤 command caller에 반환되지 않는다. Callback은
  bounded L1-only operation과 non-blocking 관측값 기록만 수행해야 한다. 반환 error는
  raw payload, physical/logical key, endpoint, credential 또는 provider error를 포함하지
  않는 low-cardinality sentinel이어야 한다.

### Redis contract

- Tracking state는 connection-local이다. Tracking을 켠 connection과 key를 읽은
  connection이 같아야 default mode가 그 key를 추적한다.
- Invalidation payload는 `['invalidate', [keys...]]` 형태다. Global flush는 key list가
  null인 invalidate payload로 나타날 수 있다.
- Connection loss 중 invalidation은 복구되지 않는다. Redis guidance는 invalidation
  connection을 heartbeat하고 연결 손실 시 전체 local cache를 flush하도록 요구한다.
- Redis Cloud/Software client-side caching은 지원 version과 RESP3 조건이 있고,
  two-connection `REDIRECT`는 지원하지 않는다. Proxy는 RESP3 `HELLO`, tracking command,
  out-of-band push 보존, disconnect detection을 각각 검증해야 한다.

## Considered Approaches

### Approach 1: strict public-only prove-or-reject spike

전용 pinned connection에서 push를 command로 drain하는 positive proof와 pooled/idle
경로의 stale behavior를 함께 검증한다. Reconnect, L1 clear, handler ownership까지
포함해 production 채택 조건을 명시적으로 판정한다.

이 접근을 채택한다. Issue의 목표인 “prove or reject”를 충족하고, frame 수신 성공을
production readiness로 오해하지 않는다.

### Approach 2: dedicated connection positive proof only

Pinned connection에서 tracking을 켜고 mutation 뒤 `PING`을 실행하면 push가 전달되는지만
검증한다. 구현은 작지만 L1-only hit, pool affinity, disconnect window를 숨기므로
near-cache 지원 결론으로 사용할 수 없다. 제외한다.

### Approach 3: raw listener 또는 `REDIRECT` subsystem

별도 connection에서 push를 지속 수신하고 data connections를 redirect하면 즉시성은
개선될 수 있다. 하지만 raw protocol/lifecycle ownership, reconnect client ID 갱신,
모든 pool connection 재설정, queue/backpressure가 필요하다. Redis Cloud/Software의
`REDIRECT` 제약도 있다. 이는 별도 Type A subsystem이므로 #536에서 제외한다.

## Chosen Spike Architecture

```text
                         test-owned Redis clients
                        +-------------------------+
                        |                         |
                        v                         v
                 tracked *redis.Conn       external writer/admin
                 RESP3 + TRACKING ON        SET / FLUSH / CLIENT KILL
                        |
                        | command read drains pending push
                        v
             retained PushNotificationProcessor
                        |
                        v
               test-only invalidate handler
                 | key payload      | null payload
                 v                  v
       TieredCache.InvalidateLocal  TieredCache.ClearLocal
                 |                  |
                 +--------> caller-owned L1

       tracked command transport failure
                 |
                 v
       test harness blocks tracked-L1 use
                 |
                 v
       TieredCache.ClearLocal before replacement tracking

                 ValueCache Redis L2 is never mutated by the handler
```

The spike separates four client owners so one sticky connection cannot starve
an unrelated command path:

1. A tracking client configured with protocol 3, `PoolSize: 1`,
   `MaxRetries: -1`, and a retained push processor. Its one sticky `*redis.Conn`
   owns serialized tracking/read/drain commands.
2. A separate `TieredCache`/`ValueCache` L2 client. Tiered reads never compete
   for the tracking client's only pool turn.
3. A separate external writer client that performs ordinary `SET` mutations.
4. A disposable-test admin client that performs only fixture-scoped `FLUSHDB`
   and `CLIENT KILL ID` operations.

The container cleanup is registered first. After all four clients are created,
the fixture immediately registers one idempotent cleanup that closes every
owned sticky connection before the tracking, L2, writer, and admin clients.
Because `testing.T.Cleanup` is LIFO, all Redis resources close before container
termination. Explicit reconnect and shutdown closes use the same per-resource
`sync.Once` wrappers, so final cleanup can safely re-enter them. The owned
closer registry is mutex-protected for race tests.

The affinity case uses a dedicated tracking client with `PoolSize: 2`, obtains
sticky connections A and B, and asserts their `CLIENT ID` values are distinct.
It does not borrow the L2 or writer client.

The handler is test-only. It accepts exactly two frame elements: the string
`invalidate` and either `nil` for global invalidation or a `[]interface{}` of at
between 1 and 64 string keys. Each key is at most 2 KiB and the aggregate key
bytes are at most 64 KiB. The fixture precomputes an exact `physical key -> logical key`
allowlist with public `redis.KeyBuilder`; it never reverse-maps by trimming a
prefix. Unknown, duplicate, oversized, or malformed entries fail the spike.

Every accepted callback creates an independent bounded key-cleanup context from
the test-owned handler lifetime rather than reusing the drain command context.
The default handler key-cleanup deadline is 1 second; the timeout subtest
injects 100 ms and uses a watchdog of at least 1 second so the assertion cannot
race the deadline it is proving. The spike uses
`TieredConfig{InvalidationWaitTimeout: 250 * time.Millisecond,
LocalCleanupTimeout: 100 * time.Millisecond}`. It calls only
`InvalidateLocal` or `ClearLocal`.

The handler validates the complete key list before performing cleanup, then
invalidates logical keys in payload order. A per-key failure stops later key
attempts and triggers exactly one full `ClearLocal` repair with a separate
250 ms context derived from the handler lifetime, not from the expired or
failed key-cleanup context. The callback still returns
`errRESP3InvalidationRejected`; its redacted observation records whether the
full-clear repair succeeded. This prevents a partially invalidated multi-key
payload from leaving an apparently successful but stale L1.

Callback admission is owned by a test-only dispatch gate. Under one mutex,
`begin` rejects work after closure or increments the in-flight `WaitGroup`;
`close` flips the admission flag under the same mutex before any `wait`. This
makes `WaitGroup.Add` impossible once shutdown waiting begins and avoids the
unsafe late-`Add` lifecycle of a raw `WaitGroup`.

A callback rejected after gate closure records exactly one non-blocking,
redacted failure observation with `reason=shutdown` before returning
`errRESP3InvalidationRejected`. The shutdown test requires the sentinel,
`overflow=false`, exact event count one, and zero local invalidator calls.

Handler results are written with a non-blocking `select` to a bounded channel.
An atomic overflow flag records any dropped observation. Tests require the
exact event count, `overflow=false`, and an explicit success/failure result for
every callback. Malformed data and local cleanup failures return only
`errRESP3InvalidationRejected`; the synchronized observation retains a typed,
redacted reason without retaining raw payloads or provider messages.

## RESP3 Negotiation And Tracking Setup

Each test must prove its prerequisites before asserting invalidation behavior:

1. Start Redis 7.4 through the already-declared upstream Testcontainers Redis
   module using the repository-pinned `redis:7.4-alpine` image. Inspect the
   actual container and record both `Inspect.Config.Image` and the engine image
   identity from `Inspect.Image`, plus `INFO server` version/build fields, in
   the result ledger. Repeating a source-level tag constant is not
   reproducibility evidence.
2. Create a tracking client with `Protocol: 3`, `PoolSize: 1`,
   `MaxRetries: -1`, and an injected `redis.NewPushNotificationProcessor()`.
3. Register the `invalidate` handler with `protected=false`.
4. Acquire one sticky connection and issue `HELLO 3`; assert the response reports
   protocol 3.
5. Run `CLIENT TRACKING ON NOLOOP` on that connection.
6. Read the physical Redis value key through that same connection so Redis tracks
   it.

`PoolSize: 1` reduces scheduling noise but is not treated as a production
affinity guarantee. It belongs only to the single-connection tracking client;
the L2, writer, and admin use separate clients. The affinity test explicitly
uses `PoolSize: 2`, holds two sticky connections, and checks distinct client IDs.

## Test Matrix

### Command-coupled delivery and L1-only invalidation

`TestRESP3TrackingSpikeDeliversInvalidationOnlyWhenTrackedConnectionReads`

- Populate Redis L2 and `TieredCache` L1 with value `old`.
- Read the physical key through the tracked connection.
- Mutate that Redis key to `new` from the external writer.
- Treat successful external `SET` completion as the server-side mutation
  barrier. Before another tracked-connection command, make an immediate
  non-blocking assertion that no handler observation exists and assert the
  tiered read still returns the stale L1 value `old`. Do not interpret this as a
  latency measurement.
- Issue `PING` on the tracked connection.
- Assert exact key invalidation payload, successful `InvalidateLocal`, and a
  subsequent tiered read returning `new` from L2.
- Assert the callback did not issue Redis mutation commands.

This is both the positive RESP3 proof and the principal production blocker:
delivery exists, but it is not autonomous.

### Connection affinity

`TestRESP3TrackingSpikeRequiresReadAndTrackingOnSameConnection`

- Hold sticky connections A and B concurrently.
- Assert `CLIENT ID` reports two distinct IDs.
- Enable tracking on A, read the key on B, mutate externally, then drain A and B.
  Assert no invalidation for that read.
- Read the key on A, mutate externally, drain A, and assert exact invalidation.

This test must not infer affinity from `PoolSize: 1` or repeated `Client.Do`
calls.

### Global flush payload

`TestRESP3TrackingSpikeMapsGlobalInvalidationToClearLocal`

- Track and cache at least two logical keys.
- Execute `FLUSHDB` from the admin client.
- Drain the tracked connection.
- Assert the null/global invalidation shape and one bounded `ClearLocal` repair.
- Assert subsequent tiered reads miss rather than serve either stale L1 value.

### Reconnect and missed-invalidation window

`TestRESP3TrackingSpikeReconnectRequiresReenableAndLocalFlush`

- Track/read a key and obtain the tracked connection client ID.
- Kill that connection from the disposable-test admin client and assert
  `CLIENT KILL ID` returns exactly one killed connection.
- Mutate the key while the tracked connection is unavailable.
- Run a bounded command on the tracked connection with retries disabled and
  assert the first transport failure proves loss detection.
- Assert the stale L1 remains populated and no missed invalidation is replayed
  even after detection.
- Mark tracked-L1 use blocked in the test harness, close the dead sticky
  connection, and call `ClearLocal`.
- Obtain a new sticky connection, assert its client ID differs, verify RESP3,
  and assert `CLIENT TRACKINGINFO` reports tracking off before re-enabling it.
- Re-read and cache the value, mutate again, drain with `PING`, and prove
  invalidation resumes.

The safe lifecycle order is fixed as: detect loss -> block use of tracked L1 ->
`ClearLocal` -> create connection -> verify RESP3 -> enable tracking -> allow
cacheable reads. `OnConnect` alone cannot close the missed-invalidation window.

### Deterministic shutdown

`TestRESP3TrackingSpikeUnregisterIsNotAQuiescenceBarrier`

- Start a direct handler invocation and hold it on a test latch after registry
  lookup.
- Unregister `invalidate` through the retained processor and prove unregister
  returns before that in-flight callback is released.
- Release and await the callback with a bounded deadline.

`TestRESP3TrackingSpikeShutdownOrdersQuiescenceBeforeUnregister`

- Start one direct callback and hold it inside the local invalidator.
- Close the synchronized dispatch gate and prove a later callback is rejected
  without entering the invalidator. Require the rejection sentinel, exactly
  one `reason=shutdown` failure observation, and `overflow=false`.
- Start a bounded gate wait and prove it remains blocked until the held
  callback is released.
- Release the held callback and require the gate wait to complete within a
  1-second watchdog.
- Unregister `invalidate`, prove later processor lookup fails, then close the
  sticky connection and client through 1-second watchdogs.
- Assert command quiescence and unregister with explicit context deadlines. Run
  context-free `Conn.Close()` and `Client.Close()` behind bounded test watchdog
  channels; a watchdog timeout observes a stuck close but cannot cancel it.
  Run the handler, unregister, and shutdown proofs under `go test -race`.

These tests document that unregister is not a callback quiescence barrier and
that direct registration on a caller-created client without processor and
in-flight ownership is insufficient for a production component.

### Handler validation, redaction, and cleanup failure

`TestRESP3TrackingSpikeHandlerRejectsUnsafePayloadsWithoutDisclosure`

- Invoke the test handler directly with wrong arity, wrong notification type,
  non-string keys, duplicate keys, an unknown key containing a sensitive
  marker, more than 64 keys, an oversized key, and aggregate bytes over 64 KiB.
- Assert one bounded failure observation per callback, `overflow=false`, and
  `errors.Is(err, errRESP3InvalidationRejected)`.
- Assert returned/loggable error text contains no raw frame, key, namespace,
  endpoint, credential, or injected provider marker.

`TestRESP3TrackingSpikeHandlerReportsLocalCleanupFailure`

- Inject a middle-key `InvalidateLocal` failure in a three-key payload. Assert
  the first and second calls occur in order, the third is not attempted, and
  exactly one `ClearLocal` repair runs under its independent repair context.
- Run the middle-key case once with successful repair and once with a
  sensitive-marker repair failure; observations distinguish `repaired=true`
  from `repaired=false` without retaining provider text.
- Inject global `ClearLocal` failure and separately force the key-cleanup
  deadline to expire using a 100 ms injected deadline and a 1-second watchdog.
- Assert no callback is reported as successful, all observation reasons are
  typed/redacted, the returned error is only the low-cardinality sentinel, and
  synchronized state is race-free.

`TestRESP3TrackingSpikeHandlerProcessesBoundedMultiKeyPayload` separately
proves that a two-key success invalidates both exact logical keys in payload
order, performs no full clear, and records one success with count two.

## Synchronization And Flake Control

- No unbounded sleeps or eventual assertions.
- Use context deadlines and observation channels. The command-coupled negative
  proof uses the completed external mutation as a barrier plus an immediate
  non-blocking absence check; it does not sleep or report a latency bound.
- A sticky `*redis.Conn` is used by one goroutine at a time.
- Handler observations use a non-blocking send plus overflow flag; a full
  buffer cannot block command processing or look like success.
- Every test owns unique namespace/key values and performs idempotent cleanup.
- Testcontainers cases run sequentially when they share Docker resources.
- Connection-kill tests use distinct `CLIENT ID` values, exact
  `CLIENT KILL ID == 1`, disabled retries, and an observed transport failure,
  not timing-based guesses about pool members.

## Failure Semantics Compared With Pub/Sub

| Dimension | Current Pub/Sub | RESP3 spike evidence target |
|---|---|---|
| Receive model | Dedicated blocking receive loop | Command-coupled socket drain |
| External Redis writer | Invisible unless it publishes bluetape protocol | Redis-native invalidation when the read connection is tracked |
| L1-only hit | Subscriber still receives messages | No command means pending push may remain unread |
| Connection affinity | Subscription connection is explicit | Tracking and read must share a physical connection |
| Disconnect | Receive error clears L1 and instance is recreated | Public client lacks equivalent per-pooled-connection loss callback |
| Reconnect | Re-subscribe in new instance | Clear L1, recreate connection, verify RESP3, re-enable tracking |
| Shutdown | NearCache owns loop and Close | Caller must retain processor to unregister handler |
| Provider contract | Pub/Sub support and app protocol | RESP3, CLIENT TRACKING, push preservation, loss detection |

## Provider And Proxy Compatibility Record

The research note will classify each target as `proved`, `documented`,
`unsupported`, or `unknown` rather than implying universal Redis compatibility.

| Target | Required conclusion |
|---|---|
| Redis OSS 7.4 Testcontainers | Execute full spike matrix |
| Redis OSS with RESP2/fallback | Reject because tracking push contract is unavailable |
| Redis Cloud/Software 7.4+ | Document provider requirement; do not claim live proof |
| Redis Cloud/Software `REDIRECT` | Document unsupported provider mode |
| Sentinel/Cluster | Unknown until a separate topology-specific spike proves per-node connection lifecycle |
| Generic proxy | Require HELLO 3, tracking, push preservation, and disconnect detection evidence |

### Identity and administrative command boundary

| Identity | Spike commands | Production permission conclusion |
|---|---|---|
| Tracked runtime | `HELLO 3`, `CLIENT TRACKING`, `CLIENT TRACKINGINFO`, `CLIENT ID`, `GET`, `PING` | Grant only required tracking/read commands; explicitly deny `FLUSHDB`, `FLUSHALL`, and `CLIENT KILL` |
| Tiered L2 runtime | Existing bounded redisvalue read/write commands | Preserve redisvalue ACL guidance; logical DB selection is not a security boundary |
| External writer | Test-owned `SET` for external mutation proof | Ordinary application mutation identity; no admin commands required |
| Disposable-test admin | `FLUSHDB`, `CLIENT KILL ID` | Test-only; never a production near-cache requirement |

Destructive admin commands may run only against the fresh endpoint returned by
the directly owned upstream Testcontainers Redis container in the current
test. The fixture constructs the admin client internally from that returned
address, accepts no caller-supplied client, options, dialer, or endpoint, and
never exports the admin client. No environment-provided, shared, staging, or
production endpoint is accepted.

The research note must record AUTH/TLS/certificate ownership as provider
requirements, state that credentials and endpoints are never written into
handler errors or observations, and distinguish documented ACL expectations
from the unauthenticated Testcontainers proof. The spike does not claim live
AUTH/TLS or managed-provider validation.

## File Scope

Expected implementation scope after written spec approval:

- `cache/redisnear/resp3_tracking_spike_test.go`
- `docs/research/2026-07-18-issue-536-resp3-client-tracking-spike.md`
- `docs/research/README.md`
- `docs/research/README.ko.md`
- implementation plan and test spec under `docs/superpowers/plans/` and
  `docs/superpowers/specs/`

No production `.go` file, dependency file, package README, root README, or
CHANGELOG change is expected.

## Decision Rule

The spike is successful when it produces deterministic, repeatable evidence and
an explicit conclusion. “Success” does not require adopting RESP3 for production.

Production adoption requires all of the following:

1. Invalidation is consumed independently of L1 misses and ordinary cache
   commands.
2. Every cacheable read has provable tracking connection affinity.
3. Every relevant connection loss immediately blocks/flushes L1 before stale
   values can be served.
4. Reconnect, tracking re-enable, handler ownership, shutdown, and callback
   failure propagation have complete public lifecycle seams.
5. Supported provider/proxy topologies preserve the contract.

If the source-pinned behavior is reproduced, criteria 1-4 fail for a normal
pooled `*redis.Client`. The expected conclusion is:

- adopt RESP3 parsing and one explicit command-coupled drain as technically
  proven;
- reject a production `redisnear.NewTracking` API on go-redis/v9 v9.20.0;
- keep `redisnear.NewPubSub` as the production strategy;
- open a separate Type A issue only if a dedicated pump/connection-owner
  subsystem or future go-redis background push API is intentionally pursued.

Periodic heartbeat/pump viability is not proven. Its cadence, maximum stale
window, disconnect latency, ticker/goroutine lifecycle, additional Redis QPS,
socket/memory cost, throughput, and provider comparison belong to issue #560 or
a separately approved Type A design. The research note may provide only the
qualitative cost statement that one tracking owner maintains one dedicated
socket and each explicit drain adds one Redis command; it must not publish
measured performance claims.

## Type A Escalation Triggers

Stop the Type B implementation and reclassify before editing if any of these is
required:

- exported tracking constructor, options, strategy, mapper, or command interface
- production goroutine, pump, queue, reconnect state machine, or owned client
- changes to `redisvalue.ValueOptions`, `ValueCache`, or `TieredCache` public API
- raw RESP3 reader, `REDIRECT` subsystem, Sentinel/Cluster lifecycle support
- go-redis version/dependency change
- five or more coupled production files

## Validation

After implementation approval, validation will run in this order:

1. Focused Testcontainers spike tests in `cache/redisnear`.
2. Repeat only the RESP3 integration matrix serially:
   `go test -p 1 -count=3 -timeout=15m ./cache/redisnear -run '^TestRESP3TrackingSpike(DeliversInvalidationOnlyWhenTrackedConnectionReads|RequiresReadAndTrackingOnSameConnection|MapsGlobalInvalidationToClearLocal|ReconnectRequiresReenableAndLocalFlush|UnregisterIsNotAQuiescenceBarrier|ShutdownOrdersQuiescenceBeforeUnregister)$'`.
   Container startup time is not performance evidence.
3. `go test -race ./cache/redisnear -count=1`.
4. `go test ./cache/redisvalue ./cache/redisnear -count=1`.
5. `make fmt-check`, `make tidy-check`, `make vet`, `make lint`, and `make test`.
6. Research note link and English/Korean index parity checks.

## Acceptance Criteria Mapping

| Issue #536 criterion | Design coverage |
|---|---|
| Prove push invalidation | Command-coupled delivery test observes exact payload and L1-only eviction |
| Prove connection affinity | Two explicit sticky connections isolate tracking/read ownership |
| Prove reconnect behavior | Kill, missed mutation, clear, recreate, re-enable, resume sequence |
| Prove local flush | Global invalidation and disconnect repair call `ClearLocal` only |
| Prove shutdown | Wait for quiescence, unregister the handler, then close the connection/client with bounded assertions |
| Compare Pub/Sub | Failure semantics table records receive, loss, recovery, and ownership differences |
| Record compatibility | Provider/proxy matrix separates proved, documented, unsupported, and unknown |
| No premature public API | File scope and Type A triggers forbid production surface changes |

## Risks And Expected Review Verdict

- `P0=0`.
- `P1-1`: command-coupled push processing permits indefinitely stale L1 hits.
- `P1-2`: ordinary pooled client usage cannot guarantee tracking/read/drain
  connection affinity.
- `P1-3`: disconnect loses tracking state and invalidations; re-enable without
  first clearing L1 leaves a stale window.
- `P1-4`: handler failures are swallowed by go-redis, so production failure
  propagation is incomplete.
- `P1-5`: caller-created client does not expose handler unregister ownership.
- `P1-6`: provider/proxy behavior is topology-specific.
- `P1-7`: unregister is not an in-flight callback quiescence barrier.
- `P1-8`: a synchronous handler needs independent cleanup deadlines,
  non-blocking observation, and redacted failure signaling.

The written spike may close the issue with these P1 capability blockers because
the issue is a prove-or-reject research gate, not a production delivery issue.
They must not be hidden or downgraded into a production-ready claim.

## Sources

- Redis client-side caching:
  https://redis.io/docs/latest/develop/clients/client-side-caching/
- Redis connection-loss guidance:
  https://redis.io/docs/latest/develop/reference/client-side-caching/#what-to-do-when-losing-connection-with-the-server
- Redis `CLIENT TRACKING`:
  https://redis.io/docs/latest/commands/client-tracking/
- Redis Cloud/Software compatibility:
  https://redis.io/docs/latest/operate/rs/references/compatibility/client-side-caching/
- go-redis/v9 source pinned at v9.20.0:
  https://github.com/redis/go-redis/tree/7d05dd3b7ce12a7b8c7923f73da0fede3bfa7c03
