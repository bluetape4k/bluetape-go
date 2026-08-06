# redisvalue

[English](README.md) | [한국어](README.ko.md)

`redisvalue` provides a bounded serialized Redis L2 and an optional
process-local L1 decorator. It is cache-aside infrastructure, not a coherent
multi-process near cache.

<!-- redisvalue-contract: l1-boundary -->
## L1 and L2 boundary

`ValueCache[V]` serializes values into Redis. `TieredCache[V]` stores `V`
directly in L1; healthy L1 hits do not call Redis or the serializer. Pointer
identity is therefore process-local: separate cold decorators decode separate
objects from the same L2 bytes.

<!-- redisvalue-contract: config -->
## Configuration

Copy `DefaultConfig()` and override either section per cache.

| Setting | Default | Meaning |
|---|---:|---|
| `Value.RemoteTTL` | `1h` | default Redis TTL; `0` persists |
| `Value.MaxValueBytes` | `1 MiB` | maximum admitted serialized bytes |
| `Value.ClearBatchSize` | `100` | `SCAN COUNT` hint and maximum `UNLINK` argument count |
| `Tiered.LocalTTL` | `30m` | L1 TTL ceiling |
| `Tiered.InvalidationWaitTimeout` | `30s` | public invalidation wait budget |
| `Tiered.LocalCleanupTimeout` | `1s` | mandatory/explicit L1 cleanup budget |

`Set` and `GetOrLoad` accept a per-entry Redis TTL; `SetDefault` and
`GetOrLoadDefault` use the copied `RemoteTTL`.

<!-- redisvalue-contract: ownership -->
## Ownership and references

The caller owns the direct `*redis.Client`, serializer, and L1 lifecycle.
`TieredCache` exclusively owns cache operations on the supplied L1. Do not
read or mutate that L1 through another path. Treat pointer-valued `V` as an
immutable snapshot while cached because L1 retains the original reference.

<!-- redisvalue-contract: l1-provenance -->
The supplied `Local` must be new or empty, or already contain values only for
the exact remote namespace, schema, and tenant. Never reuse or share one L1
across decorators: L1 keys are the caller's raw logical keys and are not
namespace-qualified internally.

<!-- redisvalue-contract: load-policy -->
## Read and load policy

Reads follow L1, then L2, then the first same-key leader's loader. Exact
`cache.ErrCacheMiss` advances to the next tier; provider, decode, and local
errors stop the operation. One process-local flight shares the first leader's
context, TTL, loader, value, and error. Cross-process stampede control is not
part of this decorator.

L2 uses one bounded `GETRANGE` for a non-empty payload. Because an empty payload
is valid, an empty first result is re-read with `GETRANGE` and `EXISTS` inside
one `MULTI`/`EXEC` transaction. This prevents a concurrent create or delete
from combining bytes and existence from different Redis points in time.

<!-- redisvalue-contract: ttl -->
## TTL semantics

Redis TTL `0` means no expiry; negative TTLs are rejected and positive values
below one millisecond are written with a one-millisecond wire minimum. A known
write shortens L1 TTL so it cannot outlive that write's L2 TTL. An L2 hit cannot
observe the key's existing remaining TTL atomically, so the refill has a
documented stale window bounded only by `LocalTTL`; choose a suitably short L1
TTL or disable this topology when that window is unacceptable.

<!-- redisvalue-contract: errors -->
## Errors and blocked recovery

Mutations are Redis-first. Once `SET` or `DEL` is invoked, a provider error or
late cancellation can be commit-unknown and triggers mandatory L1 cleanup.
If cleanup cannot be proved, the decorator enters `ReasonLocalBlocked` and
fails closed. Only a successful explicit `ClearLocal` heals that state. Public
errors are redacted and remain inspectable with `errors.Is`/`errors.As`.
The deterministic redacted key ID is a correlation pseudonym, not an
anonymization boundary: low-entropy keys can be dictionary-guessed. Keys must
not contain secrets or direct PII.

<!-- redisvalue-contract: clear -->
## Clear and fleet reset

`InvalidateLocal` removes one entry only from this decorator. `ClearLocal`
clears only this decorator and is the explicit repair operation.
`ValueCache.Clear` is an L2-only administrative namespace clear;
`TieredCache.Clear` performs it first and then clears this decorator's L1.
Redis clear is non-atomic `SCAN` plus bounded sequential `UNLINK`, so concurrent
writes may survive. For a fleet reset: quiesce writers, run L2 clear with the
clear-admin identity, run `ClearLocal` in every process or restart them, verify
the namespace, then resume traffic.

<!-- redisvalue-contract: topology -->
## Supported Redis topology

Use one stable direct writable-primary `*redis.Client`. Failover clients,
proxies with ambiguous routing, Redis Cluster, and Ring are unsupported because
the package's key, scan, mutation-order, and commit-unknown proofs target one
primary command domain.

<!-- redisvalue-contract: operations -->
## Operations and ACLs

Run Redis 6+ and connect directly to one stable writable primary. When TLS is
enabled, require server certificate verification. A Redis `SELECT` logical
database is not an ACL or security boundary; isolate trust domains with ACLs,
credentials, and network/TLS controls.

Ordinary identities need only `GETRANGE`, `EXISTS`, `MULTI`, `EXEC`, `SET`, and
`DEL` for their namespace. Use the exact key pattern
`bluetape:cache:value:<namespace>:*`. Construct a separate `ValueCache` with a
clear-admin client that has `SCAN` and namespace-scoped `UNLINK`; deny
`FLUSHDB` and `FLUSHALL`. ACL key patterns are not tenant isolation by
themselves—`SCAN` can expose foreign key names—so also use Redis network/TLS
isolation.

Set caller-owned go-redis `DialTimeout`, `ReadTimeout`, `WriteTimeout`, and
`PoolTimeout` for the service budget. A readiness check must prove the stable
writable primary command path, not only `PING`. Size Redis `maxmemory` and its
eviction policy for serialized payloads and TTL behavior. This package emits no
telemetry; install caller-owned go-redis hooks and monitor memory, eviction,
command latency/timeouts, provider reasons, blocked decorators, and
partial-clear progress. A partial clear restarts from cursor 0 because the
cursor is diagnostic, not resumable. `ClearProgress.ScannedKeys` means only
matching keys returned so far, never a total, percentage, or cursor.

Set `InvalidationWaitTimeout` above the expected same-key loader plus Redis
latency. Set `LocalCleanupTimeout` above the worst-case active lease drain plus
L1 delete or clear latency. Alert on `ReasonLocalBlocked`; recovery requires a
successful explicit `ClearLocal`, not an automatic retry.

<!-- redisvalue-contract: versioning -->
## Versioning and rollout

Prefer `serialization.VersionedSerializer`. For an incompatible wire change,
rotate namespaces. A rollout that reuses a namespace must prove the exact
reader/writer matrix for upgrade and rollback and retain old readers through
the rollback window plus the maximum finite Redis TTL. Persistent TTL `0` data
requires an explicit administrative cleanup plan before removing compatibility.

Adoption is incremental: callers may use `ValueCache` alone and add
`TieredCache` only when a process-local reference L1 is appropriate. Existing
`cache/redisfory` users retain their current wire format; this package performs
no implicit migration.

<!-- redisvalue-contract: resp3 -->
## RESP3 and adjacent cache packages

Do not wrap this decorator with the current Pub/Sub `cache/redisnear`; both
would own local invalidation and still would not make L1 coherent. Keep
`cache/rediscoord` loader ownership separate from this decorator's local
same-key flight. Issue #536 is the public RESP3 client-tracking capability and
correctness proof gate for a future coherent near-cache mode; RESP3 is not
required by or provided in #535.

<!-- redisvalue-contract: tests -->
## Tests

Run ordinary and Testcontainers tests serially when Docker resources are shared:

```bash
go test -p 1 -count=1 ./cache/redisvalue
go test -race -p 1 -count=1 ./cache/redisvalue
```

<!-- redisvalue-contract: untrusted-payload -->
## Untrusted payloads

Treat Redis bytes as untrusted. Use serializers that avoid executable
deserialization and return errors, not panics, for malformed input. The
serializer must bound temporary allocations, nesting/recursion, decompression,
and CPU work. `MaxValueBytes` bounds Redis admission bytes, not decoder work.

<!-- redisvalue-contract: authentication -->
## Payload authentication

Tamper-sensitive deployments must add an authenticated envelope around the
payload in addition to `VersionedSerializer`. Built-in versioning detects
compatibility and format mismatches; it does not detect malicious modification.

<!-- redisvalue-contract: namespace -->
## Namespace trust domain

A namespace is one exclusive tenant, schema, and clear trust domain. It is not
an authorization boundary. Incompatible tenants or wire formats require
separate namespaces plus Redis ACL and network isolation.

<!-- redisvalue-contract: scan-bounds -->
## SCAN bounds

`SCAN COUNT` is a hint. The client retains one Redis-controlled page and splits
it into `ClearBatchSize` `UNLINK` argument chunks, but it cannot bound the byte
size of a returned page or keys outside this package's control.

<!-- redisvalue-contract: serializer-concurrency -->
## Serializer concurrency

The serializer is caller-owned, immutable after construction, and safe for
concurrent `Marshal` and `Unmarshal`. The package retains it without cloning and
does not serialize calls behind a global lock.

<!-- redisvalue-contract: compatibility-matrix -->
## Compatibility matrix

The built-in versioned envelope is backward-readable only: a version-2 reader
accepts version-1 bytes, while a version-1 reader rejects version-2 bytes with
`serialization.ErrUnsupportedVersion`. Reusing a namespace across an
upgrade/rollback window is prohibited until the application proves its exact
bidirectional serializer matrix.
