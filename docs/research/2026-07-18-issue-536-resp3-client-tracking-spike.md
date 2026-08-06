# Issue #536 RESP3 CLIENT TRACKING Spike Evidence

- Issue: [#536](https://github.com/bluetape4k/bluetape-go/issues/536)
- Milestone: `0.19.0`
- Evidence captured: 2026-07-19 00:40:41 KST (`+0900`)
- Work type: Type B prove-or-reject spike; test and research only

## 실행 결정

이 spike는 public go-redis API만으로 command-coupled RESP3 invalidation handling을
재현했다. explicit command가 tracked socket을 drain한 뒤 exact key invalidation과
global invalidation이 모두 처리되었다.

그러나 이 positive frame-delivery 결과는 autonomous coherent near-cache가 아니다.
drain 전에는 `TieredCache` L1 hit가 Redis command를 실행하지 않고 stale value를 반환했다.
또한 tracking은 enablement와 cacheable read가 같은 physical connection을 사용할 때만
적용되었다. kill된 connection은 tracking state를 잃었고, disconnection 중 놓친
invalidation은 replay되지 않았다.

따라서 `github.com/redis/go-redis/v9` v9.20.0 위에서 autonomous coherent pooled
near-cache는 거절한다. `redisnear.NewPubSub`이 production strategy로 남는다. 이 spike는
test file과 이 research ledger만 추가했으며 production API는 추가하지 않았다. dedicated
push pump/connection owner 또는 future go-redis autonomous push API 채택은 별도 Type A
issue와 승인된 design을 필요로 한다.

## Environment Ledger

아래 source와 runtime identity는 이 evidence-note commit 전에 캡처했다.

| Item | Captured value |
|---|---|
| Source head | `ccda9fca03bb4a4f100be9bc8fdd7ed1f011fe99` |
| Branch | `test/issue-536-resp3-client-tracking-spike` |
| Clock | `2026-07-19 00:40:41 KST (+0900)` |
| Go | `go version go1.26.5 darwin/arm64` |
| go-redis | `github.com/redis/go-redis/v9 v9.20.0` |
| Testcontainers | `github.com/testcontainers/testcontainers-go v0.42.0` |
| Docker engine | Server `28.4.0`, API `1.51`, Ubuntu `24.04.2 LTS`, Colima socket `unix:///Users/debop/.colima/default/docker.sock` |
| Configured Redis image | `redis:7.4-alpine` |
| Docker engine image ID | `sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99` |
| Redis version | `7.4.9` |
| Redis build ID | `5a23d515cf8f8935` |
| Redis mode | `standalone` |
| Redis OS | `Linux 6.8.0-64-generic aarch64` |
| Redis architecture | `64` bits |
| Redis process ID | `1` inside the disposable container |

configured tag와 engine image ID는 같은 disposable container에서 `docker inspect`로 읽었다.
Redis field는 해당 container의 `INFO server`에서 가져왔다. container는 inspection 전에
`PING`을 수락했고 이후 제거되었다. test command가 만든 모든 Testcontainers instance도
successful termination을 보고했다.

| Command | Result |
|---|---|
| `go test -p 1 -count=1 -timeout=5m ./cache/redisnear -run '^TestRESP3TrackingSpike' -v` | PASS; complete handler, protocol, integration, reconnect, unregister, and shutdown spike suite |
| `go test -p 1 -count=3 -timeout=15m ./cache/redisnear -run '^TestRESP3TrackingSpike(DeliversInvalidationOnlyWhenTrackedConnectionReads\|RequiresReadAndTrackingOnSameConnection\|MapsGlobalInvalidationToClearLocal\|ReconnectRequiresReenableAndLocalFlush\|UnregisterIsNotAQuiescenceBarrier\|ShutdownOrdersQuiescenceBeforeUnregister)$' -v` | PASS; all six selected cases passed in each of three iterations |
| `go test -race -count=1 ./cache/redisnear` | PASS |
| `go test -count=1 ./cache/redisvalue ./cache/redisnear` | PASS for both packages |
| `go version` | `go version go1.26.5 darwin/arm64` |
| `go list -m github.com/redis/go-redis/v9` | `github.com/redis/go-redis/v9 v9.20.0` |
| `git rev-parse HEAD` | `ccda9fca03bb4a4f100be9bc8fdd7ed1f011fe99` |

## 결과 매트릭스

| Proof | Observed result | Decision effect |
|---|---|---|
| RESP3 negotiation | `HELLO 3` returned protocol `3`; `CLIENT ID` was positive; `CLIENT TRACKING ON NOLOOP` succeeded. | 캡처된 Redis/go-redis pair에서 RESP3 tracking은 기술적으로 가능하다. |
| Exact key payload | tracked connection이 physical key를 읽은 뒤 external `SET`이 정확히 하나의 exact key invalidation을 만들었고, 이는 `PING`이 해당 connection을 drain했을 때만 관측되었다. handler는 precomputed physical key를 하나의 logical L1 deletion으로 mapping했다. | explicit command drain은 증명되었지만 frame parsing만으로 autonomous delivery가 성립하지 않는다. |
| Global payload | 두 physical key의 tracked read 이후 test-admin `FLUSHDB`가 drain 시 null/global invalidation shape를 만들었다. handler는 `ClearLocal`을 호출했고 이후 두 tiered read 모두 `cache.ErrCacheMiss`를 반환했다. | drained global frame은 callback에서 추가 Redis mutation 없이 L1을 복구할 수 있다. |
| Idle stale L1 | external `SET` 완료 후 tracked socket에서 다른 command가 실행되기 전에는 관측이 없었고, `TieredCache.Get`은 stale L1 value `old`를 반환했다. | L1-only hit는 RESP3 socket을 drain하지 않으므로 stale 상태로 남을 수 있다. |
| Physical affinity | sticky connection A에서 tracking을 켰지만 distinct sticky connection B의 key read는 관측하지 못했다. 이후 A에서 read와 drain을 수행하자 exact invalidation이 발생했다. | normal pooled client만으로는 tracking/read/drain affinity가 증명되지 않는다. |
| Disconnect window | test-admin `CLIENT KILL ID`가 정확히 tracked connection을 kill했다. loss 중 writer mutation은 replay되지 않았고, retry-disabled `PING`은 stale L1이 남은 상태에서 transport failure를 드러냈다. | loss detection은 tracked-L1 사용을 막고 full local clear보다 앞서야 한다. |
| Replacement tracking | replacement connection B는 다른 client ID를 가졌고 `CLIENT TRACKINGINFO`는 `off`를 보고했다. explicit re-enable과 새 physical read 이후 drain에서 invalidation이 재개되었다. | tracking state는 connection-local이며 replacement 후 다시 설정해야 한다. |
| Unregister | `UnregisterHandler("invalidate")`가 반환된 뒤에도 이전에 선택된 callback은 계속 in flight였다. | unregister는 future lookup을 막지만 callback-quiescence barrier가 아니다. |
| Ordered shutdown | test gate가 successor callback을 거절했고, admitted generation을 기다린 뒤 unregister와 connection/client close를 수행해 test watchdog 안에서 완료했다. | production component에는 explicit callback, processor, connection, close ownership이 필요하다. |
| Repetition | command drain, affinity, global payload, reconnect, unregister, shutdown case가 각각 세 번 연속 통과했으며 Docker-backed case는 serialized로 실행되었다. | 캡처된 source와 runtime identity에서 behavior가 repeatable했다. |
| Race | `go test -race -count=1 ./cache/redisnear` passed. | test-owned handler, observation, gate, shutdown path에서 race report가 없었다. |

fixture는 `ValueCache[string]` 위의 `TieredCache[string]`를 사용한다. `TieredCache`는
process-local L1에 `V`를 직접 저장하므로 reference-shaped `V`는 L1에서 reference object로
남고 L1 hit는 serialization을 수행하지 않는다. `ValueCache`는 Redis L2에 쓰기 전에 `V`를
serialize하고 read 시 bounded Redis payload를 deserialize한다. 이 spike는 string을 사용하며
mutable reference-aliasing proof를 주장하지 않는다. handler는 `InvalidateLocal` 또는
`ClearLocal`만 호출하므로 invalidation은 L2를 쓰거나 비우지 않고 L1 state만 변경한다.

## Pub/Sub 비교

| Dimension | Production `redisnear.NewPubSub` | RESP3 spike result |
|---|---|---|
| Receive ownership | dedicated `PubSub.ReceiveMessage` loop를 소유한다. | public push processor는 tracked socket의 command read를 통해 실행되었다. |
| L1-only hit | subscription loop는 cache read와 독립적으로 application invalidation을 받을 수 있다. | L1 hit는 Redis command를 실행하지 않았고 pending frame을 소비하지 않았다. |
| External writer | bluetape invalidation protocol을 publish하지 않으면 보이지 않는다. | Redis-native tracking은 올바른 connection read와 explicit drain 뒤 external mutation을 관측했다. |
| Affinity | subscription connection이 explicit하다. | tracking enablement, cacheable read, frame drain이 관련 physical connection을 공유해야 한다. |
| Connection loss | 일반 `ReceiveMessage` error는 L1을 clear하고 backoff로 자동 retry한다. predecessor guidance는 terminal 또는 unrecoverable subscriber failure에만 instance recreation을 남겨 둔다. | 놓친 invalidation은 replay되지 않았다. safe recovery에는 L1 차단, clear, connection replacement, RESP3 verification, tracking re-enable이 필요했다. |
| Shutdown | `NearCache`가 receiver cancellation, Pub/Sub close, bounded completion을 소유한다. | spike는 processor를 보유하고 admitted callback을 quiesce한 뒤 unregister와 owned resource close를 순서대로 수행해야 했다. |

RESP3는 current pooled production surface가 소유하지 않는 connection/drain 조건 아래에서만
external-writer visibility gap을 닫는다. 이것만으로는 `redisnear.NewPubSub`을 대체하기에
부족하다.

## Provider And ACL Assumptions

live proof는 RESP3를 사용하는 unauthenticated standalone Redis OSS `7.4.9` container 하나만
다룬다. Redis OSS가 RESP2로 강제되거나 configuration/fallback으로 RESP2에 negotiate되면,
이 spike가 사용한 RESP3 tracking push contract를 사용할 수 없으므로 tracking strategy를
거절한다. 이는 Redis Pub/Sub이 불가능하다는 뜻이 아니다. `redisnear.NewPubSub`은 별도
production contract를 사용한다.

live proof는 AUTH, TLS, certificate validation, Sentinel, Cluster, managed service, proxy를
다루지 않는다. Redis Cloud/Software support는 문서화된 provider requirement이지 여기의 live
evidence가 아니다. 해당 `REDIRECT` mode는 unsupported로 문서화되어 있다. Sentinel, Cluster,
generic proxy는 topology-specific spike가 `HELLO 3`, tracking, push preservation,
physical-connection loss detection, recovery를 검증하기 전까지 unproven이다.

| Identity | Required commands in this spike | Explicit boundary |
|---|---|---|
| Tracked runtime | `HELLO 3`, `CLIENT TRACKING`, `CLIENT TRACKINGINFO`, `CLIENT ID`, `GET`, `PING` | `FLUSHDB`, `FLUSHALL`, `CLIENT KILL`은 필요하지 않다. |
| Tiered L2 runtime | go-redis RESP3 client initialization (`Protocol: 3`, including `HELLO 3` negotiation), `SET` for writes, and the initial `GETRANGE` for captured non-empty string and miss paths. An empty first read, including the captured post-`FLUSHDB` misses, adds `MULTI`, a second `GETRANGE`, `EXISTS`, and `EXEC`. A stored empty-payload hit was not separately exercised, although the same branch distinguishes its presence from a miss. | ACL policy는 connection initialization과 enabled `ValueCache` command path를 포함해야 한다. destructive admin command는 여전히 불필요하다. 아래 `ValueCache` source를 참고한다. |
| External writer | Implicit client initialization: RESP3 negotiation (`Protocol: 3`, `HELLO 3`); explicit test command: ordinary `SET`. | destructive admin command가 필요하지 않다. |
| Disposable-test admin | Implicit client initialization: RESP3 negotiation (`Protocol: 3`, `HELLO 3`); explicit fixture/test commands: `INFO server`, `FLUSHDB`, and `CLIENT KILL ID`. | fresh test-owned container 안에만 존재하며 production equivalent는 없다. |

test fixture는 fresh container endpoint에서 admin client를 만들며 environment-provided endpoint나
dialer를 받지 않는다. handler failure와 observation은 raw key, credential, endpoint,
provider error를 보존하지 않는다. production deployment는 여전히 별도 AUTH/TLS와 certificate
ownership을 정의하고 각 identity에 필요한 command만 허용해야 한다.

## Performance Boundary

이 design이 뒷받침하는 정성적 resource statement는 tracking owner 하나가 dedicated socket
하나를 사용하고 explicit drain마다 Redis command 하나가 추가된다는 점뿐이다. 이 evidence는
latency, throughput, CPU, memory, heartbeat cadence, provider ranking을 공개하지 않는다.
Issue [#560](https://github.com/bluetape4k/bluetape-go/issues/560)이 해당 측정을 소유한다.

## Follow-up Rule

이 spike에서 `redisnear.NewTracking`, tracking options, strategy enum, exported physical-key
mapper, background pump, reconnect subsystem을 추가하지 않는다. 다음 중 하나가 성립하는 별도
Type A issue에서만 재검토한다.

1. go-redis가 ordinary cache command와 독립적으로 동작하는 autonomous push-consumption
   lifecycle을 제공한다.
2. bluetape-go가 dedicated connection/pump, callback quiescence, loss fencing, full-L1
   repair, tracking re-enable, provider compatibility, deterministic shutdown을 승인된 design
   아래 의도적으로 소유한다.

follow-up은 production API를 제안하기 전에 autonomous delivery, read affinity, loss fencing,
recovery, provider topology를 증명해야 한다.

## Source Links

- [Issue #536 spike tests](../../cache/redisnear/resp3_tracking_spike_test.go)
- [Approved design](../superpowers/specs/2026-07-18-issue-536-resp3-client-tracking-spike-design.md)
- [Approved test specification](../superpowers/specs/2026-07-18-issue-536-resp3-client-tracking-spike-test-spec.md)
- [Approved implementation plan](../superpowers/plans/2026-07-18-issue-536-resp3-client-tracking-spike-plan.md)
- [Issue #110 predecessor research](2026-06-04-issue-110-resp3-client-tracking.md)
- [`redisnear.NewPubSub` production source](../../cache/redisnear/near_cache.go)
- [`redisvalue.TieredCache` source](../../cache/redisvalue/tiered_cache.go)
- [`redisvalue.ValueCache` source](../../cache/redisvalue/value_cache.go)
- [go-redis v9.20.0 client push registration](https://github.com/redis/go-redis/blob/7d05dd3b7ce12a7b8c7923f73da0fede3bfa7c03/redis.go#L1467-L1493)
- [go-redis v9.20.0 command path invoking pending-push processing](https://github.com/redis/go-redis/blob/7d05dd3b7ce12a7b8c7923f73da0fede3bfa7c03/redis.go#L952-L990)
- [go-redis v9.20.0 command-coupled pending-push processing](https://github.com/redis/go-redis/blob/7d05dd3b7ce12a7b8c7923f73da0fede3bfa7c03/redis.go#L1699-L1765)
- [go-redis v9.20.0 push processor and swallowed handler errors](https://github.com/redis/go-redis/blob/7d05dd3b7ce12a7b8c7923f73da0fede3bfa7c03/push/processor.go#L14-L113)
- [go-redis v9.20.0 handler registry and unregister behavior](https://github.com/redis/go-redis/blob/7d05dd3b7ce12a7b8c7923f73da0fede3bfa7c03/push/registry.go#L29-L61)
- [Redis client-side caching](https://redis.io/docs/latest/develop/clients/client-side-caching/)
- [Redis connection-loss guidance](https://redis.io/docs/latest/develop/reference/client-side-caching/#what-to-do-when-losing-connection-with-the-server)
- [Redis `CLIENT TRACKING`](https://redis.io/docs/latest/commands/client-tracking/)
- [Redis Cloud/Software client-side caching compatibility](https://redis.io/docs/latest/operate/rs/references/compatibility/client-side-caching/)
