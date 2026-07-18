# Lessons Learned - Redis Tiered Value Cache (#535)

**Related issue:** #535

**Affected package:** `cache/redisvalue`

## L1: A local reference cache and a serialized remote cache need different boundaries

### Problem

Serializing or cloning values before every L1 write would make the local tier a
second serialization boundary. Pointer-valued callers would receive a different
object after refill, and healthy L1 hits would pay work that belongs only to the
remote tier.

### Decision

`TieredCache[V]` stores `V` directly in its exclusively owned L1. Only
`ValueCache[V]` invokes `serialization.Serializer[V]`. Callers that choose
pointer-valued `V` treat cached objects as immutable snapshots while cached.

### Evidence

`TestTieredCacheSetPreservesReference`,
`TestTieredCacheHealthyL1SkipsRemoteAndSerializer`,
`TestTieredCacheL2HitStoresDecodedReference`,
`TestTieredCacheMixedStressRetiresState`, and
`TestRedisValueIntegration/pointer-isolation` prove the boundary at unit,
stress, race, and real-Redis levels.

### Future Guard

Future RESP3 work calls only `InvalidateLocal` or `ClearLocal`; it never routes
invalidation events through `Set`, `Delete`, or `Clear`, because those methods
mutate L2.

## L2: Redacted errors need explicit debug and structured-log contracts

### Problem

Reviewing only `Error()` leaves debug formatting and structured logging as
implicit behavior, even when the wrapped cause intentionally remains reachable
through `errors.Is` and `errors.As`.

### Decision

`CacheError` now implements redacted `GoString` and `slog.LogValuer` contracts.
Tests cover provider, serializer, partial-clear, and joined cleanup failures
across `%v`, `%+v`, `%#v`, and structured values. Nested partial-clear progress
also remains visible when an outer local-blocked error joins cleanup failure.

### Future Guard

Any new public error that retains a raw provider cause must test ordinary,
debug, and structured-log formatting separately from causal inspection.

## L3: A green race run does not replace the approved concurrency matrix

### Problem

The initial stress test proved race freedom and cleanup, but it did not prove
every generation-fence and mutation-order acceptance criterion from the spec.
The first Step 6-R stability lane caught that evidence gap.

### Decision

Deterministic latch tests now cover delayed refill against same-key mutation,
`ClearLocal`, blocked readers and token waiters, loader completion, namespace
clear, and admitted delete. Repeated same-key waves assert one loader per wave.
Real Redis tests cover dispatch-time cancellation cleanup and provider failure
through blocked-state repair.

### Future Guard

Before final review, map every spec concurrency bullet to a named test and
assert exact side-effect totals; `go test -race` is supporting evidence, not a
substitute for that traceability.
