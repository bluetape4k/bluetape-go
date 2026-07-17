# Issue #535 Redis Tiered Value Cache Design

Status: approved and self-reviewed design, awaiting Step 2-R review  
Issue: [#535](https://github.com/bluetape4k/bluetape-go/issues/535)  
Related RESP3 spike: [#536](https://github.com/bluetape4k/bluetape-go/issues/536)  
Target package: `cache/redisvalue`

## Problem

`bluetape-go` has a process-local `cache.LoadingCache`, Redis Pub/Sub
invalidation in `cache/redisnear`, cross-process load coordination in
`cache/rediscoord`, and a Fory-specific direct Redis cache in
`cache/redisfory`. It does not have a generic serialized Redis L2 value cache
or a small decorator that combines such an L2 with a caller-owned L1.

Callers currently have to repeat several sensitive decisions:

- where serialization occurs;
- how L1 and L2 TTLs relate;
- how a miss moves through L1, L2, and a loader;
- what happens when a Redis mutation may have committed despite an error;
- how namespace clear avoids `FLUSHDB`;
- how future Redis-native invalidation can evict L1 without deleting L2.

The new package must make these decisions explicit without hiding a global
Redis value store, replacing `cache.LoadingCache`, or prematurely exposing a
RESP3 production API before issue #536 proves its lifecycle.

## Goals

1. Provide a generic `ValueCache[V]` that stores serialized values in Redis.
2. Provide a `TieredCache[V]` decorator that combines a caller-owned
   `cache.LoadingCache[string,V]` L1 with `ValueCache[V]` L2.
3. Store `V` itself in L1 and serialize only at the L2 boundary.
4. Provide safe defaults that callers copy and override per cache.
5. Preserve `cache.Cache[string,V]` and `cache.LoadingCache[string,V]`
   compatibility, including per-entry TTL overrides.
6. Bound payload reads, diagnostics, namespace clearing, and cancellation
   behavior.
7. Leave a deliberate L1-only invalidation boundary for the RESP3 spike in
   #536.

## Non-goals

- Implementing or publishing RESP3 `CLIENT TRACKING` in #535.
- Replacing the existing `cache.LoadingCache` contract.
- Adding a package-global Redis client, cache registry, or mutable default
  configuration.
- Adding implicit background refresh, negative caching, write-behind, or
  fallback-to-local-on-Redis-error modes.
- Adding distributed singleflight; `cache/rediscoord` remains responsible for
  cross-process load coordination.
- Adding CAS, conditional replace, bulk `MGET`/`MSET`, or write batching.
- Publishing benchmark conclusions; milestone issue #560 owns the benchmark
  matrix.
- Claiming that `TieredCache` alone is a coherent multi-process near cache.
  That claim requires a passing invalidation strategy such as the #536 RESP3
  spike or the existing explicit Pub/Sub protocol.

## Current Evidence

### Go anchors

- `cache.Cache` and `cache.LoadingCache` define context-aware operations,
  per-entry TTL, and caller-supplied loaders.
- `cache.Memory` stores `V` directly and collapses same-key `GetOrLoad` calls
  within one process.
- `serialization.Serializer[V]` already defines `Marshal` and `Unmarshal`;
  the new package must not introduce a competing codec interface.
- `serialization.VersionedSerializer[V]` is the recommended compatibility
  wrapper when serialized values must survive application deployments.
- `redis.KeyBuilder`, `redis.OpError`, `redis.ErrCommitUnknown`, and Redis TTL
  validation are the shared key, diagnostic, ambiguity, and validation
  primitives.
- `cache/redisfory` proves caller-owned Redis clients, bounded reads, redacted
  errors, and direct `Get`/`Set`/`Delete` behavior, but remains intentionally
  Fory-specific.
- `cache/redisnear` owns explicit Pub/Sub invalidation; `cache/rediscoord` owns
  cross-process stampede coordination. Neither is a serialized L2 store.

### `bluetape4k-cache-lettuce` parity

The JVM reference keeps Caffeine objects in L1 and uses a Lettuce
`RedisCodec` only for Redis L2. L1 hits avoid serialization. L1 misses read and
decode Redis, then place the decoded object into L1. Writes complete Redis
first and update L1 afterward. L1 and L2 TTLs are independent, with guidance
that L1 should expire no later than L2.

Its RESP3 mode uses `CLIENT TRACKING ON NOLOOP` so writes from another
connection invalidate L1. Tracking has materially different connection,
reconnect, and push-message lifecycle from ordinary cache commands. Go keeps
that lifecycle in #536 rather than copying the integrated JVM type into #535.

## Selective Parity Decision

| JVM behavior | Go decision | Rationale |
|---|---|---|
| L1 stores decoded objects | Keep | `cache.Memory` already stores `V` directly and preserves pointer identity within one process. |
| L2 uses a Redis codec | Adapt | Reuse `serialization.Serializer[V]` instead of introducing a Lettuce-shaped codec. |
| One integrated L1/L2 type | Adapt | Expose a narrow `ValueCache[V]` plus a `TieredCache[V]` decorator around caller-owned L1. |
| Redis-first write-through | Keep | Avoid leaving a new value only in L1 when the L2 write fails. |
| Separate L1 and L2 TTL | Keep | Defaults and per-entry overrides preserve `L1 TTL <= L2 TTL` when L2 expires. |
| RESP3 tracking in the same type | Split | #535 establishes L1 invalidation hooks; #536 proves or rejects the `cache/redisnear` strategy. |
| Tracking startup fail-open | Reject | Silent loss of invalidation can serve stale L1 entries; future tracking must fail explicitly or clear L1. |
| Write-behind and resilient fallback modes | Reject | They add queues, retry ownership, shutdown, and data-loss ambiguity outside #535. |
| Bulk operations and Lua CAS | Defer | No issue acceptance criterion or repeated Go call site justifies the surface yet. |
| `SCAN` plus `UNLINK` namespace clear | Adapt | Required for `cache.Cache.Clear`, but documented as non-atomic operational work, not a hot-path command. |

## Chosen Architecture

```text
application
    |
    v
TieredCache[V] ----------------------+
    |                                |
    | L1 hit                         | L1 miss
    v                                v
cache.LoadingCache[string,V]     ValueCache[V]
  stores V directly                 |
  no serialization                  | Marshal / Unmarshal
                                     v
                               Redis byte value
```

The package has two independently useful public types:

1. `ValueCache[V]` is a serialized Redis L2 provider.
2. `TieredCache[V]` decorates a caller-owned local loading cache with that L2.

Neither type owns the caller's Redis client or local cache lifecycle. #535
does not create goroutines and therefore does not add a `Close` method.

## Configuration

```go
type Config struct {
    LocalTTL       time.Duration
    RemoteTTL      time.Duration
    MaxValueBytes  int
    ClearBatchSize int64
}

func DefaultConfig() Config
func (c Config) Validate() error
```

`DefaultConfig` returns:

```go
Config{
    LocalTTL:       30 * time.Minute,
    RemoteTTL:      time.Hour,
    MaxValueBytes:  1 << 20,
    ClearBatchSize: 100,
}
```

Rules:

- `LocalTTL` must be positive.
- `RemoteTTL` must not be negative. Zero explicitly means no Redis expiry,
  matching the existing `cache.Cache` TTL convention.
- When `RemoteTTL` is positive, `LocalTTL` must not exceed it.
- `MaxValueBytes` and `ClearBatchSize` must be positive.
- A nil config uses `DefaultConfig()`.
- A non-nil config is copied during construction. Later caller mutation does
  not affect an existing cache.
- There is no mutable package default. Applications create a base value with
  `DefaultConfig`, copy it, and override fields for each cache.

The configured relationship reduces the stale window but is not an absolute
expiry-order guarantee. An L2 hit can occur near the end of the Redis key's
remaining TTL and then populate L1 for the effective local TTL. #535 does not
add a Redis `PTTL` round trip or a Lua read-with-expiry operation. Therefore a
local entry can outlive the corresponding Redis key for at most its local TTL
unless an invalidation strategy evicts it earlier.

`Namespace`, `Serializer`, `Client`, and `Local` are cache identity or owned
dependencies, not configuration defaults.

## Public API

The exact exported surface is:

```go
type ValueOptions[V any] struct {
    Client     redis.Cmdable
    Namespace  string
    Serializer serialization.Serializer[V]
    Config     *Config
}

type ValueCache[V any] struct { /* constructor-only */ }

func NewValueCache[V any](options ValueOptions[V]) (*ValueCache[V], error)

func (c *ValueCache[V]) Get(
    ctx context.Context,
    key string,
) (V, error)

func (c *ValueCache[V]) Set(
    ctx context.Context,
    key string,
    value V,
    ttl time.Duration,
) error

func (c *ValueCache[V]) SetDefault(
    ctx context.Context,
    key string,
    value V,
) error

func (c *ValueCache[V]) Delete(
    ctx context.Context,
    key string,
) error

func (c *ValueCache[V]) Clear(ctx context.Context) error
```

`ValueCache[V]` implements `cache.Cache[string,V]`. Its zero value is not
usable; every method returns an inspectable uninitialized error instead of
panicking.

```go
type TieredOptions[V any] struct {
    Local  cache.LoadingCache[string, V]
    Remote *ValueCache[V]
}

type TieredCache[V any] struct { /* constructor-only */ }

func NewTiered[V any](options TieredOptions[V]) (*TieredCache[V], error)

func (c *TieredCache[V]) Get(
    ctx context.Context,
    key string,
) (V, error)

func (c *TieredCache[V]) Set(
    ctx context.Context,
    key string,
    value V,
    remoteTTL time.Duration,
) error

func (c *TieredCache[V]) SetDefault(
    ctx context.Context,
    key string,
    value V,
) error

func (c *TieredCache[V]) Delete(
    ctx context.Context,
    key string,
) error

func (c *TieredCache[V]) Clear(ctx context.Context) error

func (c *TieredCache[V]) GetOrLoad(
    ctx context.Context,
    key string,
    remoteTTL time.Duration,
    loader cache.Loader[string, V],
) (V, error)

func (c *TieredCache[V]) GetOrLoadDefault(
    ctx context.Context,
    key string,
    loader cache.Loader[string, V],
) (V, error)

func (c *TieredCache[V]) InvalidateLocal(
    ctx context.Context,
    key string,
) error

func (c *TieredCache[V]) ClearLocal(ctx context.Context) error
```

`TieredCache[V]` implements `cache.LoadingCache[string,V]`. `SetDefault` and
`GetOrLoadDefault` use `Config.RemoteTTL`. The standard interface methods keep
per-entry TTL overrides.

## Key Contract

Redis keys use the stable form:

```text
bluetape:cache:value:<namespace segments>:<logical key>
```

- Structural namespace segments follow `redis.KeyBuilder` validation.
- The caller's logical key is preserved verbatim and collision-tested.
- Empty or whitespace-only logical keys are rejected.
- Diagnostics use the redacted key ID and never expose the raw physical or
  logical key by default.
- #535 locks the physical format in tests but does not expose a speculative
  public reverse-key mapper. #536 may add the minimum mapper required by a
  passing RESP3 spike.

## Serialization Boundary and Reference Semantics

Only `ValueCache` calls `Serializer.Marshal` or `Serializer.Unmarshal`.
`TieredCache` places `V` itself into L1.

If `V` is a pointer:

- repeated L1 hits return the same pointer while the local entry remains;
- an L2 hit creates a newly deserialized pointer, which then becomes that
  process's L1 object;
- different processes or different cold L1 instances do not share object
  identity;
- mutating the pointer returned from L1 mutates that process's cached object
  without updating L2.

Documentation therefore tells callers to treat pointer cache values as
immutable snapshots or call `Set` after mutation. The package does not clone
L1 values.

Empty serialized payloads are valid when the serializer accepts them. Redis
key absence is distinguished from an existing empty value; only absence maps
to `cache.ErrCacheMiss`.

## Read and Load Behavior

### `ValueCache.Get`

1. Validate initialization, context, and logical key.
2. Read no more than `MaxValueBytes + 1` bytes using a bounded Redis read.
3. Distinguish an absent key from an existing empty value.
4. Reject an oversized payload before deserialization.
5. Unmarshal exactly once and return the resulting `V`.

### `TieredCache.Get`

1. Read L1.
2. Return an L1 hit immediately without Redis or serializer calls.
3. Continue only when L1 returns `cache.ErrCacheMiss`; propagate other L1
   errors.
4. Read L2.
5. On L2 hit, store the decoded `V` in L1 for `Config.LocalTTL` and return it.
6. On L2 miss, return `cache.ErrCacheMiss`.

### `TieredCache.GetOrLoad`

`TieredCache` delegates same-key collapse to the caller-provided L1 by calling
`Local.GetOrLoad` with a loader closure. Inside that closure:

1. Read L2.
2. Return the L2 value on hit; the L1 implementation stores that same `V`.
3. Continue only on `cache.ErrCacheMiss`.
4. Run the caller loader once for the collapsed local flight.
5. Store the loaded value in L2.
6. Return it only after L2 succeeds, allowing L1 to store it.

The effective L1 TTL is:

```text
remote TTL > 0: min(Config.LocalTTL, remote TTL)
remote TTL = 0: Config.LocalTTL
```

A loader error is returned unchanged and is not cached. A Redis or
serialization error is not converted into a miss and does not fall through to
the loader.

This collapse is process-local. Two processes can still execute the loader at
the same time unless the caller deliberately composes `cache/rediscoord`.

## Mutation Behavior

### `Set`

1. Validate and serialize the value.
2. Write L2 with the requested remote TTL.
3. After L2 success, write the original `V` into L1 with the effective L1
   TTL.

The package does not serialize or clone the L1 value. It does not update L1
before Redis succeeds.

If the Redis mutation fails after dispatch, the error preserves
`redis.ErrCommitUnknown`. `TieredCache` attempts to invalidate L1 before
returning because Redis may contain the new value. If L1 population fails
after a successful L2 write, `TieredCache` invalidates L1 and returns the L1
error; the next read can recover from L2.

### `Delete`

`ValueCache.Delete` removes the L2 key and treats an absent key as success.
`TieredCache.Delete` attempts the L2 delete and always invalidates the L1 key
before returning. Errors are joined without hiding the Redis mutation outcome
or L1 failure.

### `Clear`

`ValueCache.Clear` scans only its namespace and sends `UNLINK` in batches of
`ClearBatchSize`. It never calls `FLUSHDB`.

`TieredCache.Clear` attempts remote clear and always clears L1. It returns all
relevant errors without silently declaring success.

Namespace clear is:

- context-cancellable;
- idempotent for already removed keys;
- non-atomic and not a snapshot;
- proportional to namespace size;
- intended for administration, tests, and cache reset, not request hot paths.

Keys added concurrently can escape a scan iteration, and callers must not use
`Clear` as a transaction or authorization boundary.

## Error Contract

The package defines an inspectable `CacheError` with operation and `Reason`
while preserving causes for `errors.Is` and `errors.As`:

```go
type Reason string

const (
    ReasonConfiguration  Reason = "configuration"
    ReasonUninitialized  Reason = "uninitialized"
    ReasonSerialization  Reason = "serialization"
    ReasonPayloadTooLarge Reason = "payload-too-large"
    ReasonInvalidPayload Reason = "invalid-payload"
    ReasonLocalFailure   Reason = "local-failure"
    ReasonProviderFailure Reason = "provider-failure"
    ReasonPartialClear   Reason = "partial-clear"
)

type CacheError struct { /* redacted fields */ }

func (e *CacheError) Error() string
func (e *CacheError) Unwrap() error
func (e *CacheError) Operation() string
func (e *CacheError) Reason() Reason
```

Reasons cover:

- configuration;
- uninitialized zero value;
- serialization;
- payload too large;
- invalid payload;
- local cache failure;
- Redis provider failure;
- a namespace clear that stopped after partial progress.

Rules:

- `cache.ErrCacheMiss` means only a missing cache entry.
- Serializer errors are wrapped and do not issue a Redis mutation.
- Redis errors use redacted `redis.OpError` labels for the `redisvalue`
  family.
- Mutation ambiguity preserves `redis.ErrCommitUnknown`.
- Raw values, serialized bytes, Redis keys, namespace contents, and provider
  diagnostics are not logged by the package.
- The package owns no logger; callers observe returned errors.
- Joined cleanup or L1 invalidation errors preserve each inspectable cause.
- A failed `Clear` reports partial progress without enumerating raw keys and is
  safe to retry because `UNLINK` of already removed keys is idempotent.

## Context and Cancellation

- Nil contexts follow the repository convention and normalize to
  `context.Background()`.
- A canceled context is checked before serialization and before Redis
  dispatch.
- Caller cancellation and deadlines are never retried.
- Read cancellation must not populate L1 after the caller has returned.
- A loader result observed after cancellation must not be written to L2 or L1.
- A mutation error after dispatch may still be commit-unknown; cancellation
  does not prove Redis rejected the operation.
- `Clear` checks cancellation between scan/unlink batches.
- No method starts a background goroutine, retains a waiter, or outlives a
  completed operation by design.

## RESP3 and Near-cache Boundary

`TieredCache` is a two-tier cache with a local front. Without invalidation,
another process can change Redis while this process continues serving its L1
entry until local expiry. The package does not market #535 alone as a coherent
multi-process near cache.

Issue #536 owns the proof required before a public RESP3 strategy exists:

- enforce RESP3 negotiation;
- use a pinned connection for the reads being tracked;
- prove that the bounded read command used by `ValueCache.Get` registers the
  key for tracking;
- parse invalidation pushes without racing ordinary responses;
- invalidate through `TieredCache.InvalidateLocal` without deleting L2;
- flush L1 on disconnect or reconnect ambiguity;
- re-enable tracking before tracked local hits resume;
- define `NOLOOP`, external-writer, shutdown, provider, and proxy behavior.

The future strategy remains constructor-separated in `cache/redisnear`; it is
not a `Strategy` enum hidden inside #535 options. If the spike fails, #535
remains a useful explicit tiered cache with bounded stale time, and Pub/Sub
remains the supported invalidation mechanism for writers participating in its
protocol.

## Failure Modes and Required Response

| Failure mode | Required response |
|---|---|
| Serializer rejects a value | Return an inspectable serialization error; do not call Redis or update L1. |
| Redis read returns malformed or oversized bytes | Return an invalid/oversized payload error; do not treat it as a miss or auto-delete it. |
| Redis `SET` may have committed before timeout/cancellation | Return `redis.ErrCommitUnknown` and invalidate L1. |
| L1 update fails after L2 success | Invalidate L1, return the L1 error, and leave L2 as the recovery source. |
| Loader succeeds but L2 write fails | Return the L2 error and do not populate L1. |
| L1 TTL is configured above the finite default L2 TTL | Reject construction; per-entry overrides cap the effective local TTL. |
| L1 is filled near the end of an existing Redis TTL | Permit a stale window bounded by the effective local TTL; document that only invalidation closes it earlier. |
| Namespace clear is interrupted | Return cancellation plus any operation error; L1 is still cleared by `TieredCache.Clear`. |
| Future tracking connection becomes ambiguous | #536 must clear L1 and re-establish tracking before serving tracked local hits. |

## Testing Strategy

### Unit tests with fakes

- default config, copied per-cache override, validation, and mutation isolation;
- nil dependencies and safe constructor-only zero values;
- logical-key preservation, namespace collision resistance, and redaction;
- L1 hit performs no Redis or serializer call;
- L1 pointer hit returns the same pointer;
- L2 hit unmarshals once and puts the resulting `V` into L1;
- separate cold TieredCache instances deserialize distinct pointers;
- L2 miss returns `cache.ErrCacheMiss`;
- `GetOrLoad` follows L1 -> L2 -> loader and collapses same-key local calls;
- loader and serializer failures are not cached;
- Redis-first `Set` ordering and original-reference L1 population;
- finite, zero, negative, default, and per-entry TTL behavior;
- bounded and empty payload behavior;
- commit-unknown and local invalidation behavior;
- `Delete`, `ClearLocal`, `InvalidateLocal`, and namespace `Clear` error joining;
- cancellation before dispatch, during load, after load, and between clear
  batches, with no late L1/L2 write.

### Redis Testcontainers

- set/get/delete/miss against a real Redis service;
- finite TTL expiration and explicit zero-TTL persistence;
- finite default/per-entry TTL validation and the documented bounded stale
  window when L1 is filled near Redis expiry;
- namespace isolation and `SCAN` plus `UNLINK` clear;
- maximum payload boundary and oversized-value rejection;
- existing empty payload versus missing key;
- cancellation/failure behavior with connection readiness proved;
- two TieredCache instances sharing L2 bytes but not pointer identity.

Testcontainers commands run sequentially.

### Concurrency and race

- bounded same-key `GetOrLoad` stress with exact loader-call totals;
- concurrent different-key loads;
- concurrent `Get`, `Set`, `Delete`, `ClearLocal`, and cancellation without
  races, retained flights, or late local writes;
- `go test -race -count=1 ./cache/redisvalue`.

### Public examples

Compile-checked examples cover:

- `DefaultConfig` and per-cache override;
- `ValueCache` with an existing serializer;
- `TieredCache` with `cache.NewMemory`;
- default and per-entry TTL methods;
- pointer snapshot semantics;
- `ClearLocal` versus namespace `Clear`;
- recommended `VersionedSerializer` use.

## Documentation

Add synchronized `cache/redisvalue/README.md` and `README.ko.md` documenting:

- L1 object/reference and L2 serialization boundaries;
- defaults and per-cache override;
- strict Redis-first mutation behavior;
- pointer immutability guidance;
- TTL and bounded-staleness behavior;
- operational cost and non-atomic semantics of `Clear`;
- cancellation and commit-unknown behavior;
- `cache/redisnear` as invalidation, `cache/rediscoord` as load coordination,
  and `cache/redisfory` as the Fory-specific direct store;
- #536 as the RESP3 proof gate before a coherent Redis-native near-cache API.

Update root README package indexes and changelog only when implementation makes
the package public. A new diagram is not required for #535; the data flow is
small enough for text and compile-checked examples.

## Compatibility and Migration

- No existing package or interface changes are required.
- `TieredCache` implements the current loading-cache interface rather than
  replacing it.
- Callers can adopt only `ValueCache` or opt into the decorator.
- Callers that already use `cache/redisfory` keep its wire format and do not
  migrate implicitly.
- Wire compatibility is serializer-owned. Long-lived caches should use
  `serialization.VersionedSerializer` or rotate namespaces for incompatible
  schema changes.
- The physical key format is new and must remain stable after release unless a
  documented migration or namespace generation is introduced.

## Acceptance Criteria

1. `cache/redisvalue` exposes constructor-only, zero-value-safe
   `ValueCache[V]` and `TieredCache[V]` types.
2. `ValueCache[V]` implements `cache.Cache[string,V]` using caller-owned Redis
   and `serialization.Serializer[V]`.
3. `TieredCache[V]` implements `cache.LoadingCache[string,V]`, stores `V`
   directly in L1, and serializes only through L2.
4. `DefaultConfig` is valid, copied on construction, and can be overridden per
   cache without global mutable state.
5. L1 hits make no Redis or serializer call; L2 hits populate the same decoded
   `V` into L1.
6. Loads follow L1 -> L2 -> loader, collapse within the local cache, and write
   L2 before L1 population.
7. TTL defaults and overrides satisfy the documented finite/zero relationship,
   and tests do not claim that configured duration ordering is an atomic Redis
   expiry-order guarantee.
8. Payload reads, namespace clear, keys, errors, and cancellation are bounded
   and redacted as specified.
9. Mutation ambiguity preserves `redis.ErrCommitUnknown` and invalidates L1.
10. `InvalidateLocal` and `ClearLocal` never mutate Redis and are suitable for
    the #536 spike boundary.
11. Unit, Testcontainers, race, stress, and compile-checked example evidence
    passes.
12. English and Korean package documentation stay synchronized and clearly
    distinguish a two-tier cache from a RESP3-coherent near cache.

## Definition of Done

- The approved spec and implementation plan are committed before code.
- Step 2-R and Step 3-R seven-perspective reviews converge at P0=0/P1=0.
- Implementation follows test-first RED/GREEN evidence.
- Fresh targeted, race, stress, Testcontainers, example, lint, vet, tidy, and
  repository CI checks pass as triggered.
- Public docs, root indexes, and changelog agree with the implemented API.
- A Type A lesson is committed before PR creation.
- Final pre-PR and PR reviews converge at P0=0/P1=0.
- PR creation and merge follow their separate authority gates; no auto-merge is
  enabled.

## Step 2 Checklist

| Item | Status | Evidence |
|---|---|---|
| Worktree isolated | Done | `feat/issue-535-redis-l2-value-cache` at `3684299`, aligned with `origin/develop` before spec mutation. |
| Live requirements checked | Done | Issue #535 remains open in milestone `0.19.0`; issue #536 remains the separate RESP3 spike. |
| Current Go patterns inspected | Done | `cache`, `serialization`, `redis`, `redisfory`, `redisnear`, and `rediscoord` anchors. |
| JVM parity classified | Done | `bluetape4k-cache-lettuce` keep/adapt/split/reject/defer table above. |
| Alternatives presented | Done | L2-only, integrated near cache, and TieredCache decorator compared; user selected the decorator. |
| Architecture approved | Done | User approved L1 reference/L2 serialization, config overrides, strict errors, clear, and #535/#536 split. |
| Spec self-review | Done | No placeholders; clarified remaining-TTL stale windows, public error shape, and partial-clear behavior. |
| Step 2-R review | Pending | Six independent perspectives plus main-session integration. |
