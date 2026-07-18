# Issue #536 RESP3 CLIENT TRACKING Spike Evidence

- Issue: [#536](https://github.com/bluetape4k/bluetape-go/issues/536)
- Milestone: `0.19.0`
- Evidence captured: 2026-07-19 00:40:41 KST (`+0900`)
- Work type: Type B prove-or-reject spike; test and research only

## Executive Decision

The spike reproduced command-coupled RESP3 invalidation handling with public
go-redis APIs. An exact key invalidation and a global invalidation were both
processed after an explicit command drained the tracked socket.

That positive frame-delivery result is not an autonomous coherent near-cache.
Before the drain, a `TieredCache` L1 hit returned the stale value without
running a Redis command. Tracking also applied only when enablement and the
cacheable read used the same physical connection. A killed connection lost its
tracking state and did not replay an invalidation missed during disconnection.

Therefore an autonomous coherent pooled near-cache is rejected on
`github.com/redis/go-redis/v9` v9.20.0. `redisnear.NewPubSub` remains the
production strategy. This spike added only a test file and this research
ledger; it added no production API. A dedicated push pump/connection owner, or
adoption of a future go-redis autonomous push API, requires a separate Type A
issue and approved design.

## Environment Ledger

The source and runtime identities below were captured before this evidence-note
commit.

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

The configured tag and engine image ID were read from the same disposable
container with `docker inspect`. The Redis fields came from that container's
`INFO server`. The container accepted `PING` before inspection and was removed
afterward. Every Testcontainers instance created by the test commands also
reported successful termination.

| Command | Result |
|---|---|
| `go test -p 1 -count=1 -timeout=5m ./cache/redisnear -run '^TestRESP3TrackingSpike' -v` | PASS; complete handler, protocol, integration, reconnect, unregister, and shutdown spike suite |
| `go test -p 1 -count=3 -timeout=15m ./cache/redisnear -run '^TestRESP3TrackingSpike(DeliversInvalidationOnlyWhenTrackedConnectionReads\|RequiresReadAndTrackingOnSameConnection\|MapsGlobalInvalidationToClearLocal\|ReconnectRequiresReenableAndLocalFlush\|UnregisterIsNotAQuiescenceBarrier\|ShutdownOrdersQuiescenceBeforeUnregister)$' -v` | PASS; all six selected cases passed in each of three iterations |
| `go test -race -count=1 ./cache/redisnear` | PASS |
| `go test -count=1 ./cache/redisvalue ./cache/redisnear` | PASS for both packages |
| `go version` | `go version go1.26.5 darwin/arm64` |
| `go list -m github.com/redis/go-redis/v9` | `github.com/redis/go-redis/v9 v9.20.0` |
| `git rev-parse HEAD` | `ccda9fca03bb4a4f100be9bc8fdd7ed1f011fe99` |

## Result Matrix

| Proof | Observed result | Decision effect |
|---|---|---|
| RESP3 negotiation | `HELLO 3` returned protocol `3`; `CLIENT ID` was positive; `CLIENT TRACKING ON NOLOOP` succeeded. | RESP3 tracking is technically available on the captured Redis/go-redis pair. |
| Exact key payload | After the tracked connection read the physical key, an external `SET` produced one exact key invalidation only when `PING` drained that connection. The handler mapped the precomputed physical key to one logical L1 deletion. | Explicit command drain is proven; frame parsing alone does not establish autonomous delivery. |
| Global payload | After tracked reads of two physical keys, test-admin `FLUSHDB` produced the null/global invalidation shape on drain. The handler called `ClearLocal`, and both later tiered reads returned `cache.ErrCacheMiss`. | A drained global frame can repair L1 without another Redis mutation from the callback. |
| Idle stale L1 | After external `SET` completed and before another command on the tracked socket, no observation existed and `TieredCache.Get` returned stale L1 value `old`. | L1-only hits can remain stale because they do not drain the RESP3 socket. |
| Physical affinity | Tracking enabled on sticky connection A did not observe a key read on distinct sticky connection B. A later read and drain on A produced the exact invalidation. | A normal pooled client does not by itself prove tracking/read/drain affinity. |
| Disconnect window | Test-admin `CLIENT KILL ID` killed exactly the tracked connection. A writer mutation during loss was not replayed; retry-disabled `PING` exposed a transport failure while stale L1 remained. | Loss detection must block tracked-L1 use and precede a full local clear. |
| Replacement tracking | Replacement connection B had a different client ID and `CLIENT TRACKINGINFO` reported `off`. After explicit re-enable and a new physical read, invalidation resumed on drain. | Tracking state is connection-local and must be re-established after replacement. |
| Unregister | `UnregisterHandler("invalidate")` returned while a previously selected callback remained in flight. | Unregister prevents future lookup but is not a callback-quiescence barrier. |
| Ordered shutdown | Closing the test gate rejected successor callbacks; waiting for the admitted generation, then unregistering and closing the connection/client, completed within the test watchdogs. | A production component would need explicit callback, processor, connection, and close ownership. |
| Repetition | The command drain, affinity, global payload, reconnect, unregister, and shutdown cases each passed three consecutive executions with Docker-backed cases serialized. | The captured behavior was repeatable at the pinned source and runtime identities. |
| Race | `go test -race -count=1 ./cache/redisnear` passed. | The test-owned handler, observation, gate, and shutdown paths produced no race report. |

The fixture uses `TieredCache[string]` over `ValueCache[string]`.
`TieredCache` stores `V` directly in its process-local L1, so a reference-shaped
`V` remains a reference object there and an L1 hit performs no serialization.
`ValueCache` serializes `V` before writing Redis L2 and deserializes the bounded
Redis payload on read. This spike uses strings and does not claim a mutable
reference-aliasing proof. Its handler calls only `InvalidateLocal` or
`ClearLocal`, so invalidation changes L1 state without writing or clearing L2.

## Pub/Sub Comparison

| Dimension | Production `redisnear.NewPubSub` | RESP3 spike result |
|---|---|---|
| Receive ownership | Owns a dedicated `PubSub.ReceiveMessage` loop. | The public push processor was exercised through command reads on the tracked socket. |
| L1-only hit | The subscription loop can receive an application invalidation independently of cache reads. | An L1 hit runs no Redis command and did not consume the pending frame. |
| External writer | Invisible unless it publishes the bluetape invalidation protocol. | Redis-native tracking observed an external mutation after the correct connection read and explicit drain. |
| Affinity | The subscription connection is explicit. | Tracking enablement, cacheable read, and frame drain must share the relevant physical connection. |
| Connection loss | Ordinary `ReceiveMessage` errors clear L1 and are retried automatically with backoff; predecessor guidance reserves instance recreation for terminal or unrecoverable subscriber failure. | The missed invalidation was not replayed; safe recovery required blocking L1, clearing it, replacing the connection, verifying RESP3, and re-enabling tracking. |
| Shutdown | `NearCache` owns receiver cancellation, Pub/Sub close, and bounded completion. | The spike had to retain the processor, quiesce admitted callbacks, unregister, and close owned resources in order. |

RESP3 closes the external-writer visibility gap only under connection and drain
conditions that the current pooled production surface does not own. That is
not enough to displace `redisnear.NewPubSub`.

## Provider And ACL Assumptions

The live proof covers one unauthenticated, standalone Redis OSS `7.4.9`
container using RESP3. Redis OSS forced to RESP2, or negotiated down to RESP2
by configuration or fallback, is rejected for this tracking strategy because
the RESP3 tracking push contract used by the spike is unavailable. This does
not imply that Redis Pub/Sub is unavailable; `redisnear.NewPubSub` uses a
separate production contract.

The live proof does not cover AUTH, TLS, certificate validation, Sentinel,
Cluster, a managed service, or a proxy. Redis Cloud/Software support is a
documented provider requirement, not live evidence here; its `REDIRECT` mode
is documented as unsupported. Sentinel, Cluster, and generic proxies remain
unproven until a topology-specific spike verifies `HELLO 3`, tracking, push
preservation, physical-connection loss detection, and recovery.

| Identity | Required commands in this spike | Explicit boundary |
|---|---|---|
| Tracked runtime | `HELLO 3`, `CLIENT TRACKING`, `CLIENT TRACKINGINFO`, `CLIENT ID`, `GET`, `PING` | Does not require `FLUSHDB`, `FLUSHALL`, or `CLIENT KILL`. |
| Tiered L2 runtime | go-redis RESP3 client initialization (`Protocol: 3`, including `HELLO 3` negotiation), `SET` for writes, and the initial `GETRANGE` for the captured non-empty string and miss paths. The existing empty-payload ambiguity path conditionally adds `MULTI`, a second `GETRANGE`, `EXISTS`, and `EXEC`; that path was not exercised by this spike. | ACL policy must cover connection initialization and whichever `ValueCache` command paths are enabled; destructive admin commands remain unnecessary. See the `ValueCache` source below. |
| External writer | Ordinary `SET` | Does not require destructive admin commands. |
| Disposable-test admin | `FLUSHDB`, `CLIENT KILL ID` | Exists only inside a fresh test-owned container; there is no production equivalent. |

The test fixture constructs its admin client from the fresh container endpoint
and accepts no environment-provided endpoint or dialer. Handler failures and
observations retain neither raw keys nor credentials, endpoints, or provider
errors. A production deployment would still have to define separate AUTH/TLS
and certificate ownership and grant only the commands required by each
identity.

## Performance Boundary

The only qualitative resource statements supported by this design are that one
tracking owner uses one dedicated socket and each explicit drain adds one Redis
command. This evidence publishes no latency, throughput, CPU, memory, heartbeat
cadence, or provider ranking. Issue [#560](https://github.com/bluetape4k/bluetape-go/issues/560)
owns those measurements.

## Follow-up Rule

Do not add `redisnear.NewTracking`, tracking options, a strategy enum, an
exported physical-key mapper, a background pump, or a reconnect subsystem from
this spike. Revisit only through a separate Type A issue when either:

1. go-redis exposes an autonomous push-consumption lifecycle that works
   independently of ordinary cache commands; or
2. bluetape-go intentionally owns a dedicated connection/pump, callback
   quiescence, loss fencing, full-L1 repair, tracking re-enable, provider
   compatibility, and deterministic shutdown under an approved design.

The follow-up must prove autonomous delivery, read affinity, loss fencing,
recovery, and provider topology before proposing a production API.

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
