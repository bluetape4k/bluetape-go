# Issue #535 Redis Tiered Value Cache Design

Status: Step 2-R converged; awaiting written spec review
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
| Separate L1 and L2 TTL | Keep | Configured durations keep the local default no greater than the remote default; cross-tier expiry ordering remains explicitly non-absolute. |
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
    LocalTTL                time.Duration
    InvalidationWaitTimeout time.Duration
    LocalCleanupTimeout     time.Duration
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
        LocalTTL:                30 * time.Minute,
        InvalidationWaitTimeout: 30 * time.Second,
        LocalCleanupTimeout:     time.Second,
    },
}
```

Rules:

- `TieredConfig.LocalTTL`, `InvalidationWaitTimeout`, and
  `LocalCleanupTimeout` must be positive.
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
expiry-order guarantee for an existing L2 value. An L2 response can be delayed
after Redis read, and #535 does not add a `PTTL` round trip or Lua
read-with-expiry operation. Such a value may be served for at most `LocalTTL`
after L1 population is accepted, but the package makes no bounded claim from
the Redis key's unknown expiry instant. Invalidation is required to close that
cross-tier window deterministically.

When the current call writes a positive Redis TTL, it first normalizes the
duration to the actual go-redis wire precision (`EX` whole seconds or `PX`
milliseconds, including the sub-millisecond minimum), records a monotonic
timestamp before mutation invocation, and subtracts elapsed time immediately
before `Local.Set`. L1 receives
`min(LocalTTL, normalizedWireTTL-elapsed)` and skips population when that
remainder is non-positive. This reduces cross-tier skew but is not an absolute
expiry-order guarantee because a caller-owned L1 may anchor its TTL after an
arbitrary `Local.Set` delay. A zero remote TTL still uses `LocalTTL`.

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

func NewTieredCache[V any](options TieredOptions[V]) (*TieredCache[V], error)

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

`NewTieredCache` rejects a nil `Remote` and a non-nil but zero/uninitialized
`ValueCache` with `CacheError` and `ReasonConfiguration` before reading the
remote default TTL, retaining the L1, or creating any coordinator state.

`InvalidateLocal` acquires the same-key operation token and calls only
`Local.Delete` through a maintenance lease. `ClearLocal` enters the repairing
state, drains local leases, and calls only `Local.Clear`. `InvalidateLocal`
caps its complete key-token wait with `InvalidationWaitTimeout`; after token
acquisition, local admission/delete uses the smaller of the remaining caller/
wait budget and `LocalCleanupTimeout`. `ClearLocal` has no key-token wait and
uses one `LocalCleanupTimeout` budget across repair admission, drain, and
clear. Cancellation or timeout before admission counts as failed invalidation
and establishes or retains blocked state. Successful `InvalidateLocal` never
unblocks the decorator, while successful `ClearLocal` is the one explicit full
repair that unblocks it. Neither method invokes Redis.

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
- The deterministic redacted key ID is a correlation pseudonym, not an
  anonymization or authorization boundary. Applications must not place
  secrets or direct PII in logical keys; low-entropy keys can be guessed by
  dictionary comparison.
- #535 locks the physical format in tests but does not expose a speculative
  public reverse-key mapper. #536 may add the minimum mapper required by a
  passing RESP3 spike.

The public client type is the concrete go-redis `*redis.Client`, which issues
commands synchronously and makes pipelines, transactional queues,
`*redis.ClusterClient`, `*redis.Ring`, and opaque `Cmdable` wrappers
unrepresentable at the constructor boundary. #535 supports a single writable
Redis primary through a direct endpoint whose identity remains stable for the
whole operation. `redis.NewFailoverClient`, automatic-failover proxies,
cluster/proxy endpoints, and primary changes during `Clear` are unsupported in
#535 even when they present as `*redis.Client`. A changed server invalidates the
server-local SCAN cursor and can produce silent partial clear. Production
construction rejects a nil client, while topology is a caller-owned operational
precondition rather than something the Go type can prove. Package-internal
narrow command interfaces remain replaceable by fakes in unit tests.

Each namespace is an exclusive administrative deletion and wire-format trust
domain. It must not be shared across tenants or serializer schemas that cannot
safely read each other's values. `Clear` unlinks every matching key regardless
of which process or cache instance wrote it. Two instances deliberately using
the same namespace therefore share one clear domain. A namespace prefix is not
a Redis security-principal or key-name-confidentiality boundary because `SCAN`
enumeration is wider than key-prefix ACL matching.

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

1. Acquire a healthy local-state lease, read L1, and post-check its generation
   before returning a hit. A stable hit makes no Redis or serializer call.
2. Continue only when L1 returns `cache.ErrCacheMiss`; propagate other L1
   errors, then acquire the context-aware operation token for that logical key.
3. Under a healthy local-state lease, recheck L1. Return a stable recheck hit
   or capture the current generation and atomically admit one L2-read ticket
   for a miss.
4. Release the local lease and read L2 under that ticket while holding the
   per-key operation token.
5. On L2 hit, acquire a healthy local lease for the captured generation, check
   caller cancellation, call `Local.Set`, and post-check state/generation
   before returning. If generation changed without blocking, return the
   decoded value without relying on the L1 population; if blocked, return
   `ReasonLocalBlocked`.
6. On L2 miss, return `cache.ErrCacheMiss`.

If L1 population returns an error, the operation still owns the key token and
must call the token-held mandatory delete before returning. This covers L1
implementations that store a value and then report failure. Cleanup failure
blocks the decorator; the returned error preserves both the original local
population error and cleanup error.

### `TieredCache.GetOrLoad`

Before coordinator lookup or any cache side effect, `GetOrLoad` validates that
the loader is non-nil. A nil loader returns `CacheError` with
`ReasonConfiguration`; it never creates a flight, reads either tier, or
invokes Redis.

`TieredCache` owns same-key collapse through the per-key coordinator described
below. A leader holds the same context-aware operation token used by `Get`,
mutations, and invalidation, then rechecks L1 under a healthy local-state
lease. Every L2 method or loader invocation below first obtains its own
one-shot side-effect admission ticket under a newly admitted healthy lease at
the expected generation, releases the lease, then invokes the admitted action.
Inside the leader flight:

1. Read L2.
2. Return the L2 value on hit and populate L1 for `TieredConfig.LocalTTL` only
   if context and local state still permit it. No `PTTL` command is added, so
   an existing L2 hit does not claim knowledge of remaining remote TTL.
3. Continue only on `cache.ErrCacheMiss`.
4. Admit and run the caller loader once for the collapsed local flight.
5. Recheck local state/generation, admit the Redis write, record a monotonic
   start immediately before invocation, and store the loaded value in L2.
6. After L2 succeeds, check cancellation and local state. For a positive
   requested TTL, subtract elapsed time from normalized wire TTL and populate
   L1 only for a positive adjusted remainder; for zero TTL, use `LocalTTL`.
   Return the original
   `V` even when a healthy newer generation causes L1 population to be skipped.

`GetOrLoad` applies the same token-held mandatory delete and joined-error rule
when its `Local.Set` returns an error, and publishes that combined outcome to
the flight.

The effective L1 TTL depends on whether this call knows the remote write TTL:

```text
existing L2 hit: TieredConfig.LocalTTL
known write TTL > 0: min(TieredConfig.LocalTTL, normalized wire TTL - elapsed)
known write TTL = 0: TieredConfig.LocalTTL
```

A loader error is not cached and, while local state remains healthy, is returned
unchanged. A Redis or serialization error is not converted into a miss and does
not fall through to the loader. A concurrent blocked-state transition uses the
terminal classification below while preserving the original error as cause.

After an L2 miss, the leader rechecks state/generation before loader admission.
A blocked state returns `ReasonLocalBlocked`; a healthy generation change
restarts L1/L2 checks inside the same leader flight. Before publishing a loader
error, the leader performs the same terminal state check: a blocked state
publishes `ReasonLocalBlocked`, preserving the loader error as its cause, while
a healthy state publishes the loader error unchanged. After loader success, a
blocked state returns `ReasonLocalBlocked` without writing either tier. A
healthy generation change may return the loaded value as an uncached operation
that linearized before the transition, but it does not write L2 or L1. After
known L2 write success, a healthy generation change similarly skips L1 and
returns the value; a blocked state publishes `ReasonLocalBlocked`.

Followers wait context-sensitively. Followers already registered in one active
`GetOrLoad` flight receive that flight's value or error without rerunning the
loader, unless their own context is canceled first. The leader's loader, TTL,
and context define the flight; follower-supplied loaders and TTLs are not run.
Leader-context cancellation publishes that cancellation as the flight error
to all still-registered followers. A caller arriving after result publication
starts or joins a new flight generation. One same-key flight executes the
caller loader exactly once, including an error result. Two processes can still
execute the loader at the same time unless the caller deliberately composes
`cache/rediscoord`.

## Coordination and Linearization

`TieredCache` owns a registry of active per-key coordinators. Each coordinator
contains:

- one context-aware operation token that serializes same-key L2 reads, load
  leaders, sets, deletes, and `InvalidateLocal` calls; and
- at most one atomically joinable `GetOrLoad` flight record with its generation,
  leader inputs, result/error, completion channel, and participant references.

`GetOrLoad` checks or creates the active flight under the coordinator mutex.
Only the creator becomes leader and waits for the operation token. Existing
callers atomically register before publication and wait on the flight channel;
ordinary `Get`, `Set`, `Delete`, and invalidation never join a load result and
instead wait for the operation token. The leader publishes value/error and
detaches the active flight while still holding the operation token, then
releases its own participant reference immediately after publication on both
success and error, then releases the token. New arrivals after publication
create or join a new flight generation. The published record remains alive
until every registered follower participant consumes it or cancels. A follower
releases its participant reference after receiving the result or atomically
detaches only itself on cancellation. Publication and follower cancellation
arbitrate under the coordinator mutex; whichever records first determines that
follower's result and releases its reference exactly once.

Registry lookup/install/retain and idle retirement are serialized by the
registry mutex. Lookup increments an external coordinator reference before
releasing that mutex. Final release reacquires the registry mutex and removes a
coordinator only when the map still points to the same instance and it has no
external reference, token holder/waiter, active flight, or retained
participant. No new caller can retain the instance between that idle check and
map removal. Code never acquires the registry mutex while holding the
coordinator mutex; an internal final release drops the coordinator mutex before
requesting retirement. This identity check and lock order prevent ABA removal
and simultaneous same-key token domains. Completed keys are not retained.

If the leader context ends before operation-token acquisition, the acquisition
loses a single atomic race to cancellation. Under the coordinator mutex, that
leader publishes its context error, closes the flight channel, detaches the
same active generation, and releases its participant/token-wait references
without pretending to hold the token. Followers receive the published error;
new arrivals may create the next generation. A deterministic latch test fixes
this terminal path so it cannot leak a flight or coordinator. If token
acquisition wins the race, the leader checks context before any cache side
effect, publishes cancellation while holding the token, and releases it
normally.

The flight record is constant-size shared state: one completion channel,
result/error fields, generation metadata, and an atomic participant count. It
does not retain a waiter slice/map or scan waiters during publication or
cleanup; each caller owns only its own wait/select state.

The decorator separately owns a context-aware local-state barrier with states
`healthy`, `blocking`, `blocked`, and `repairing`, plus a monotonically
increasing generation. Repairing state records whether its owned repair epoch
began from healthy or blocked state. A healthy lease registers one active L1
operation and captures the generation. Every L1 method post-checks
state/generation before returning or treating a population as usable. A block
transition atomically enters `blocking`, increments generation, and denies new
healthy leases; it can then publish `blocked` without waiting indefinitely for
an uncooperative old lease. An old read that post-checks after the fence returns
`ReasonLocalBlocked`; an old write may physically finish, but cannot be served
while blocked and is removed by the next successful repair.

Healthy lease admission/release is O(1), allocation-free, and does not create
a per-key coordinator on the initial L1-hit path. It does not hold a globally
exclusive mutex across `Local.Get`, so different-key healthy hits and L1 calls
can proceed concurrently. A lease used to check/capture state is released
immediately after its L1 operation or generation capture and is never retained
across Redis or loader work.

Initial healthy-lease admission is exact:

| Observed state | Admission behavior |
|---|---|
| `healthy` | Grant the lease and capture its generation. |
| `repairing` that began from healthy | Wait context-sensitively without a local lease or key operation token; after the state changes, retry admission from the method's initial state check. Caller cancellation returns the context error and does not change cache state. |
| `blocking`, `blocked`, or `repairing` that began from blocked | Return `ReasonLocalBlocked` without admitting the operation. |

If a path already acquired a key token before discovering that it needs initial
admission, it releases the token before waiting and restarts, so repair never
waits behind a token holder that is itself waiting for repair. An active
`GetOrLoad` leader retains its flight leadership and participant reference
during this wait; after repair it reacquires the token and restarts L1/L2 checks
inside the same flight rather than re-entering registry lookup or joining its
own flight.

Before every loader or L2 method invocation, an ordinary tiered operation
obtains an atomic one-shot side-effect admission ticket while holding a healthy
lease at the expected generation. The ticket admits that whole L2 method call,
including `ValueCache.Get`'s conditional `EXISTS` command. It is a caller-local
generation decision,
not a retained registry object: it adds no waiter collection or background
work. The operation releases the lease before invoking the admitted action. A
state transition that starts after ticket issuance linearizes after that
admission and cannot revoke the one already admitted invocation, even if the
actual function call begins after the transition. Cancellation checked before
invocation leaves the ticket unused. Any later loader/Redis action requires a
new ticket, so a transition during an L2 read or loader can prevent the next
write. Already-admitted work may finish, but cannot create a usable L1
population after a generation change, and blocked state remains its public
terminal classification.

After an operation releases its local-state lease, its terminal behavior is
fixed by the state and generation it observes:

| Observed state | Required terminal behavior |
|---|---|
| `healthy` at the captured generation | Continue the documented read, load, mutation, or population path. |
| `healthy` at a newer generation, or `repairing` that began from healthy | Return an already-read or already-loaded value uncached, or return the defined outcome of an already-admitted remote mutation without L1 population. Start no additional cache side effect without a new successful admission ticket; the overlapping operation linearizes before the transition. |
| `blocking`, `blocked`, or `repairing` that began from blocked | Return `ReasonLocalBlocked` without a value and without starting another cache side effect. |

An operation releases every healthy or maintenance lease before waiting for a
state transition, key token, Redis result, loader, or repair. It never waits
while holding a local-state lease. The table governs ordinary read, load,
mutation, and population paths; the explicitly administrative
`InvalidateLocal` maintenance delete and blocked-state `Clear` paths remain the
only documented side-effect exceptions.

`ClearLocal` and mandatory full-local cleanup enter `repairing`, increment the
generation, deny new leases, and context-sensitively drain existing leases
before calling `Local.Clear`. The single timeout budget covers state/barrier
admission, lease drain, and the L1 cleanup call. Timeout or failure publishes
`blocked`. Each repairing transition owns a unique repair epoch. Successful
explicit `ClearLocal` publishes `healthy` only through a compare-and-swap that
still owns that epoch. Mandatory full cleanup opened by `TieredCache.Clear`
records origin when that local cleanup is admitted, not when the outer `Clear`
method began. It returns to `healthy` only when admitted from healthy state and
still owns the same epoch; if admitted from blocked state or an intervening
block replaced its epoch, success preserves `blocked`. A successful explicit
`ClearLocal` that completes during the outer remote-scan phase is therefore a
newer repair: the later mandatory cleanup observes healthy at admission and an
older `Clear` start snapshot cannot re-block it. This removes the
check-then-write and accidental-heal windows without requiring a goroutine that
outlives the method.
The caller-provided L1 must not re-enter the same `TieredCache` from an L1
method.

A maintenance lease permits `Local.Delete` while state is healthy or blocked
but never serves data and never unblocks the decorator. It cannot enter during
repairing state. This lets later invalidations reduce stale residue without
weakening fail-closed behavior. Explicit invalidation that cannot acquire its
key token or maintenance/repair admission before its budget expires counts as
failed and atomically establishes or retains blocked state.

The operation token plus local-state fence provides the following
linearization guarantees:

- an old L2 read cannot populate L1 after a completed same-key `Set`, `Delete`,
  or `InvalidateLocal`;
- concurrent same-key mutations reach Redis and L1 in operation-token order,
  so a delayed earlier local commit cannot overwrite a later Redis value;
- an RESP3 invalidation that completes after a refill has removed that refill,
  and no pre-invalidation refill can resurrect it afterward.

An operation whose L1 post-check completed before a state transition may
linearize before it. An operation still waiting on a key coordinator, Redis,
or an L1 method must observe the newer generation/state before returning a
value, starting a later loader/mutation side effect, or using a population. An
observed healthy generation mismatch may return the already-read or
already-loaded value uncached and linearize that overlapping operation before
the full-clear transition; it may not start another cache side effect. An
observed blocked state returns `ReasonLocalBlocked` instead. An already-admitted
operation may preserve its original miss, loader error, or provider error as
the cause of `ReasonLocalBlocked`, but the blocked reason is the public terminal
classification. A refill or mutation that captured an older generation may
finish already-admitted remote work but cannot create a usable
post-clear/post-block local hit.

The L1 remains caller-owned for lifecycle purposes, but mutation and reads are
exclusively transferred to one `TieredCache` after construction. The caller
must not access it directly or share it with another decorator. It must provide
linearizable per-key `Get`/`Set`/`Delete`, concurrency-safe `Clear`, honor
context cancellation, and avoid callbacks into the decorator. Multiple
decorators may share the same `ValueCache` L2, but their coordinators,
local-state generation, blocked state, pointer identity, and invalidation are
process-instance-local.

## Mutation Behavior

### `Set`

`ValueCache.Set` validates and serializes the value, rejects a result larger
than `MaxValueBytes`, and only then dispatches Redis `SET`. An oversized value
therefore cannot mutate either tier.

`TieredCache.Set` holds the same-key operation token for the complete operation:

1. Acquire a healthy local-state lease, fail closed if blocked, and capture the
   current generation plus a one-shot Redis `SET` admission ticket, then
   release the lease.
2. Record a monotonic start immediately before invoking the admitted
   `ValueCache.Set`.
3. After known L2 success, acquire a healthy lease for the captured generation
   and recheck caller cancellation.
4. If the generation is still current, write the original `V` into L1 using
   the normalized/elapsed adjusted TTL formula and post-check
   state/generation. A
   healthy generation mismatch skips L1 and returns success; blocked state
   returns `ReasonLocalBlocked` after preserving the known L2 success.

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

Public `InvalidateLocal` acquires the per-key operation token. Internal
mandatory single-key cleanup is a separate `invalidateLocalHeld` path whose
caller already owns that token; it must not reacquire it. Its owned timeout
starts before maintenance-lease admission and covers only local-state wait and
`Local.Delete`. This prevents non-reentrant self-deadlock on commit-unknown,
post-success cancellation, or failed L1 population.

### `Delete`

`ValueCache.Delete` removes the L2 key and treats an absent key as success.
`TieredCache.Delete` holds the same-key operation token, checks blocked state
through a healthy local lease, atomically admits one Redis `DEL`, releases that
lease, and then invokes the admitted L2 delete. Once the Redis `DEL` call has
been invoked, it
always performs the token-held mandatory L1 invalidation before returning.
Errors are joined without hiding the Redis mutation outcome or L1 failure.
Validation, blocked state, or cancellation detected before Redis invocation
returns without mutating either tier.

### `Clear`

`ValueCache.Clear` asks `SCAN MATCH` to return only its namespace and sends
matching keys to `UNLINK` in batches no larger than `ClearBatchSize`. Redis
still traverses the full selected database keyspace. Every returned page is
processed immediately and, when Redis returns more matching keys than the
configured limit, re-chunked before `UNLINK`; the implementation never
accumulates all matches in memory.
Every server-side scan uses the exact pattern
`bluetape:cache:value:<validated-namespace>:*` with
`COUNT ClearBatchSize`; it does not scan the database and filter locally.
Redis `SCAN COUNT` is only a hint. A page of `n` keys costs one `SCAN` plus
`ceil(n/ClearBatchSize)` sequential `UNLINK` commands/round trips. The client
retains at most one server-returned page and one `UNLINK` argument chunk at a
time, but does not impose a hard byte bound on that Redis-controlled page or on
external key lengths. It never calls `FLUSHDB` or falls back to blocking
`DEL`.

Total round trips are one `SCAN` for every cursor iteration, including empty
matched pages, plus the sum of those per-page `UNLINK` chunks. No complexity
claim is based only on namespace cardinality.

`TieredCache.Clear` attempts remote clear and, once the remote clear has begun,
always invokes the common mandatory full-local-clear primitive. That primitive
enters repairing state, increments the generation, drains existing leases,
calls `Local.Clear` within the owned cleanup budget, and blocks the decorator
on failure. It does not unblock a decorator that another operation blocked;
successful explicit `ClearLocal` remains the only user-invoked repair. If a
newer block is preserved despite successful local clear, or if mandatory local
cleanup was admitted from blocked state, `TieredCache.Clear` returns
`ReasonLocalBlocked` rather than nil. If a concurrent explicit `ClearLocal`
healed the decorator before mandatory cleanup admission, that newer repair
wins and the later cleanup may return healthy. Remote and local errors are
joined without silently declaring success. Validation or cancellation detected
before the first remote command returns without mutating either tier.

Namespace clear is:

- context-cancellable;
- idempotent for already removed keys;
- non-atomic and not a snapshot;
- proportional to Redis `SCAN` iterations over selected-database cardinality
  plus `UNLINK` chunks for matching namespace keys;
- intended for administration, tests, and one-provider/one-decorator reset,
  not request hot paths.

Keys added concurrently can escape a scan iteration, and callers must not use
`Clear` as a transaction or authorization boundary.

`ValueCache.Clear` is L2-only. `TieredCache.Clear` additionally clears only
that calling decorator's L1 and cannot clear or repair another process-local
decorator sharing the namespace. Without fleet coordination, other L1s may
serve their existing values for up to `LocalTTL`, and concurrent writes may
survive the non-atomic Redis scan. An exact fleet reset requires an
application-owned runbook: fence and quiesce readers/loaders/writers across all
instances, perform the admin-scoped L2 clear through the stable primary, fan
out `ClearLocal` to every live decorator and confirm each success, then resume
traffic. Without that fence and fanout, namespace clear is only a best-effort
L2 deletion plus optional cleanup of the calling decorator.

### Mandatory Local Cleanup and Recovery

Safety cleanup after a remote mutation does not reuse a canceled caller
context. `TieredCache` creates a package-owned `context.Background()` child
with `TieredConfig.LocalCleanupTimeout` before attempting local-state admission.
For token-held single-key cleanup, that budget covers maintenance admission
and `Local.Delete` without another coordinator wait. For mandatory full clear,
it covers repairing-state admission, lease drain, and `Local.Clear`. The
supplied L1 must honor context cancellation; the package does not spawn a
detached goroutine to force an uncooperative implementation to stop.

If mandatory cleanup fails or its cleanup context expires, `TieredCache`
atomically enters a local-blocked state. A failed `InvalidateLocal` or
`ClearLocal` also blocks the decorator because an externally requested
invalidation may not have removed stale data. While blocked, `Get`,
`GetOrLoad`, `Set`, and `Delete` fail closed with `ReasonLocalBlocked` instead
of serving potentially stale L1 data. Administrative `Clear` remains
available: it attempts the namespace remote clear and mandatory full local
clear. Cleanup admitted while still blocked preserves blocked state even when
both tiers are cleaned and returns `ReasonLocalBlocked`, so the caller must
explicitly complete a successful `ClearLocal` repair. A concurrent successful
`ClearLocal` that wins before cleanup admission is the newer repair and is not
undone by the older outer `Clear` invocation.
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
- A nil or zero/uninitialized `TieredOptions.Remote` returns `*CacheError` with
  `ReasonConfiguration` before the constructor reads remote configuration.
- Invalid namespace construction returns `*CacheError` with
  `ReasonConfiguration` and preserves `redis.ErrInvalidKey` for `errors.Is`.
  Invalid per-call logical keys and negative per-call TTLs return errors that
  satisfy `errors.Is(err, redis.ErrInvalidKey)` and
  `errors.Is(err, redis.ErrInvalidTTL)` respectively, before serializer, tier,
  coordinator, or loader work. These caller-input sentinels are non-retryable.
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
#535 remains an explicit tiered cache with local-TTL-bounded serving after L1
population but no Redis-expiry-relative bound for existing L2 hits; it is not
advertised as a coherent multi-process near cache. The separate existing
Pub/Sub strategy remains available for its own local-cache contract, not as a
direct wrapper around `TieredCache`.

## Failure Modes and Required Response

| Failure mode | Required response |
|---|---|
| Serializer rejects a value | Return an inspectable serialization error; do not call Redis or update L1. |
| Serialized value exceeds `MaxValueBytes` | Reject before Redis dispatch or L1 update. |
| Redis read returns malformed or oversized bytes | Return an invalid/oversized payload error; do not treat it as a miss or auto-delete it. |
| Redis mutation invocation returns a non-nil command error and outcome is unknown | Return `redis.ErrCommitUnknown` and perform required local invalidation. |
| Redis reports known mutation success, then caller cancellation is observed | Return the context/cleanup error, not `redis.ErrCommitUnknown`; preserve the known remote success and invalidate L1 when required. |
| L1 update fails after L2 success | Invalidate L1, return the L1 error, and leave L2 as the recovery source. |
| Mandatory L1 cleanup fails or times out | Enter local-blocked state; fail ordinary data operations until successful `ClearLocal`; administrative `Clear` may clean but cannot heal. |
| Loader succeeds but L2 write fails | Return the L2 error and do not populate L1. |
| L1 TTL is configured above the finite default L2 TTL | Reject construction; per-entry overrides cap the effective local TTL. |
| Existing L2 hit has unknown remaining Redis TTL | Permit at most `LocalTTL` of serving after accepted L1 population, but make no Redis-expiry-relative stale bound; only invalidation closes it deterministically. |
| Current call writes a known positive Redis TTL | Normalize wire precision, subtract monotonic elapsed time, and skip L1 when the adjusted remainder is non-positive; document residual L1 anchor skew. |
| Namespace clear is interrupted | Return cancellation plus redacted partial progress; `TieredCache.Clear` still clears only its calling decorator's L1. |
| An admin `ValueCache.Clear` runs while other decorators are live | Clear only L2; other process-local L1 entries and blocked states remain until expiry or explicit per-instance repair. Exact fleet reset uses the documented traffic fence and `ClearLocal` fanout. |
| Pipeline, transaction queue, cluster, ring, or opaque `Cmdable` wrapper is considered | The concrete `*redis.Client` API makes the unsupported executor shape unrepresentable. |
| Failover client/proxy or a primary change is considered | Unsupported even when exposed as `*redis.Client`; `Clear` requires a direct endpoint with stable server identity for the entire cursor traversal. |
| Existing Pub/Sub strategy is given a TieredCache as Local | Unsupported composition; document the local-only adapter boundary rather than deleting L2 on invalidation. |
| Future tracking connection becomes ambiguous | #536 must clear L1 and re-establish tracking before serving tracked local hits. |

## Operational Contract

#535 targets Redis 6 or newer with one writable primary reached through a
direct endpoint whose server identity remains stable for each operation. The
caller owns Redis client creation, authentication, TLS,
dial/read/write/pool timeouts, and readiness checks. Automatic failover clients
or proxies are unsupported because a primary change invalidates the
server-local `SCAN` cursor and can silently produce partial namespace clear.
The Redis identity must be authorized for `GETRANGE`,
`EXISTS`, `SET`, `DEL`, `SCAN`, and `UNLINK` on the configured namespace.
`UNLINK` is required so namespace clear does not degrade into a blocking delete.
Deployments should use a dedicated Redis identity, the narrowest feasible
key-prefix/command ACL, and verified server certificates when TLS is enabled.
Redis key ACLs do not scope `SCAN MATCH` enumeration because its pattern is not
a key argument; a credential granted `+SCAN` can observe key names outside the
configured prefix in the selected database. Deployments requiring key-name
confidentiality between principals must isolate them in dedicated Redis
instances or separately enforced service endpoints whose backend identity is
stable. Redis logical
databases selected with `SELECT` are not a security boundary: ACLs are not
scoped by logical database, and a `SCAN` credential can enumerate the selected
database. A runtime identity without `SCAN` may use ordinary cache operations
while a separately constructed admin-scoped cache instance, connected directly
to a stable primary endpoint, performs `ValueCache.Clear`. That identity clears
only L2 and cannot invalidate or repair process-local decorators. Neither
identity needs or should receive `FLUSHDB`,
`FLUSHALL`, or unrelated administrative commands.

Operators must size Redis memory and eviction policy for the chosen TTLs. A
zero remote TTL means the package never expires that key; namespace rotation
or explicit `Clear` is then required to reclaim old data. Readiness must prove
the cache's actual command path, while an application may choose whether cache
readiness is startup-fatal based on its own availability contract. #535 does
not conceal provider failure by serving a stale local fallback.

`InvalidationWaitTimeout` must exceed the application's expected same-key
loader and Redis-command latency. Expiry while waiting for a key token blocks
the entire decorator by design, because the invalidation was not proved.
Long-running loaders therefore require an explicit override and regression
test; issue #536 must also ensure its tracking consumer does not silently lose
subsequent invalidations while one key waits.

`LocalCleanupTimeout` must exceed the worst expected active-lease drain plus
L1 `Delete` or `Clear` latency. Large or slow caller-owned L1 implementations
must override the one-second default. Exhaustion produces
`ReasonLocalBlocked`, requires an alert, and requires an explicit successful
`ClearLocal` recovery before ordinary data operations resume.

The package emits no metrics or logs. Callers use go-redis hooks and returned
errors to measure command latency and provider failure, alert on
`ReasonLocalBlocked` and `ReasonPartialClear`, and record redacted
`ClearProgress`. A partial clear retry starts a new scan at cursor zero; the
reported progress is diagnostic and not a resumable snapshot.

Cluster and ring deployments are outside the accepted constructor type;
failover clients and proxies that still present as `*redis.Client` are outside
the documented topology contract. Supporting dynamic topology later requires
an explicit design for per-primary `SCAN`, slot-safe deletion, fenced topology
changes during clear, and aggregate partial-progress reporting.

## Testing Strategy

### Unit tests with fakes

- nested default config, independent value/tiered per-cache overrides,
  validation, and mutation isolation;
- nil client plus nil and typed-nil interface dependencies, safe
  constructor-only zero values, and a public API whose concrete client type
  excludes pipelines, transaction queues, clusters, rings, and opaque wrappers;
- nil and zero/uninitialized `TieredOptions.Remote` rejection before remote
  TTL access or decorator state retention;
- nil loader rejection before coordinator creation, tier access, or any Redis
  command;
- invalid namespace, logical-key, and per-call TTL errors preserve the exact
  `redis.ErrInvalidKey`/`redis.ErrInvalidTTL` identities documented above;
- logical-key preservation, adversarial namespace glob rejection, namespace
  collision resistance, length limits, and redaction;
- deterministic redacted IDs never expose injected raw keys/secrets through
  `Error()`, while explicit `errors.As`/`Unwrap` still preserves causes;
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
  releases the leader participant on publication, follower participants on
  receive/cancellation, and then inactive coordinator/flight entries;
- loader and serializer failures are not cached;
- Redis-first `Set` ordering and original-reference L1 population;
- retained pointer aliases from `Set` and loader results follow the documented
  immutable-snapshot rule;
- finite, zero, negative, default, and per-entry TTL behavior, with
  `LocalTTL` after pre-existing L2 population and monotonic elapsed-time
  subtraction for known positive writes, including EX/PX precision,
  sub-millisecond minimum, fractional-millisecond truncation, Local.Set delay,
  and non-positive remainder;
- bounded and empty payload behavior, including `MaxValueBytes + 1`, the
  64-MiB configuration cap, and oversized `Set` rejection before Redis;
- exact error mapping for miss, marshal, unmarshal, oversized payload, Redis,
  commit-unknown, local failure, local-blocked, and partial clear;
- commit-unknown and mandatory local invalidation behavior using a fresh
  cleanup context;
- token-held mandatory invalidation never reacquires its coordinator token and
  distinguishes unknown Redis outcome from known success plus later
  cancellation;
- L2 refill/load `Local.Set` failure always performs token-held delete, joins
  failures, and blocks when that cleanup cannot be proved;
- cleanup timeout/failure enters local-blocked state, ordinary operations fail
  closed, and successful `ClearLocal` repairs the cache;
- the complete healthy/current, healthy/new-generation,
  repairing-from-healthy, blocking/blocked, and repairing-from-blocked terminal
  state table, including proof that no wait occurs while holding a local-state
  lease;
- initial admission grants healthy, waits context-sensitively and retries for
  repairing-from-healthy without a lease/token, fails closed for blocked-origin
  states, and leaves cache state unchanged on caller cancellation;
- administrative `Clear` begun while blocked still attempts remote and local
  cleanup; cleanup admitted from blocked preserves blocked state and requires
  a later successful `ClearLocal`, while a concurrent explicit repair that
  wins before admission is not undone;
- two decorators sharing L2 prove that one decorator or admin
  `ValueCache.Clear` does not remove the other's L1 hit or blocked state, and
  that explicit `ClearLocal` fanout is required for fleet reset;
- independent invalidation-wait and local-cleanup defaults/overrides, including
  invalidation wait expiry behind a long same-key loader;
- failed direct `InvalidateLocal`/`ClearLocal` blocks the decorator, and a
  successful single-key invalidation does not unblock it;
- cancellation while invalidation waits for a key token or maintenance/repair
  admission blocks the decorator;
- `Delete`, `ClearLocal`, `InvalidateLocal`, and namespace `Clear` error joining;
- each `SCAN` uses the exact MATCH/COUNT contract, pages are streamed
  immediately, oversized pages are re-chunked, every `UNLINK` batch stays
  bounded, and no whole-namespace slice is retained;
- cancellation before dispatch, during load, after load, and between clear
  batches, with no late L1/L2 write.

### Redis Testcontainers

- set/get/delete/miss against a real Redis service;
- finite TTL expiration and explicit zero-TTL persistence;
- finite default/per-entry TTL validation, the absence of a Redis-expiry bound
  for existing L2 hits, and adjusted remaining-TTL population for known writes
  under injected response/admission/Local.Set delay;
- namespace isolation and `SCAN` plus `UNLINK` clear;
- restricted ACL identities proving allowed cache commands, denied foreign
  prefix `GET`/`UNLINK`, denied `FLUSHDB`/`FLUSHALL`, and the documented fact
  that `+SCAN` can enumerate foreign-prefix key names in the selected database;
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
- publication-versus-follower-cancellation arbitration under the coordinator
  mutex, including a latch where completion and cancellation become ready
  together and exactly one path releases the participant;
- leader cancellation before key-token acquisition publishes/closes/detaches
  the flight and leaves no coordinator references;
- coordinator final-retirement ABA against a new same-key registry lookup,
  proving identity-checked removal and one operation-token domain;
- constant-size flight state and allocation-free healthy L1-hit admission via
  `testing.AllocsPerRun`;
- different-key healthy L1 hits proceed concurrently without creating per-key
  coordinators or holding a global exclusive lock across `Local.Get`;
- concurrent different-key loads;
- deterministic latch tests for read/refill versus `Set`, `Delete`,
  `InvalidateLocal`, and `ClearLocal`, proving no stale resurrection or
  mutation-order reversal;
- pause after generation capture, complete `ClearLocal`, then resume the
  refill, proving state leases prevent a usable post-clear `Local.Set`;
- capture generation in `Set` and `GetOrLoad`, complete `ClearLocal`, then
  resume remote/loader completion, proving healthy mismatch skips L1 and
  blocked state stops later side effects;
- pause after one-shot admission but before invoking the loader, Redis `SET`,
  and Redis `DEL`, then transition state, proving the admitted call may finish
  but no later side effect or usable L1 population crosses the fence;
- block transition while a key-token waiter or L2 read is active, proving no
  post-block L1 hit return or population;
- hold L1 readers/writers across the cleanup deadline, proving the total
  admission/drain/cleanup budget returns blocked without admitting new L1
  operations;
- `TieredCache.Clear` versus a delayed refill, proving the common mandatory
  full-local-clear primitive fences old generations, while documenting that a
  concurrent loader may write an L2 value after its key's cursor position so
  the remote survivor is accepted but L1 remains cleared or unusable;
- mandatory-full-clear repair epochs return healthy only without an
  intervening block; stale repair owners cannot heal a newer block;
- begin administrative `Clear` while blocked, pause its remote phase, complete
  a successful `ClearLocal`, then admit the mandatory local phase, proving the
  newer explicit repair wins and the older outer call cannot re-block it;
- concurrent `Get`, `Set`, `Delete`, `ClearLocal`, and cancellation without
  races, retained coordinators/flights, or usable late local writes;
- active coordinator registry returns to zero after stress, failure waves, and
  canceled waiters;
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
  client shapes, including failover/proxy clients that share the concrete type;
- recommended `VersionedSerializer` use and namespace rotation for
  incompatible wire changes.

## Documentation

Add synchronized `cache/redisvalue/README.md` and `README.ko.md` documenting:

- L1 object/reference and L2 serialization boundaries;
- exclusive post-construction L1 access and instance-local coordination/
  invalidation guarantees;
- defaults and per-cache override;
- first-leader flight policy: the leader's loader, TTL, and context govern
  already-registered followers, while follower cancellation affects only that
  follower;
- strict Redis-first mutation behavior;
- pointer immutability guidance;
- TTL and bounded-staleness behavior;
- operational cost and non-atomic semantics of `Clear`;
- L2-only admin clear, calling-decorator-only local clear, and the
  fence/quiesce -> L2 clear -> per-instance `ClearLocal` -> resume fleet-reset
  runbook;
- sequential `SCAN`/`UNLINK` round-trip cost; bounded internal pipelining is
  deferred to avoid queued-success and partial-progress ambiguity in #535;
- namespace ownership as one exclusive tenant/security/wire-format clear
  domain, including same-namespace cross-instance deletion;
- cancellation and commit-unknown behavior;
- caller serializer panic-free/resource-bound requirements and the distinction
  between Redis write admission size and serializer allocation;
- required Redis commands (`GETRANGE`, `EXISTS`, `SET`, `DEL`, `SCAN`, and
  `UNLINK`), Redis 6+ single-primary topology, ACL/TLS ownership, caller
  dial/read/write/pool timeouts, readiness, memory eviction policy, and
  zero-TTL cleanup responsibility;
- `InvalidationWaitTimeout` and `LocalCleanupTimeout` sizing, blocked-state
  alert/recovery, go-redis hook metrics, and cursor-zero partial-clear retry;
- why key-prefix ACLs do not restrict `SCAN` enumeration, why Redis logical
  databases are not security boundaries, and when a dedicated instance,
  separately enforced endpoint, or separate clear-admin identity is required;
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
   `ValueCache[V]` and `TieredCache[V]` types through `NewValueCache` and
   `NewTieredCache`; the tiered constructor rejects nil/uninitialized L2.
2. `ValueCache[V]` implements `cache.Cache[string,V]` using a caller-owned
   go-redis `*redis.Client` and `serialization.Serializer[V]`.
3. `TieredCache[V]` implements `cache.LoadingCache[string,V]`, stores `V`
   directly in L1, and serializes only through L2.
4. `DefaultConfig` provides independent value/tiered defaults, is copied on
   construction, and can be overridden per cache without global mutable state.
5. Healthy L1 hits make no Redis or serializer call, allocate no lease state,
   and create no key coordinator; accepted L2 population stores the same
   decoded `V` into L1.
6. Loads follow L1 -> L2 -> loader, collapse through TieredCache-owned
   context-aware flights that share success or error with existing waiters,
   atomically retire inactive coordinator entries without ABA, and write L2
   before L1 population.
7. TTL defaults and overrides satisfy the documented finite/zero relationship;
   existing L2 hits make no Redis-expiry-relative bound, while known positive
   writes normalize wire precision and subtract monotonic elapsed time to
   reduce, but not claim elimination of, cross-tier skew.
8. Payload reads and Redis write admission, namespace `UNLINK` chunks, retained
   clear page count, logical-key inputs, errors, and cancellation are bounded
   and redacted as specified; Redis controls one returned `SCAN` page/key-byte
   size and the serializer controls its own allocations. Invalid namespace,
   logical-key, and TTL errors preserve the shared Redis sentinel identities.
9. Unknown mutation outcomes preserve `redis.ErrCommitUnknown`; known success
   plus later cancellation does not. Token-held cleanup invalidates L1 without
   reacquiring its own coordinator token.
10. Same-key reads, loads, mutations, and invalidations are linearized; the
    context-aware local-state leases, one-shot side-effect tickets, state
    machine, and generations prevent stale local resurrection after clear or
    block transitions.
11. Mandatory cleanup uses one owned timeout budget across local-state
    admission/drain/cleanup and blocks ordinary cache use until successful
    `ClearLocal` repair if cleanup fails; repair epochs ensure the newest
    admitted repair transition determines final health.
12. `InvalidateLocal` and `ClearLocal` never mutate Redis and are suitable for
    the #536 spike boundary; current Pub/Sub invalidation never directly wraps
    the TieredCache mutation surface. Namespace clear affects only L2 plus the
    calling decorator's L1; exact fleet reset requires the documented fence and
    per-instance `ClearLocal` fanout.
13. The public constructor accepts only synchronous single-primary
    `*redis.Client`; queued, cluster, ring, and opaque `Cmdable` executors are
    unrepresentable. Direct stable-primary topology remains a documented
    caller precondition because the concrete type cannot distinguish a
    failover/proxy client whose server identity may change during `Clear`.
14. Sharing a namespace across tenants/incompatible schemas or accessing one
    L1 outside its sole decorator is unsupported and invalidates the documented
    safety guarantees; the constructor does not claim to detect either misuse.
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
| Step 2-R review | Done | Exact commit `4e6758d`: performance `P0=0/P1=0/P2=0`; stability `0/0/0`; security `0/0/1`; operator/Ops `0/0/0`; developer/API `0/0/1`; user/caller lane timed out and main integration fallback found `0/0/0`. Final integrated verdict: `P0=0/P1=0`. |
| Written spec review | Pending | Awaiting explicit user approval before invoking `writing-plans` and beginning Step 3. |
