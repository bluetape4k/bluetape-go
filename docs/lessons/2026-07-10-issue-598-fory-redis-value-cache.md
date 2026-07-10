# Issue #598 Lesson: A Payload Bound Must Start At The Redis Read

## Context

A direct Redis value cache can validate its binary envelope perfectly and still
fail its resource-bound promise if it first downloads the entire stored value.
Bad rollout data, a stale writer, manual corruption, or a compromised peer can
place a value larger than the configured Fory payload limit under a valid key.

## Learning

Enforce the bound at the earliest controllable allocation. Read only the
envelope header, configured payload, and one overflow-detection byte from Redis;
reject overflow before Fory sees the value. Because `GETRANGE` returns an empty
string for both a missing key and an existing empty value, pair it with an
existence check so `cache.ErrCacheMiss` continues to mean absent or expired,
while empty corrupt data still fails envelope validation.

Operational documentation is part of this contract. Replacing `GET` with
`GETRANGE` and `EXISTS` changes the least-privilege ACL surface even when the Go
API is unchanged. Keep an integration test that authenticates as a restricted
Redis user and proves the exact documented command set supports the lifecycle.

Wire-format independence also matters. Shared Fory runtime code can remove
duplicate locking, registration, panic, and bounds logic, while public packages
retain distinct `BTFV` and `BTFY` envelopes for different storage semantics.

## Durable Checks

- Recheck cancellation immediately before every external side effect.
- Bound network response materialization before decode or decompression.
- Preserve the distinction between missing, empty-corrupt, and oversized data.
- Treat namespace, profile, registration names, schema generation, limits, and
  Redis ACL commands as one rollout contract.
- Replace provider causes with sanitized categories; classify infrastructure
  failures at caller-owned hooks without logging raw provider text.
- Use explicit Redis readiness polling and run shared Testcontainers packages
  serially when local Docker resources are constrained.
- Keep performance claims in #599 until raw output, environment/revision
  metadata, a table, a Chart, and written analysis exist.
