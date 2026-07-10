# Lessons Learned - Redis Cache Coordinator Substrate Migration (2026-07-10)

**Related issue:** #588
**Affected module:** `cache/rediscoord`

## L1: Reuse a safety primitive only when its input contract is compatible

### Problem

The shared `redis.KeyBuilder` validates package-owned structural segments and
`redis.OwnerToken` accepts canonical values. `cache/rediscoord` intentionally
preserves caller namespaces/keys verbatim and compares short-lived result
envelope tokens as opaque historical values.

### Decision

Reuse only `redis.OpError` for direct provider diagnostics. Keep key layout,
duration normalization, envelope token handling, and the migrated `lock/redis`
lease boundary local.

### Evidence

- `cache/rediscoord/operation_error_test.go`: redacted failures, typed causes,
  late-context cause joining, key-byte preservation, and opaque token coverage.
- `TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make ci`

### Future Guard

For every remaining #570 slice, compare public key/token/TTL/error inputs with
the shared helper before adopting it. A helper that rejects an established
caller value is a compatibility boundary, not a refactoring opportunity.

## L2: Local Testcontainers verification must override stale reuse settings

### Problem

The machine-level Testcontainers configuration enabled reuse and disabled Ryuk,
allowing old provider containers to remain alive and intermittently corrupt
port-mapped integration runs.

### Decision

For repository-wide local verification, use explicit non-reuse plus cleanup
environment values until the machine-level setting is repaired deliberately.

### Future Guard

When unrelated Redis, PostgreSQL, and NATS tests fail with mixed connection
resets, EOFs, or timeouts, inspect labeled stale containers before changing
application code. Re-run the full gate with
`TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false`.
