// Package redisvalue provides a bounded serialized Redis L2 value cache and a
// process-local tiered cache decorator.
//
// ValueCache serializes values only at the Redis boundary. TieredCache stores
// values directly in its caller-owned L1; it does not provide cross-process L1
// coherence. RESP3 invalidation tracking is intentionally outside this package.
package redisvalue
