# Issue 23 Near Cache Invalidation Strategies Research

Issue: #23
Milestone: 0.3.0
Date: 2026-06-04

## Research Question

`bluetape-go` near cache should support a local L1 cache with Redis-backed
invalidation. The design question is whether to expose only an application-level
Redis Pub/Sub invalidation implementation, or to leave a first-class extension
point for Redis server-assisted client-side caching through RESP3
`CLIENT TRACKING`.

## Primary Evidence

| Source | Evidence | Decision impact |
|---|---|---|
| GitHub issue #23 | Requires local TTL cache, explicit invalidation, Redis Pub/Sub peer invalidation, and a Testcontainers two-client proof | The first implementation must prove app-level Pub/Sub invalidation. |
| Redis client-side caching introduction | Redis server-assisted client-side caching tracks keys read by a client and sends invalidation when another client writes those keys | RESP3 tracking is a different invalidation strategy, not just another channel format. |
| Redis `CLIENT TRACKING` command reference | Invalidation messages can be delivered on the same connection with RESP3, or redirected to another connection; options include `BCAST`, `PREFIX`, `OPTIN`, `OPTOUT`, and `NOLOOP` | A future tracking implementation needs dedicated lifecycle and option modeling. |
| Redis client-side caching reference | Default tracking stores client-key relationships server-side; broadcasting mode avoids server tracking memory but sends prefix notifications. RESP2 Pub/Sub redirection is a compatibility mechanism and not ordinary broadcast Pub/Sub | App-level Pub/Sub and server-assisted tracking must not be collapsed into one semantic mode. |
| go-redis repository and Redis blog | `go-redis/v9` supports RESP3 and remains the repository's existing Redis dependency | RESP3 is technically plausible without replacing the Redis dependency, but high-level client-side caching behavior still needs proof. |
| Current `cache` package | `cache.Memory` already provides TTL, `Delete`, `Clear`, `GetOrLoad`, same-key duplicate-load suppression, and stress/cancellation coverage | Reuse `cache.Memory` as the default local L1 store instead of choosing Ristretto or BigCache before benchmark evidence. |
| Milestone 0.3.0 research | Ristretto and BigCache are local-cache candidates; `go-redis/v9` is the Redis backend candidate | Keep storage and invalidation strategy independent so #107 can benchmark storage choices later. |

## Strategy Comparison

| Strategy | Strengths | Risks |
|---|---|---|
| Application-level Redis Pub/Sub invalidation | Simple with `go-redis/v9`; works with ordinary Pub/Sub; can carry bluetape-go-specific fields such as namespace, operation, origin ID, and version; easy to prove with Testcontainers; independent from RESP3 support in managed Redis/proxies | Only writes routed through this near-cache contract publish invalidations; subscribers can miss messages during disconnect; reconnect should clear local state; broadcast volume depends on namespace/channel design |
| RESP3 `CLIENT TRACKING` | Redis observes actual read keys and invalidates when any client modifies them; can reduce notification volume in default tracking mode; supports Redis-native options like `PREFIX`, `BCAST`, `NOLOOP`, `OPTIN`, and `OPTOUT`; closer to Lettuce/Redisson client-side caching semantics | Connection-pool and connection-lifecycle behavior is harder; disconnect requires local cache flush/retracking; push-message handling must be proven in Go; managed Redis/proxy RESP3 support can vary; API must expose Redis tracking options without leaking low-level protocol details |

## Design Decision

Expose near-cache invalidation as a strategy boundary instead of encoding one
hard-coded mode in the public API.

The first implementation should be application-level Redis Pub/Sub invalidation
because it satisfies #23 directly and can be validated with the existing
`go-redis/v9` dependency and Testcontainers. RESP3 `CLIENT TRACKING` should be
tracked as a separate follow-up implementation that satisfies the same public
near-cache contract where possible.

## Proposed API Direction

Use constructors or concrete strategy types rather than a single overloaded enum
with hidden behavior.

- `redisnear.NewPubSub(...)`: application-level invalidation channel and payload
  contract.
- Future `redisnear.NewTracking(...)`: Redis server-assisted client-side
  caching through RESP3 `CLIENT TRACKING`.
- Shared local store default: `cache.Memory[K, V]`.
- Shared public behavior: `Get`, `Set`, `Delete`, `Clear`, `GetOrLoad`, local
  TTL, explicit invalidation, close/shutdown semantics, and documented
  reconnect behavior.

The strategy option should be visible at construction time. Runtime switching
between Pub/Sub and tracking should not be supported because the lifecycle,
wire semantics, and invalidation guarantees differ.

## Pub/Sub Implementation Notes for #23

- Define a stable invalidation message with at least namespace, operation, key,
  origin ID, and protocol version.
- Do not invalidate peer caches from arbitrary Redis keyspace changes; only
  near-cache writes/deletes/clears publish messages.
- Use origin ID to avoid redundant local invalidation where appropriate.
- On subscription disconnect or reconnect ambiguity, clear the local cache to
  avoid stale reads.
- Use Testcontainers to prove two near-cache instances observe peer invalidation.
- Use `GoroutineStressTester` and `AsyncJobTester` for timing/concurrency and
  cancellation coverage.

## RESP3 Follow-up Notes

- Verify `go-redis/v9` push-message and connection-pinning behavior before
  committing the public constructor shape.
- Decide whether tracking uses default mode, `BCAST` + `PREFIX`, `OPTIN`, or
  `OPTOUT`.
- Document Redis server and managed Redis compatibility requirements.
- Define disconnect behavior: local flush, tracking re-enable, and how errors
  surface to users.
- Confirm how `NOLOOP` interacts with local write-through or cache-aside flows.
- Add a Testcontainers proof that external Redis writes invalidate a tracked
  local entry.

## Adopt / Borrow / Skip Decisions

| Candidate | Decision | Rationale |
|---|---|---|
| `cache.Memory` | Adopt for first L1 | Already merged, tested, generic, and sufficient for #23 semantics. |
| Ristretto / BigCache | Defer to #107 | Useful performance candidates, but storage choice should be benchmark-driven. |
| App-level Redis Pub/Sub | Adopt for #23 | Directly satisfies issue acceptance criteria and has predictable tests. |
| RESP3 `CLIENT TRACKING` | Track as follow-up | Valuable for Lettuce/Redisson parity and external-write invalidation, but lifecycle risk is higher. |
| One enum hiding both modes | Reject | It hides materially different lifecycle and consistency semantics. |

## Source Links

- Redis client-side caching introduction:
  https://redis.io/docs/latest/develop/clients/client-side-caching/
- Redis `CLIENT TRACKING` command:
  https://redis.io/docs/latest/commands/client-tracking/
- Redis client-side caching reference:
  https://redis.io/docs/latest/develop/reference/client-side-caching/
- go-redis repository:
  https://github.com/redis/go-redis
- Redis blog: Go-Redis is now an official Redis client:
  https://redis.io/blog/go-redis-official-redis-client/

## Step 1-R Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Official docs checked | Done | Redis client-side caching introduction/reference and `CLIENT TRACKING`. |
| Current repo checked | Done | Existing `cache.Memory`, issue #23, milestone 0.3.0 research, and `go.mod`. |
| Third-party assumptions checked | Done | `go-redis/v9` remains the Redis dependency and supports RESP3 at the client level. |
| Strategy risks identified | Done | Pub/Sub disconnect loss, RESP3 connection lifecycle, managed Redis compatibility, and storage benchmark timing. |
| Decision recorded | Done | Pub/Sub first, RESP3 tracking as separate strategy/follow-up. |
