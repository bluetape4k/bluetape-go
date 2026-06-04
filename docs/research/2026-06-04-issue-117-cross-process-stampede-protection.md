# Issue #117 Cross-Process Cache Stampede Protection Research

Issue: #117
Milestone: 0.3.0
Date: 2026-06-04
Work type: Type A - Full Feature

## Research Question

How should `bluetape-go` add cross-process cache stampede protection after the
#22 local cache contract and #23 Redis Pub/Sub NearCache work, without weakening
the existing `cache.LoadingCache` contract or turning `redisnear` into a hidden
value store?

## Current Behavior

`cache.Memory.GetOrLoad` suppresses duplicate same-key loads only inside one
cache instance. It uses `singleflight` and stores a successful loader result in
the process-local cache.

`cache/redisnear.NearCache.GetOrLoad` delegates to its local cache. Redis is
used for peer invalidation only. A peer invalidation evicts another process'
local entry, but it does not coordinate the following reload.

`lock/redis` now provides a reusable single-Redis-instance owner-token lock:
`SetNX(ctx, key, token, ttl)` for acquire and a compare-and-delete Lua script
for unlock.

## External Evidence

| Source | Relevant point | Decision impact |
|---|---|---|
| Redis `SET` docs, https://redis.io/docs/latest/commands/set/ | `SET` supports `NX` and positive expiration options such as `PX`. | Redis can atomically create a short-lived load lease. |
| Redis distributed locks docs, https://redis.io/docs/latest/develop/clients/patterns/distributed-locks/ | Single-instance locking uses a unique random value and release must compare the value before deleting; mutual exclusion is bounded by lock validity time. | Reuse #24 owner-token/TTL lock and document that over-TTL loaders may overlap. |
| Redis `EVAL` docs, https://redis.io/docs/latest/commands/eval/ | Scripts should receive accessed keys explicitly through `KEYS` and extra arguments through `ARGV`. | #24 unlock script remains the correct safe release primitive for Redis versions before `DELEX IFEQ`. |
| Redis cache stampede glossary, https://redis.io/glossary/cache-stampede/ | Stampedes occur when many clients regenerate an expired or missing item simultaneously. | The #117 target is not just peer invalidation; it needs load coordination after misses. |

## Key Design Constraint

Redis locks alone can serialize backend loader execution, but they cannot share a
loaded Go value between processes. `redisnear` is invalidation-only and has no
codec for generic `V any`, so a losing process cannot receive the winner's
loaded value unless an explicit value-sharing mechanism is added.

Therefore there are two materially different semantics:

- **Load serialization:** only one process executes a loader at a time, but
  other cold processes may execute the loader later after the lock is released.
- **Load result collapse:** one process executes the loader, then waiters reuse
  the same result and populate their local caches without running their loaders.

#117 should implement load result collapse because the issue asks for
cross-process duplicate-load suppression and Testcontainers proof with multiple
near-cache instances contending for the same cold key.

## Options

| Option | Summary | Pros | Cons | Decision |
|---|---|---|---|---|
| Change `redisnear.NearCache.GetOrLoad` by default | Add Redis coordination directly to all near-cache loads. | One package, minimal user discovery cost. | Hidden latency, new Redis commands on every cold miss, requires value codec that `redisnear` deliberately avoided, changes #23 invalidation-only contract. | Reject. |
| Add lock-only opt-in wrapper | Use #24 Redis lock around loader execution, no result sharing. | Small, no codec, easy to reason about lease expiry. | Prevents simultaneous backend load but does not collapse results across cold processes. Does not satisfy strongest #117 interpretation. | Reject for #117 primary implementation; document as a possible fallback pattern. |
| Add opt-in Redis coordinator with result envelope | Wrap any `cache.LoadingCache[string,V]`, use #24 lock plus a short-lived Redis result key encoded by a caller-supplied codec. | Keeps `cache.LoadingCache` unchanged, preserves `redisnear` default behavior, supports true cross-process result collapse, works with NearCache as underlying L1. | Requires explicit codec and documents Redis as transient coordination/result transport for this wrapper. | Adopt. |
| Add a Redis L2 cache package | Store durable cache values in Redis and make local caches L1. | Useful long-term for app caches that want shared values. | Larger semantics: eviction, TTL consistency, serialization policy, invalidation with writes that bypass L2. | Defer to a future issue if needed. |

## Adopted Direction

Add a new opt-in package under `cache/rediscoord`.

The package wraps an existing `cache.LoadingCache[string,V]`; for #117 tests,
that underlying cache is `cache/redisnear.NearCache[V]`. The wrapper implements
`cache.LoadingCache[string,V]` and delegates `Get`, `Set`, `Delete`, and `Clear`
to the underlying cache.

`GetOrLoad` behavior:

1. Check the underlying cache first.
2. On miss, try a Redis owner-token load lock keyed by namespace and cache key.
3. The winner runs the loader through the underlying cache and publishes a
   short-lived result envelope in Redis before releasing the lock.
4. Waiters poll the lock owner token and matching result envelope.
5. A waiter that sees a matching result uses a small local loader that returns
   the decoded value, so the underlying cache is populated without calling the
   user loader.
6. If the owner fails, the lock TTL bounds the wait. Peers retry after the lease
   expires or return their context error.

## Result Envelope

The result key is not a durable cache. It is a short-lived coordination artifact
used only by waiters for the current load attempt.

The envelope includes:

- `version`: result format version;
- `token`: Redis lock owner token for the attempt;
- `payload`: codec output for `V`.

Waiters only accept an envelope when its token matches the owner token they
observed for the current lock attempt. This avoids consuming stale result keys
left by a previous load window.

## Public API Direction

```go
package rediscoord

type Codec[V any] interface {
    Marshal(V) ([]byte, error)
    Unmarshal([]byte) (V, error)
}

type JSONCodec[V any] struct{}

type Options[V any] struct {
    Client       redis.Cmdable
    Cache        cache.LoadingCache[string, V]
    Namespace    string
    Codec        Codec[V]
    LockTTL      time.Duration
    ResultTTL    time.Duration
    PollInterval time.Duration
}

type StampedeCache[V any] struct { ... }

func NewStampedeCache[V any](options Options[V]) (*StampedeCache[V], error)
```

Defaults:

- `Namespace`: `default`;
- `LockTTL`: `5s`;
- `ResultTTL`: `1s`;
- `PollInterval`: `10ms`.

Required:

- `Client`;
- `Cache`;
- `Codec`.

## Failure Semantics

- Loader errors are not cached and do not publish a result envelope.
- Caller cancellation returns the caller's context error and does not cache a
  result.
- If a winner crashes or returns before publishing a result, the lock TTL lets a
  waiter retry instead of deadlocking.
- If a loader exceeds `LockTTL`, another process may acquire the load lock and
  run a loader concurrently. This is the same lease-validity boundary as #24 and
  Redis' single-instance lock guidance.
- The wrapper does not provide fencing tokens for writes to external systems.
- The result envelope is transient and must not be treated as a Redis-backed
  durable cache value.

## Test Requirements

- Unit tests for option validation, JSON codec, result envelope token matching,
  and wrapper interface conformance.
- Testcontainers test with two `redisnear.NearCache` instances under the same
  namespace:
  - prime both local caches;
  - invalidate the key through one peer;
  - concurrently call the coordinator wrappers for the same cold key;
  - assert one user loader invocation and equal returned values.
- Stress test using `GoroutineStressTester` across two wrappers and one key;
  assert every caller completes and loader count remains one for the cold burst.
- Cancellation test using `AsyncJobTester`; waiters must return context
  cancellation/deadline errors instead of hanging.
- Lease expiry test; an abandoned or over-TTL loader must not deadlock peers.
- Race-targeted test run for the new package and `cache/redisnear`.

## Benchmark Boundary

Do not add the coordinator benchmarks to `make ci`. #107 should be updated or
linked so opt-in benchmark work can measure:

- winner path overhead;
- waiter result-sharing latency;
- load count per operation under invalidation pressure;
- lease expiry recovery.

## Decision

Implement `cache/rediscoord.StampedeCache` as an explicit opt-in cross-process
load-result coordinator. Keep `cache.LoadingCache` unchanged and keep
`redisnear.NewPubSub` invalidation-only by default.
