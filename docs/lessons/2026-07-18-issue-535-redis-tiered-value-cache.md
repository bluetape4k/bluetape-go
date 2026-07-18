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

## L4: Admission and publication need separate fence proofs

### Problem

Pausing only inside a loader or provider callback proves in-flight cleanup, but
does not isolate the boundary after a side-effect ticket is issued and before
the admitted loader, `SET`, or `DEL` is invoked.

### Decision

A deterministic local-state seam now issues a one-shot ticket, transitions the
decorator out of its generation, and then proves that the already admitted
side effect runs exactly once while its result cannot publish into L1.

### Future Guard

When a state machine separates admission from effect execution, test the
ticket and the later publication classification independently; callback latches
alone do not prove both boundaries.

## L5: Option names do not explain an operational budget

### Problem

Listing timeout fields and ACL commands did not tell operators how to size the
budgets, which Redis/TLS baseline to require, or how to alert and recover from a
blocked local tier.

### Decision

Both package READMEs now bind invalidation and cleanup timeouts to the work they
must cover, require Redis 6+ with verified TLS certificates when enabled,
reject logical databases as security boundaries, and make blocked-state alert
and explicit recovery part of the executable documentation parity contract.

### Future Guard

Operational documentation tests should preserve decision rules and recovery
actions, not only option and command names.

## L6: Public examples must model the failure policy

### Problem

A compile-checked example can still teach unsafe behavior when it discards
mutation or repair errors, or demonstrates namespace clear with an ordinary
client identity.

### Decision

The example checks every mutation and invalidation result, repairs blocked
state only with a fresh bounded context, and constructs namespace clear with a
separately credentialed admin client. Migration guidance also states that
`ValueCache` adoption is incremental and does not rewrite `redisfory` data.

### Future Guard

Review examples as caller code, not merely as compilable syntax: error handling,
credentials, recovery contexts, and migration boundaries must match production
guidance.

The example's separate clear-admin client also names its credential inputs;
a second client instance without an explicit identity does not demonstrate ACL
separation.

## L7: Bounded reads still need one consistency point

### Problem

`GETRANGE` bounded payload admission, but an empty result required a later
`EXISTS` to distinguish a missing key from a valid empty payload. Another
client could create or delete the key between those commands, combining bytes
from one Redis point in time with existence from another and fabricating an
empty cache hit.

### Decision

Keep the single-command path for non-empty payloads. When the first bounded
read is empty, re-run bounded `GETRANGE` and `EXISTS` inside one `MULTI`/`EXEC`
transaction. The ordinary Redis identity therefore includes transaction
commands, and a deterministic two-client integration test fixes the exact
interleaving that exposed the defect.

### Future Guard

When absence and an empty value are both meaningful, never combine a payload
probe and an existence probe from different backend snapshots. Use one atomic
transaction/script or a deliberately versioned non-empty envelope, and test a
cross-client mutation between the original probes.
