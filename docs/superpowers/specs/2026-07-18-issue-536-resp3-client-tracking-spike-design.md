# Issue #536 RESP3 CLIENT TRACKING Spike Design

Status: approved decision boundary; awaiting written spec review
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
- Handler는 command processing goroutine에서 동기 실행되고 handler error는 log된 뒤
  command caller에 반환되지 않는다. Callback은 bounded L1-only operation과 관측값
  기록만 수행해야 한다.

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
                 | key payload      | null payload / disconnect
                 v                  v
       TieredCache.InvalidateLocal  TieredCache.ClearLocal
                 |                  |
                 +--------> caller-owned L1

                 ValueCache Redis L2 is never mutated by the handler
```

The spike has three independently owned connections:

1. A test-owned `*redis.Client` configured with protocol 3 and a retained push
   processor.
2. A sticky `*redis.Conn` from that client for serialized tracking/read/drain
   commands.
3. A separate writer/admin client that mutates keys, flushes the database, and
   kills the tracked client when required.

The handler is test-only. It validates the payload shape, records a bounded
observation, maps the known test physical key to its logical key, and calls only
`InvalidateLocal` or `ClearLocal`. Unknown namespace keys, malformed payloads,
and local invalidation failures are recorded as spike failures rather than
silently ignored.

## RESP3 Negotiation And Tracking Setup

Each test must prove its prerequisites before asserting invalidation behavior:

1. Start Redis 7.4 with the existing Testcontainers helper.
2. Create a client with `Protocol: 3`, `PoolSize: 1`, and an injected
   `redis.NewPushNotificationProcessor()`.
3. Register the `invalidate` handler with `protected=false`.
4. Acquire one sticky connection and issue `HELLO 3`; assert the response reports
   protocol 3.
5. Run `CLIENT TRACKING ON NOLOOP` on that connection.
6. Read the physical Redis value key through that same connection so Redis tracks
   it.

`PoolSize: 1` reduces scheduling noise but is not treated as a production
affinity guarantee. Tests that prove affinity hold two explicit sticky
connections instead of depending on pool scheduling.

## Test Matrix

### Command-coupled delivery and L1-only invalidation

`TestRESP3TrackingSpikeDeliversInvalidationOnlyWhenTrackedConnectionReads`

- Populate Redis L2 and `TieredCache` L1 with value `old`.
- Read the physical key through the tracked connection.
- Mutate that Redis key to `new` from the external writer.
- Before another tracked-connection command, assert no handler observation and
  that the tiered read still returns the stale L1 value `old`.
- Issue `PING` on the tracked connection.
- Assert exact key invalidation payload, successful `InvalidateLocal`, and a
  subsequent tiered read returning `new` from L2.
- Assert the callback did not issue Redis mutation commands.

This is both the positive RESP3 proof and the principal production blocker:
delivery exists, but it is not autonomous.

### Connection affinity

`TestRESP3TrackingSpikeRequiresReadAndTrackingOnSameConnection`

- Hold sticky connections A and B concurrently.
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
- Kill that connection from the admin client.
- Mutate the key while the tracked connection is unavailable.
- Assert the stale L1 remains populated and no missed invalidation is replayed.
- Close the dead sticky connection, clear L1, obtain a new sticky connection,
  verify RESP3, and re-enable tracking.
- Re-read and cache the value, mutate again, drain with `PING`, and prove
  invalidation resumes.

The safe lifecycle order is fixed as: detect loss -> block use of tracked L1 ->
`ClearLocal` -> create connection -> verify RESP3 -> enable tracking -> allow
cacheable reads. `OnConnect` alone cannot close the missed-invalidation window.

### Deterministic shutdown

`TestRESP3TrackingSpikeShutdownUnregistersHandler`

- Retain the injected processor and register the handler unprotected.
- Close the sticky connection and client.
- Unregister `invalidate` through the retained processor.
- Assert bounded completion, no owned goroutine leak, and no callback after
  unregistration.

The test documents that registering directly on a caller-created client without
retaining processor ownership is insufficient for a production component.

## Synchronization And Flake Control

- No unbounded sleeps or eventual assertions.
- Use context deadlines, observation channels, and short bounded negative
  windows only where absence is the behavior under test.
- A sticky `*redis.Conn` is used by one goroutine at a time.
- Handler observations are buffered so synchronous command processing cannot
  block on the test reader.
- Every test owns unique namespace/key values and performs idempotent cleanup.
- Testcontainers cases run sequentially when they share Docker resources.
- Connection-kill tests use `CLIENT ID` and `CLIENT KILL ID`, not timing-based
  guesses about pool members.

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

- adopt RESP3 parsing and dedicated-connection heartbeat as technically proven;
- reject a production `redisnear.NewTracking` API on go-redis/v9 v9.20.0;
- keep `redisnear.NewPubSub` as the production strategy;
- open a separate Type A issue only if a dedicated pump/connection-owner
  subsystem or future go-redis background push API is intentionally pursued.

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
2. Repetition of timing-sensitive cases with `-count` to expose flakes.
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
| Prove shutdown | Retained processor unregisters handler after bounded close |
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
