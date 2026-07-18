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
