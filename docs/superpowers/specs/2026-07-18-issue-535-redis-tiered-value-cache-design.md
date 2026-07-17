# Issue #535 Redis Tiered Value Cache Design

Status: approved design, repairing initial Step 2-R findings
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
   `cache.Cache[string,V]` L1 with `ValueCache[V]` L2.
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
| Separate L1 and L2 TTL | Keep | Configured durations keep the local default no greater than the remote default; existing L2 hits still have a documented remaining-TTL stale window. |
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
cache.Cache[string,V]            ValueCache[V]
  stores V directly                 |
  no serialization                  | Marshal / Unmarshal
                                     v
                               Redis byte value
```

The package has two independently useful public types:

1. `ValueCache[V]` is a serialized Redis L2 provider.
2. `TieredCache[V]` decorates a caller-owned local cache with that L2 and owns
   its own context-aware same-key loading coordination.

Neither type owns the caller's Redis client or local cache lifecycle. #535
does not create goroutines and therefore does not add a `Close` method.

## Configuration

```go
type ValueConfig struct {
    RemoteTTL      time.Duration
    MaxValueBytes  int
    ClearBatchSize int64
}

type TieredConfig struct {
    LocalTTL           time.Duration
    LocalCleanupTimeout time.Duration
}

type Config struct {
    Value  ValueConfig
    Tiered TieredConfig
}

func DefaultConfig() Config
func (c Config) Validate() error
```

`DefaultConfig` returns:

```go
Config{
    Value: ValueConfig{
        RemoteTTL:      time.Hour,
        MaxValueBytes:  1 << 20,
        ClearBatchSize: 100,
    },
    Tiered: TieredConfig{
        LocalTTL:            30 * time.Minute,
        LocalCleanupTimeout: time.Second,
    },
}
```

Rules:

- `TieredConfig.LocalTTL` and `LocalCleanupTimeout` must be positive.
- `RemoteTTL` must not be negative. Zero explicitly means no Redis expiry,
  matching the existing `cache.Cache` TTL convention.
- `Config.Validate` and `NewTieredCache` reject a positive configured
  `RemoteTTL` when `LocalTTL` exceeds it. `NewTieredCache` reads that immutable
  remote default from its `ValueCache`.
- `MaxValueBytes` must be in `[1, 64 MiB]`; this makes the bounded
  `MaxValueBytes + 1` read and its `int64` conversion overflow-safe.
- `ClearBatchSize` must be in `[1, 1000]`.
- A nil value or tiered config uses the matching field from `DefaultConfig()`.
- A non-nil config is copied during construction. Later caller mutation does
  not affect an existing cache.
- There is no mutable package default. Applications create a base value with
  `DefaultConfig`, copy it, and override `Value` and `Tiered` fields for each
  cache.
- `ValueCache` validates only `ValueConfig`. `TieredCache` separately copies
  and validates `TieredConfig` plus its relationship to the remote default, so
  two decorators may share one L2 while using different valid local TTLs.

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
    Client     *redis.Client
    Namespace  string
    Serializer serialization.Serializer[V]
    Config     *ValueConfig
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
    Local  cache.Cache[string, V]
    Remote *ValueCache[V]
    Config *TieredConfig
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

`TieredCache[V]` implements `cache.LoadingCache[string,V]` but only requires a
`cache.Cache[string,V]` L1. It owns same-key load coordination rather than
depending on an opaque L1's optional `GetOrLoad` behavior. `SetDefault` and
`GetOrLoadDefault` use the remote value cache's `ValueConfig.RemoteTTL`; the
standard interface methods keep per-entry TTL overrides.

`InvalidateLocal` acquires the same-key gate and calls only `Local.Delete`
inside the shared local barrier. `ClearLocal` takes the exclusive local
barrier, increments the generation, and calls only `Local.Clear`. Both derive
a context capped by `LocalCleanupTimeout` from the normalized caller context.
Failure or timeout transitions the decorator to blocked state; successful
`InvalidateLocal` never unblocks it, while successful `ClearLocal` is the one
explicit full repair that unblocks it. Neither method invokes Redis.

## Key Contract

Redis keys use the stable form:

```text
bluetape:cache:value:<namespace>:<logical key>
```

- `Namespace` is exactly one ASCII segment matching `[A-Za-z0-9._-]+` and is
  at most 128 bytes. Colons, whitespace, Redis glob metacharacters (`*`, `?`,
  `[`, `]`), and the escape character (`\`) are rejected.
- A one-segment namespace prevents `namespace=a, key=b:c` from colliding with
  `namespace=a:b, key=c` and makes the clear prefix unambiguous.
- The caller's logical key is preserved verbatim and collision-tested.
- Logical keys must be between 1 and 1024 bytes; whitespace-only keys are
  rejected. Accepted leading/trailing spaces, colons, and other bytes are
  preserved exactly.
- Diagnostics use the redacted key ID and never expose the raw physical or
  logical key by default.
- #535 locks the physical format in tests but does not expose a speculative
  public reverse-key mapper. #536 may add the minimum mapper required by a
  passing RESP3 spike.

The public client type is the concrete go-redis `*redis.Client`, which issues
commands synchronously and makes pipelines, transactional queues,
`*redis.ClusterClient`, `*redis.Ring`, and opaque `Cmdable` wrappers
unrepresentable at the constructor boundary. #535 supports a single writable
Redis primary. `redis.NewFailoverClient` is supported because it also returns a
`*redis.Client`. Cluster-wide clear requires per-primary scanning and slot-safe
deletion and is deferred rather than silently clearing one node. Production
construction rejects a nil client; package-internal narrow command interfaces
remain replaceable by fakes in unit tests.

Each namespace is an exclusive administrative deletion and wire-format trust
domain. It must not be shared across tenants, security principals, or
serializer schemas that cannot safely read each other's values. `Clear`
unlinks every matching key regardless of which process or cache instance wrote
it. Two instances deliberately using the same namespace therefore share one
clear domain.

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
immutable snapshots or call `Set` after mutation. This applies to every alias
the caller retains: values passed to `Set`, values returned by a loader, and
values returned from cache reads. The package does not clone L1 values.

Empty serialized payloads are valid when the serializer accepts them. Redis
key absence is distinguished from an existing empty value; only absence maps
to `cache.ErrCacheMiss`.

## Read and Load Behavior

### `ValueCache.Get`

1. Validate initialization, context, and logical key.
2. Issue one `GETRANGE 0 MaxValueBytes`, which returns no more than
   `MaxValueBytes + 1` bytes.
3. A non-empty hit costs one Redis command and one round trip.
4. A zero-length result issues one conditional `EXISTS` command, making a miss
   or an existing empty value cost at most two sequential commands/round
   trips.
5. Reject an oversized payload before deserialization.
6. Unmarshal exactly once and return the resulting `V`.

### `TieredCache.Get`

1. Enter the shared local-state barrier, fail closed if L1 is blocked, and read
   L1. Return a hit while still protected by that barrier, without Redis or
   serializer calls.
2. Continue only when L1 returns `cache.ErrCacheMiss`; propagate other L1
   errors, then acquire the context-aware gate for that logical key.
3. Under the shared barrier, recheck blocked state and L1. Return a recheck hit
   or capture the current generation for a miss.
4. Read L2 while holding the per-key gate but not the local barrier.
5. On L2 hit, reacquire the shared barrier and atomically check caller
   cancellation, blocked state, and generation before calling `Local.Set`
   inside that same barrier. If state changed, return the decoded value without
   repopulating L1; if blocked, return `ReasonLocalBlocked`.
6. On L2 miss, return `cache.ErrCacheMiss`.

### `TieredCache.GetOrLoad`

`TieredCache` owns same-key collapse. It acquires the same context-aware
per-key gate used by `Get`, mutations, and invalidation, then rechecks blocked
state and L1 under the shared local barrier. Inside the gate:

1. Read L2.
2. Return the L2 value on hit and populate L1 for `TieredConfig.LocalTTL` only
   if context and local state still permit it. No `PTTL` command is added, so
   an existing L2 hit does not claim knowledge of remaining remote TTL.
3. Continue only on `cache.ErrCacheMiss`.
4. Run the caller loader once for the collapsed local flight.
5. Store the loaded value in L2.
6. After L2 succeeds, check cancellation and local state, store the original
   `V` in L1 with `min(TieredConfig.LocalTTL, requested remote TTL)` when the
   requested TTL is positive, and return it.

The effective L1 TTL depends on whether this call knows the remote write TTL:

```text
existing L2 hit: TieredConfig.LocalTTL
known write TTL > 0: min(TieredConfig.LocalTTL, requested remote TTL)
known write TTL = 0: TieredConfig.LocalTTL
```

A loader error is returned unchanged and is not cached. A Redis or
serialization error is not converted into a miss and does not fall through to
the loader.

Followers wait context-sensitively. Followers already present in one active
`GetOrLoad` flight receive that flight's value or error without rerunning the
loader, unless their own context is canceled first. A caller arriving after
the flight entry is retired may start a new attempt. One same-key flight
executes the caller loader exactly once, including an error result. Two
processes can still execute the loader at the same time unless the caller
deliberately composes `cache/rediscoord`.

## Coordination and Linearization

`TieredCache` owns a registry of active per-key gates. A gate exists only while
an operation holds or waits for it; reference counting removes the entry after
the final holder/waiter exits. Waiting uses a channel/select contract so each
follower can return its own cancellation without leaking the gate.

The decorator also owns a local-state read/write barrier and a monotonically
increasing generation. Every L1 read, hit return, and population runs inside a
shared barrier section that checks blocked state. Capturing a generation also
runs in a shared section. `ClearLocal`, common mandatory full-local cleanup,
and transition to blocked state run in an exclusive section. They increment
the generation before clearing or blocking. The post-L2 generation/state check
and `Local.Set` occur in one shared section, eliminating a check-then-write
window. The caller-provided L1 must not re-enter the same `TieredCache` from an
L1 method.

All same-key L2 reads, loads, sets, deletes, and `InvalidateLocal` calls are
serialized by the same gate. L1 is rechecked after acquisition. This provides
the following linearization guarantees:

- an old L2 read cannot populate L1 after a completed same-key `Set`, `Delete`,
  or `InvalidateLocal`;
- concurrent same-key mutations reach Redis and L1 in gate order, so a delayed
  earlier local commit cannot overwrite a later Redis value;
- an RESP3 invalidation that completes after a refill has removed that refill,
  and no pre-invalidation refill can resurrect it afterward.

`ClearLocal` takes the exclusive local barrier, increments the generation, and
clears L1 before releasing the barrier. A refill or mutation that captured an
older generation may finish remote work but must not populate L1. A blocked
transition uses the same exclusive barrier and generation fence, so an
operation already waiting on a key gate or Redis cannot later return an L1 hit
or repopulate L1 after blocking completes. An operation whose L1 hit completed
under the shared barrier before the transition may complete before it, which
is the explicit linearization boundary. The gate registry is bounded by
concurrent active keys rather than historical keys; no completed-key entry is
retained.

The L1 remains caller-owned for lifecycle purposes, but mutation and reads are
exclusively transferred to one `TieredCache` after construction. The caller
must not access it directly or share it with another decorator. It must be
concurrency-safe, honor context cancellation, and avoid callbacks into the
decorator. Multiple decorators may share the same `ValueCache` L2, but their
gates, local-state generation, blocked state, pointer identity, and
invalidation are process-instance-local.

## Mutation Behavior

### `Set`

`ValueCache.Set` validates and serializes the value, rejects a result larger
than `MaxValueBytes`, and only then dispatches Redis `SET`. An oversized value
therefore cannot mutate either tier.

`TieredCache.Set` holds the same-key gate for the complete operation:

1. Under the shared barrier, fail closed if blocked and capture the current
   local-state generation.
2. Call `ValueCache.Set` with the requested remote TTL.
3. After known L2 success, enter the shared barrier and atomically recheck
   caller cancellation, blocked state, and generation.
4. If all still permit population, write the original `V` into L1 inside the
   same shared section with the effective L1 TTL.

The package does not serialize or clone the L1 value. It does not update L1
before Redis succeeds.

The generic go-redis command surface does not expose whether a failed mutation
was written to the socket. Therefore, after `ValueCache` invokes `SET` or
`DEL`, any non-nil command result conservatively preserves
`redis.ErrCommitUnknown`. `TieredCache` performs mandatory local invalidation
before returning because Redis may contain the new value. If caller context is
canceled after Redis reports known success, the method also invalidates L1 but
returns the caller context error rather than claiming commit ambiguity. If L1
population fails after successful L2 write, the cache invalidates L1 and
returns the L1 error; the next read can recover from L2.

### `Delete`

`ValueCache.Delete` removes the L2 key and treats an absent key as success.
`TieredCache.Delete` holds the same-key gate, checks blocked state under the
shared local barrier, and attempts the L2 delete. Once the Redis `DEL` call has
been invoked, it always performs mandatory L1 invalidation before returning.
Errors are joined without hiding the Redis mutation outcome or L1 failure.
Validation, blocked state, or cancellation detected before Redis invocation
returns without mutating either tier.

### `Clear`

`ValueCache.Clear` scans only its namespace and sends `UNLINK` in batches no
larger than `ClearBatchSize`. Every `SCAN` page is processed immediately and,
when Redis returns more keys than the configured limit, re-chunked before
`UNLINK`; the implementation never accumulates the full namespace in memory.
Every server-side scan uses the exact pattern
`bluetape:cache:value:<validated-namespace>:*`; it does not scan the database
and filter locally. Redis `SCAN COUNT` is only a hint, so peak key-list memory
is one server-returned page plus one `UNLINK` chunk of at most
`ClearBatchSize`, not a promised hard page size. It never calls `FLUSHDB` or
falls back to blocking `DEL`.

`TieredCache.Clear` attempts remote clear and, once the remote clear has begun,
always invokes the common mandatory full-local-clear primitive. That primitive
takes the exclusive local barrier, increments the generation, calls
`Local.Clear` with the owned cleanup context, and blocks the decorator on
failure. It does not unblock a decorator that another operation blocked;
successful explicit `ClearLocal` remains the only repair. Remote and local
errors are joined without silently declaring success. Validation or
cancellation detected before the first remote command returns without mutating
either tier.

Namespace clear is:

- context-cancellable;
- idempotent for already removed keys;
- non-atomic and not a snapshot;
- proportional to namespace size;
- intended for administration, tests, and cache reset, not request hot paths.

Keys added concurrently can escape a scan iteration, and callers must not use
`Clear` as a transaction or authorization boundary.

### Mandatory Local Cleanup and Recovery

Safety cleanup after a remote mutation does not reuse a canceled caller
context. `TieredCache` creates a package-owned `context.Background()` child
with `TieredConfig.LocalCleanupTimeout`, calls the required L1 `Delete` or
`Clear` synchronously inside the appropriate shared or exclusive local barrier
section, and waits for that call before returning. The supplied L1 must honor
context cancellation; the package does not spawn a detached goroutine to force
an uncooperative implementation to stop.

If mandatory cleanup fails or its cleanup context expires, `TieredCache`
atomically enters a local-blocked state. A failed `InvalidateLocal` or
`ClearLocal` also blocks the decorator because an externally requested
invalidation may not have removed stale data. While blocked, `Get`,
`GetOrLoad`, `Set`, `Delete`, and `Clear` fail closed with
`ReasonLocalBlocked` instead of serving potentially stale L1 data.
`InvalidateLocal` may still attempt a single-key deletion but does not clear
the global blocked state. A successful `ClearLocal` is the only repair
operation: it advances the generation, clears L1, and then unblocks the
decorator. A failed repair, including a failed `ClearLocal` that began while
unblocked, remains blocked. This uses one bounded state flag rather than
retaining per-key tombstones.

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
    ReasonLocalBlocked   Reason = "local-blocked"
    ReasonProviderFailure Reason = "provider-failure"
    ReasonPartialClear   Reason = "partial-clear"
)

type ClearProgress struct {
    ScannedKeys     int64
    UnlinkedBatches int64
}

type CacheError struct { /* redacted fields */ }

func (e *CacheError) Error() string
func (e *CacheError) Unwrap() error
func (e *CacheError) Operation() string
func (e *CacheError) Reason() Reason
func (e *CacheError) ClearProgress() (ClearProgress, bool)
```

Reasons cover:

- configuration;
- uninitialized zero value;
- serialization;
- payload too large;
- invalid payload;
- local cache failure;
- local cache blocked pending explicit repair;
- Redis provider failure;
- a namespace clear that stopped after partial progress.

Rules:

- A missing Redis key returns `cache.ErrCacheMiss` exactly; no `CacheError`
  wrapper changes miss identity.
- Invalid config, a nil client, and nil or typed-nil interface dependencies
  return `*CacheError` with `ReasonConfiguration`. Unsupported Redis executor
  shapes do not satisfy the concrete public client field type.
- `Serializer.Marshal` failures return `*CacheError` with
  `ReasonSerialization`; `Serializer.Unmarshal` failures return
  `ReasonInvalidPayload`. Both preserve the serializer cause through
  `Unwrap`, and a marshal failure issues no Redis mutation.
- An oversized serialized value or Redis payload returns `*CacheError` with
  `ReasonPayloadTooLarge`.
- Redis failures return an outer `*CacheError` with
  `ReasonProviderFailure`; its cause is a redacted `*redis.OpError` for the
  `redisvalue` family. Mutation ambiguity additionally satisfies
  `errors.Is(err, redis.ErrCommitUnknown)`.
- L1 failures return `ReasonLocalFailure`; blocked-state rejections return
  `ReasonLocalBlocked`.
- A clear that fails after work began returns `ReasonPartialClear` and exposes
  only monotonic `ClearProgress` counts. Its cause remains the relevant
  `*redis.OpError` or context error.
- Raw values, serialized bytes, Redis keys, namespace contents, and provider
  diagnostics are not logged by the package.
- `CacheError.Error` and `redis.OpError.Error` contain only stable,
  low-cardinality operation/reason labels and a redacted identifier. They do
  not interpolate causal `Error()` text. Access to a wrapped provider or
  serializer cause through `errors.Unwrap`/`errors.As` is an explicit caller
  choice.
- The package owns no logger; callers observe returned errors.
- Joined cleanup or L1 invalidation errors preserve each inspectable cause.
- A failed `Clear` reports partial progress without enumerating raw keys and is
  safe to retry because `UNLINK` of already removed keys is idempotent.

Redis is treated as an authenticated service boundary configured by the
caller through ACLs, TLS, and network isolation, but stored bytes are still
untrusted serializer input. Serializers must not perform executable
deserialization. `MaxValueBytes` bounds input bytes, not every allocation a
decoder can make. Applications that need tamper detection supply an
authenticated envelope serializer and reject unverifiable payloads.

The serializer is caller-owned trusted code. The package does not recover its
panics; a valid serializer must return a redacted error for malformed input and
must not panic. It owns its own allocation, nesting/recursion, decompression,
and CPU-amplification limits for every input up to `MaxValueBytes`. On writes,
`MaxValueBytes` is a Redis admission bound applied after `Marshal`; it cannot
bound temporary or result allocations already made by the serializer.

## Context and Cancellation

- Nil contexts follow the repository convention and normalize to
  `context.Background()`.
- A canceled context is checked before serialization and before Redis
  dispatch.
- Validation or cancellation before the first mutation command causes no L1
  or L2 side effect.
- Caller cancellation and deadlines are never retried.
- Read cancellation must not populate L1 after the caller has returned.
- A loader result observed after cancellation must not be written to L2 or L1.
- A mutation error after dispatch may still be commit-unknown; cancellation
  does not prove Redis rejected the operation.
- Mandatory safety invalidation uses the bounded package-owned cleanup context
  and completes before the method returns, even when caller context is already
  canceled. Cleanup failure blocks L1 until `ClearLocal` repairs it.
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

The current `redisnear.NewPubSub` constructor must not receive a `TieredCache`
as its `Local` cache. That strategy calls ordinary `Delete` and `Clear`, which
would also mutate L2 through the decorator. A future adapter or strategy may
compose the two only by calling `InvalidateLocal` and `ClearLocal`.

The future RESP3 strategy remains constructor-separated in `cache/redisnear`;
it is not a `Strategy` enum hidden inside #535 options. If the spike fails,
#535 remains an explicit tiered cache with bounded stale time but is not
advertised as a coherent multi-process near cache. The separate existing
Pub/Sub strategy remains available for its own local-cache contract, not as a
direct wrapper around `TieredCache`.

## Failure Modes and Required Response

| Failure mode | Required response |
|---|---|
| Serializer rejects a value | Return an inspectable serialization error; do not call Redis or update L1. |
| Serialized value exceeds `MaxValueBytes` | Reject before Redis dispatch or L1 update. |
| Redis read returns malformed or oversized bytes | Return an invalid/oversized payload error; do not treat it as a miss or auto-delete it. |
| Redis `SET` may have committed before timeout/cancellation | Return `redis.ErrCommitUnknown` and invalidate L1. |
| L1 update fails after L2 success | Invalidate L1, return the L1 error, and leave L2 as the recovery source. |
| Mandatory L1 cleanup fails or times out | Enter local-blocked state; fail ordinary operations until successful `ClearLocal`. |
| Loader succeeds but L2 write fails | Return the L2 error and do not populate L1. |
| L1 TTL is configured above the finite default L2 TTL | Reject construction; per-entry overrides cap the effective local TTL. |
| L1 is filled near the end of an existing Redis TTL | Permit a stale window bounded by the effective local TTL; document that only invalidation closes it earlier. |
| Namespace clear is interrupted | Return cancellation plus redacted partial progress; L1 is still cleared by `TieredCache.Clear`. |
| Pipeline, transaction queue, cluster, ring, or opaque `Cmdable` wrapper is considered | The concrete `*redis.Client` API makes the unsupported shape unrepresentable. |
| Existing Pub/Sub strategy is given a TieredCache as Local | Unsupported composition; document the local-only adapter boundary rather than deleting L2 on invalidation. |
| Future tracking connection becomes ambiguous | #536 must clear L1 and re-establish tracking before serving tracked local hits. |

## Operational Contract

#535 targets Redis 6 or newer with one writable primary. The caller owns Redis
client creation, authentication, TLS, failover, dial/read/write/pool timeouts,
and readiness checks. The Redis identity must be authorized for `GETRANGE`,
`EXISTS`, `SET`, `DEL`, `SCAN`, and `UNLINK` on the configured namespace.
`UNLINK` is required so namespace clear does not degrade into a blocking delete.
Deployments should use a dedicated Redis identity and database where practical,
the narrowest feasible key-prefix/command ACL, and verified server certificates
when TLS is enabled. The identity does not need and must not be granted
`FLUSHDB`, `FLUSHALL`, or unrelated administrative commands for this package.

Operators must size Redis memory and eviction policy for the chosen TTLs. A
zero remote TTL means the package never expires that key; namespace rotation
or explicit `Clear` is then required to reclaim old data. Readiness must prove
the cache's actual command path, while an application may choose whether cache
readiness is startup-fatal based on its own availability contract. #535 does
not conceal provider failure by serving a stale local fallback.

Cluster and ring deployments are outside the accepted constructor type.
Supporting them later requires an explicit design for per-primary `SCAN`,
slot-safe deletion, topology changes during clear, and aggregate
partial-progress reporting.

## Testing Strategy

### Unit tests with fakes

- nested default config, independent value/tiered per-cache overrides,
  validation, and mutation isolation;
- nil client plus nil and typed-nil interface dependencies, safe
  constructor-only zero values, and a public API whose concrete client type
  excludes pipelines, transaction queues, clusters, rings, and opaque wrappers;
- logical-key preservation, adversarial namespace glob rejection, namespace
  collision resistance, length limits, and redaction;
- exact server-side namespace scan pattern and intentional same-namespace
  shared-clear-domain behavior;
- L1 hit performs no Redis or serializer call;
- L1 pointer hit returns the same pointer;
- L2 hit unmarshals once and puts the resulting `V` into L1;
- separate cold TieredCache instances deserialize distinct pointers;
- L2 miss returns `cache.ErrCacheMiss`;
- bounded read command counts: one `GETRANGE` for non-empty hits and one
  conditional `EXISTS` only for zero-length results;
- `GetOrLoad` follows L1 -> L2 -> loader, shares one flight's success or error
  with its existing waiters independently of the L1 implementation, and
  releases active gate entries;
- loader and serializer failures are not cached;
- Redis-first `Set` ordering and original-reference L1 population;
- retained pointer aliases from `Set` and loader results follow the documented
  immutable-snapshot rule;
- finite, zero, negative, default, and per-entry TTL behavior, with
  `LocalTTL` for pre-existing L2 hits and `min(LocalTTL, requestedTTL)` only
  for known positive writes;
- bounded and empty payload behavior, including `MaxValueBytes + 1`, the
  64-MiB configuration cap, and oversized `Set` rejection before Redis;
- exact error mapping for miss, marshal, unmarshal, oversized payload, Redis,
  commit-unknown, local failure, local-blocked, and partial clear;
- commit-unknown and mandatory local invalidation behavior using a fresh
  cleanup context;
- cleanup timeout/failure enters local-blocked state, ordinary operations fail
  closed, and successful `ClearLocal` repairs the cache;
- failed direct `InvalidateLocal`/`ClearLocal` blocks the decorator, and a
  successful single-key invalidation does not unblock it;
- `Delete`, `ClearLocal`, `InvalidateLocal`, and namespace `Clear` error joining;
- each `SCAN` page is streamed immediately, oversized pages are re-chunked,
  every `UNLINK` batch stays bounded, and no whole-namespace slice is retained;
- cancellation before dispatch, during load, after load, and between clear
  batches, with no late L1/L2 write.

### Redis Testcontainers

- set/get/delete/miss against a real Redis service;
- finite TTL expiration and explicit zero-TTL persistence;
- finite default/per-entry TTL validation and the documented bounded stale
  window when L1 is filled near Redis expiry;
- namespace isolation and `SCAN` plus `UNLINK` clear;
- multi-page clear with batch sizes smaller and larger than a returned page;
- maximum payload boundary and oversized-value rejection;
- existing empty payload versus missing key;
- cancellation/failure behavior with connection readiness proved;
- two TieredCache instances sharing L2 bytes but not pointer identity.

Testcontainers commands run sequentially.

### Concurrency and race

- bounded same-key `GetOrLoad` stress with exact loader-call totals;
- leader/follower cancellation and one-flight error sharing without sequential
  loader retries by existing waiters;
- concurrent different-key loads;
- deterministic latch tests for read/refill versus `Set`, `Delete`,
  `InvalidateLocal`, and `ClearLocal`, proving no stale resurrection or
  mutation-order reversal;
- pause after generation check, complete `ClearLocal`, then resume the refill,
  proving the shared/exclusive barrier prevents post-clear `Local.Set`;
- block transition while a key-gate waiter or L2 read is active, proving no
  post-block L1 hit return or population;
- `TieredCache.Clear` versus a delayed refill, proving the common mandatory
  full-local-clear primitive fences old generations;
- concurrent `Get`, `Set`, `Delete`, `ClearLocal`, and cancellation without
  races, retained gates, or late local writes;
- active-gate registry returns to zero after stress and canceled waiters;
- `go test -race -count=1 ./cache/redisvalue`.

### Public examples

Compile-checked examples cover:

- `DefaultConfig` and per-cache override;
- `ValueCache` with an existing serializer;
- `TieredCache` with `cache.NewMemory`;
- default and per-entry TTL methods;
- pointer snapshot semantics;
- `ClearLocal` versus namespace `Clear`;
- safe single-primary `*redis.Client` construction and documented unsupported
  client shapes;
- recommended `VersionedSerializer` use and namespace rotation for
  incompatible wire changes.

## Documentation

Add synchronized `cache/redisvalue/README.md` and `README.ko.md` documenting:

- L1 object/reference and L2 serialization boundaries;
- exclusive post-construction L1 access and instance-local coordination/
  invalidation guarantees;
- defaults and per-cache override;
- strict Redis-first mutation behavior;
- pointer immutability guidance;
- TTL and bounded-staleness behavior;
- operational cost and non-atomic semantics of `Clear`;
- namespace ownership as one exclusive tenant/security/wire-format clear
  domain, including same-namespace cross-instance deletion;
- cancellation and commit-unknown behavior;
- caller serializer panic-free/resource-bound requirements and the distinction
  between Redis write admission size and serializer allocation;
- required Redis commands (`GETRANGE`, `EXISTS`, `SET`, `DEL`, `SCAN`, and
  `UNLINK`), Redis 6+ single-primary topology, ACL/TLS ownership, caller
  dial/read/write/pool timeouts, readiness, memory eviction policy, and
  zero-TTL cleanup responsibility;
- no blocking `DEL` fallback for namespace clear and no cluster/ring support in
  #535;
- why the current `cache/redisnear` Pub/Sub strategy must not directly wrap a
  `TieredCache`, `cache/rediscoord` as cross-process load coordination, and
  `cache/redisfory` as the Fory-specific direct store;
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
- Wire compatibility is serializer-owned. A rolling deploy may reuse a
  namespace only when old and new binaries have a tested bidirectional
  read/write compatibility matrix and agree on the same versioned envelope.
  Otherwise the deployment rotates the namespace. The old namespace remains
  available through the rollback window and at least the maximum finite Redis
  TTL plus an operational margin; zero-TTL namespaces require explicit
  administrative cleanup. Mixed-version tests cover upgrade and rollback
  readers before reuse is documented as safe.
- The physical key format is new and must remain stable after release unless a
  documented migration or namespace generation is introduced.

## Acceptance Criteria

1. `cache/redisvalue` exposes constructor-only, zero-value-safe
   `ValueCache[V]` and `TieredCache[V]` types.
2. `ValueCache[V]` implements `cache.Cache[string,V]` using a caller-owned
   go-redis `*redis.Client` and `serialization.Serializer[V]`.
3. `TieredCache[V]` implements `cache.LoadingCache[string,V]`, stores `V`
   directly in L1, and serializes only through L2.
4. `DefaultConfig` provides independent value/tiered defaults, is copied on
   construction, and can be overridden per cache without global mutable state.
5. L1 hits make no Redis or serializer call; L2 hits populate the same decoded
   `V` into L1.
6. Loads follow L1 -> L2 -> loader, collapse through TieredCache-owned
   context-aware flights that share success or error with existing waiters,
   release inactive gate entries, and write L2 before L1 population.
7. TTL defaults and overrides satisfy the documented finite/zero relationship,
   and tests do not claim that configured duration ordering is an atomic Redis
   expiry-order guarantee.
8. Payload reads and Redis write admission, namespace `UNLINK` chunks, retained
   clear memory, keys, errors, and cancellation are bounded and redacted as
   specified; Redis controls the size of one returned `SCAN` page and the
   serializer controls its own allocations.
9. Mutation ambiguity preserves `redis.ErrCommitUnknown` and invalidates L1.
10. Same-key reads, loads, mutations, and invalidations are linearized; the
    shared/exclusive local-state barrier and generations prevent stale local
    resurrection after clear or block transitions.
11. Mandatory cleanup uses an owned timeout context and blocks ordinary cache
    use until successful `ClearLocal` repair if cleanup fails.
12. `InvalidateLocal` and `ClearLocal` never mutate Redis and are suitable for
    the #536 spike boundary; current Pub/Sub invalidation never directly wraps
    the TieredCache mutation surface.
13. The public constructor accepts only synchronous single-primary
    `*redis.Client`; queued, cluster, ring, and opaque `Cmdable` executors are
    unrepresentable.
14. Namespace and L1 ownership contracts prevent accidental cross-tenant clear
    domains and access outside the decorator's coordination barrier.
15. Unit, Testcontainers, race, stress, and compile-checked example evidence
    passes.
16. English and Korean package documentation stay synchronized and clearly
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
