# Issue #110 RESP3 CLIENT TRACKING NearCache Research

Issue: #110
Milestone: 0.3.0
Date: 2026-06-04
Work type: Type E - Research / Maintenance

## Research Question

Should `bluetape-go` provide Redis server-assisted NearCache invalidation based
on RESP3 `CLIENT TRACKING` as a first-class alternative to the existing
application-level Redis Pub/Sub strategy?

## Executive Decision

Do not implement RESP3 `CLIENT TRACKING` as a production `redisnear.NewTracking`
constructor in the current slice.

Keep the strategy boundary from #23, but require a separate Testcontainers spike
before public API commitment. The current `go-redis/v9` dependency can negotiate
RESP3 and handle RESP3 push notifications, but it does not expose a typed
`CLIENT TRACKING` API or a documented high-level client-side caching contract.
That makes a direct production implementation possible only through lower-level
`Do("CLIENT", "TRACKING", ...)` commands plus push notification handlers and
connection-pinning rules that must be proven first.

## Primary Evidence

| Source | Evidence | Impact |
|---|---|---|
| Redis client-side caching docs | Redis server-assisted client-side caching tracks keys read by a client and sends invalidation messages when another client modifies those keys. | RESP3 tracking can invalidate external writes that bypass `redisnear.NewPubSub`. |
| Redis `CLIENT TRACKING` command docs | `CLIENT TRACKING` supports default tracking, `BCAST`, `PREFIX`, `OPTIN`, `OPTOUT`, `NOLOOP`, and `REDIRECT`; invalidations are delivered as RESP3 push messages or redirected notifications. | A future API must model tracking mode explicitly instead of hiding it behind Pub/Sub options. |
| Redis client-side caching reference | Default mode keeps server-side client-key state; broadcasting mode trades server memory for broader prefix notifications. | Memory and notification-volume tradeoffs differ from Pub/Sub and must be documented. |
| `go-redis/v9` README and source at v9.20.0 | `Options.Protocol` supports RESP2/RESP3, default protocol initialization is RESP3, and `PushNotificationProcessor` / `RegisterPushNotificationHandler` exist for RESP3 push messages. | RESP3 proof is technically plausible without replacing `go-redis/v9`. |
| `go-redis/v9` source search | No first-class `ClientTracking` or `CLIENT TRACKING` typed method was found; low-level `Do` / `Conn.Do` remain available. | Public bluetape-go support would own the command shape, lifecycle, and parsing risk. |
| Current `cache/redisnear` implementation | `NewPubSub` already owns app-level invalidation message shape, origin filtering, receive-error local clear, recreate guidance, and Testcontainers outage coverage. | Pub/Sub remains the stable 0.3.0 implementation. RESP3 should not weaken its contract. |

## Strategy Comparison

| Dimension | Pub/Sub `NewPubSub` | RESP3 tracking candidate |
|---|---|---|
| Invalidation source | bluetape-go writes/deletes/clears publish explicit invalidation messages. | Redis observes tracked keys and invalidates when other clients mutate them. |
| External writes | Not covered unless external writers publish the bluetape-go protocol. | Potentially covered, which is the main value proposition. |
| Wire contract | bluetape-go JSON message with namespace, origin, operation, and version. | Redis-native invalidation push payloads. |
| Connection model | Ordinary Pub/Sub subscription plus documented recreate path after terminal subscriber failure. | Requires RESP3 connection behavior, tracking enablement, push handler registration, and reconnect re-enable policy. |
| API stability | Already implemented and tested in #23/#116. | Not ready for public API without spike evidence. |
| Managed Redis / proxy risk | Ordinary Pub/Sub support is widely available. | RESP3 push and tracking support must be verified per provider/proxy. |

## Recommended API Direction

Keep constructor-per-strategy.

```go
// Implemented and stable in 0.3.0.
redisnear.NewPubSub[V](ctx, redisnear.Options[V]{...})

// Candidate only. Do not expose until a spike proves lifecycle semantics.
redisnear.NewTracking[V](ctx, redisnear.TrackingOptions[V]{...})
```

Do not introduce a runtime enum such as `StrategyRESP3` inside the existing
`Options` type. The lifecycle, wire semantics, and provider compatibility
risks are materially different from Pub/Sub.

Candidate `TrackingOptions` should be explicit if implemented later:

- `Client` or pinned `Conn` provider.
- `Namespace` / key prefix mapping.
- `Mode`: default tracked keys, `BCAST`, `OPTIN`, or `OPTOUT`.
- `Prefixes`: valid only for `BCAST`.
- `NoLoop`: maps to Redis `NOLOOP`.
- `OnError`: receive, reconnect, tracking re-enable, and handler failures.
- `Local`: same `cache.LoadingCache[string,V]` abstraction as Pub/Sub.

## Lifecycle Requirements Before Implementation

A future implementation must prove all of the following before public API
commitment:

1. RESP3 negotiation is enforced and fails clearly when Redis or a proxy falls
   back to RESP2.
2. `CLIENT TRACKING ON ...` is enabled on the same connection whose reads are
   expected to be tracked, or the implementation documents an explicit redirect
   connection.
3. Invalidation push payloads can be parsed without racing normal command
   responses.
4. Disconnect, reconnect, pool connection replacement, and `ConnMaxLifetime`
   cause local cache flush and tracking re-enable.
5. `NOLOOP` behavior is defined for local write-through and cache-aside flows.
6. External Redis writes invalidate a tracked local entry in a Testcontainers
   proof.
7. Managed Redis / Redis proxy compatibility is documented as supported,
   unsupported, or unknown.

## Test Plan For A Future Spike

The spike should live in a separate PR and may stay internal until it converges.

- Start Redis with Testcontainers and `go-redis/v9` `Protocol: 3`.
- Register a push notification handler for tracking invalidations.
- Enable tracking with low-level `CLIENT TRACKING ON` using `Conn.Do` or
  `Client.Do` only after proving the connection affinity requirement.
- Read key `k1` through the tracked connection and populate local cache.
- Mutate `k1` from a separate Redis client that does not use `redisnear`.
- Assert the push handler observes invalidation and the local entry is removed.
- Repeat with reconnect/recreate and local flush.
- Repeat with `NOLOOP` to confirm local writer semantics.

## Decision Matrix

| Option | Decision | Reason |
|---|---|---|
| Implement `NewTracking` now | Reject | Too much lifecycle and low-level protocol risk without a spike. |
| Keep #23 `NewPubSub` as the only production strategy for 0.3.0 | Adopt | Already implemented, stress-tested, outage-tested, and documented. |
| Preserve `NewTracking` as a future strategy boundary | Adopt | External-write invalidation is valuable enough to keep the API path open. |
| Hide Pub/Sub and RESP3 behind one `Options.Strategy` enum | Reject | It obscures materially different connection and failure semantics. |
| Add benchmarks for RESP3 tracking in #107 | Defer | #107 should benchmark implemented cache behavior; RESP3 tracking benchmarks need the spike first. |

## Acceptance Criteria Mapping

| #110 criterion | Status | Evidence / decision |
|---|---|---|
| Document whether `go-redis/v9` can support robust `CLIENT TRACKING` without replacing the dependency | Done | Technically plausible through RESP3 + push handlers + low-level `Do`; not robust enough for public API without spike. |
| Decide public constructor/API shape | Done | Future `redisnear.NewTracking` with explicit `TrackingOptions`; no hidden enum. |
| Verify Redis server version and managed Redis compatibility assumptions | Partial | Redis docs support the feature; provider/proxy compatibility remains a required spike/documentation gate. |
| Define disconnect/reconnect behavior | Done | Local flush and tracking re-enable required; otherwise recreate and do not serve tracked local entries. |
| Define `NOLOOP` and external-write interaction | Done | `NOLOOP` must be explicit; external-write invalidation is the main reason to revisit RESP3. |
| Add or plan Testcontainers proof | Done | Test plan above is the required next implementation gate. |

## Source Links

- Redis client-side caching introduction:
  https://redis.io/docs/latest/develop/clients/client-side-caching/
- Redis `CLIENT TRACKING` command:
  https://redis.io/docs/latest/commands/client-tracking/
- Redis client-side caching reference:
  https://redis.io/docs/latest/develop/reference/client-side-caching/
- go-redis repository:
  https://github.com/redis/go-redis
- Local dependency inspected:
  `github.com/redis/go-redis/v9 v9.20.0`

## Final Recommendation

Close #110 with a research decision, not a production implementation.

The next milestone work should continue with #107 benchmark baselines and #24
Redis distributed locks as planned. RESP3 tracking should be re-opened only
after a focused spike proves push invalidation handling, connection affinity,
and reconnect semantics with Testcontainers.
