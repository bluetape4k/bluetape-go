# Go-native Apache Fory Codecs for Redis Stampede Coordination

Issue: #597
Parent: #596
Research: #595
Status: Approved; implementation plan in progress
Date: 2026-07-10

## Goal

Add opt-in Go-native Apache Fory codecs that satisfy `rediscoord.Codec[V]`.
They serialize only the inner owner-result payload used by
`cache/rediscoord.StampedeCache`; the existing JSON/base64 owner-result
envelope, Redis keys, owner-token protocol, lock, result TTL, and cache-load
semantics remain unchanged.

## Constraints

- Pin `github.com/apache/fory/go/fory` at `v1.3.0` (Apache-2.0; upstream Go
  module declares Go 1.25). Do not rely on documentation defaults.
- All runtimes use `fory.WithXlang(false)`. This provider does not decode or
  write xlang data.
- Native fast and native compatible payloads are distinct formats. JSON has no
  fallback path and existing values fail closed after a caller changes codec or
  schema generation.
- This is `TRUSTED_INTERNAL` cache data only. It does not add arbitrary-object
  decoding, dynamic post-construction registration, JWT state storage, or a
  general `serialization` default.
- Fory v1.3.0's `threadsafe.Fory` factory cannot return registration errors.
  This first provider uses one registered Fory runtime guarded by a mutex. A
  future pool optimization requires a separate error-returning acquisition
  design and benchmark evidence.

## Package And API

Create `cache/rediscoord/fory` with package name `rediscoordfory`. Keeping the
provider in a child package prevents existing `rediscoord` consumers from
importing Apache Fory unless they select this codec explicitly.

```go
type Registration func(*fory.Fory) error

type Options struct {
    Register                         Registration
    MaxPayloadBytes                 int
    MaxDepth                        int
    MaxTypeFields                   int
    MaxTypeMetaBytes                int
    MaxSchemaVersionsPerType        int
    MaxAverageSchemaVersionsPerType int
}

type Codec[V any] struct { /* immutable after construction; zero value is invalid */ }

func NewNativeFast[V any](options Options) (*Codec[V], error)
func NewNativeCompatible[V any](options Options) (*Codec[V], error)

func (c *Codec[V]) Marshal(value V) ([]byte, error)
func (c *Codec[V]) Unmarshal(payload []byte) (V, error)
```

`Options.Register` is mandatory and must be deterministic: it registers the
root and all declared Fory struct/enum/extension types. The constructor applies
it once and returns any registration error before the codec is usable. Marshal
and unmarshal lock the one immutable runtime for the duration of the Fory call;
the provider copies Fory's returned marshal bytes before unlocking.

The provider has no mutation API. Registration happens before the codec becomes
available to callers, so concurrent marshal/unmarshal operations cannot race
with type registration.

The root `V` shape is part of the contract. The first implementation accepts
non-pointer Go primitives, structs, `string`, and `[]byte` values that pass the
Fory v1.3.0 typed round-trip tests. Pointer, interface, function, channel, and
unsafe-pointer roots are rejected by the constructor, including nil pointers.
Cycles and shared pointer identity are unsupported because reference tracking is
disabled. A zero-value `Codec[V]` returns a typed `uninitialized` error from both
methods; callers must use a constructor.

## Wire Contract

The inner codec writes a small binary envelope before Fory bytes:

| Field | Size | Purpose |
| --- | --- | --- |
| Magic | 4 bytes: `BTFY` | Reject JSON, raw Fory, and unrelated values before Fory decode. |
| Codec version | 1 byte: `1` | Evolves this wrapper independently from Fory. |
| Profile | 1 byte | `1` native-fast; `2` native-compatible. |
| Payload length | 4-byte unsigned big-endian | Allows exact truncation/trailing-byte and maximum-size validation. |
| Fory payload | declared length | Bytes returned by the mutex-guarded Fory runtime. |

`NewNativeFast` uses exactly `WithXlang(false)`, `WithCompatible(false)`,
`WithTrackRef(false)`, and caller-selected bounded `WithMaxDepth`. It is for
fixed-schema volatile cache values; callers rotate the `rediscoord` namespace
when changing the struct/schema generation.

`NewNativeCompatible` uses exactly `WithXlang(false)`, `WithCompatible(true)`,
the same explicit reference policy, and caller-selected bounded `WithMaxDepth`.
It supports Fory-compatible field evolution but does not make semantic changes
or incompatible type changes safe. Callers still rotate the namespace for
those changes.

All Fory resource limits are explicit options, not Fory defaults. The caller
owns sizing because cache value shape is application-specific. A documented
starting profile is `MaxPayloadBytes=1 MiB`, `MaxDepth=20`,
`MaxTypeFields=512`, `MaxTypeMetaBytes=4096`,
`MaxSchemaVersionsPerType=10`, and `MaxAverageSchemaVersionsPerType=3`.
Applications must lower or raise these only with fixture and memory-impact
evidence. A limit rejection is an expected typed error and should be counted
by caller metrics; the provider does not log globally or retry it.

`MaxPayloadBytes` applies before Fory decode and excludes the wrapper header.
It also applies immediately after Fory marshal and before wrapper allocation:
the codec rejects an encoded payload larger than the configured maximum and
must not construct the wrapper or pass it to the owner-result envelope. The
codec rejects non-positive configuration, an input larger than header plus the
configured maximum, unknown magic/version/profile, mismatched profile,
declared-length mismatch, truncation, and trailing bytes.

## Error Contract

Export `CodecError` with stable `Operation()`, `Profile()`, and `Reason()` methods.
Reasons are `configuration`, `uninitialized`, `registration`,
`payload-too-large`, `invalid-magic`, `unsupported-version`,
`profile-mismatch`, `length-mismatch`, `unsupported-value`, and
`fory-failure`. Its formatted message must not include payload bytes, caller
Redis key, owner token, registration error text, or raw Fory cause text. It may
unwrap the safe underlying cause for `errors.Is`/`errors.As`. Redis operation
error wrapping remains owned by `cache/rediscoord` and `btredis.OpError`.
Callers may use operation/profile/reason as low-cardinality metrics or tracing
labels; the provider has no global logger.

## Data Flow

1. The load owner calls `Codec.Marshal(value)`.
2. The Fory codec locks its registered runtime, serializes a value, copies the
   returned Fory bytes while still holding the lock, and unlocks.
3. The codec rejects an oversized Fory byte slice, then allocates exactly one
   bounded `header+payload` wrapper. It does not copy the payload again during
   unmarshal header validation; the validated payload is a slice of the input.
4. Existing `rediscoord.encodeResult` wraps those bytes in its current JSON
   owner-token envelope and writes the short-TTL Redis result key.
5. A waiter reads the existing result envelope, validates the Fory envelope,
   decodes the payload, then fills its local cache as before.

The outer JSON/base64 envelope remains the existing coordination format, but
`rediscoord.Options` gains an optional `MaxResultBytes` guard. When positive,
`storeResult` rejects an oversized encoded envelope before Redis publication
and `readOwnerResult` rejects it before `json.Unmarshal`. Its zero default
preserves existing callers. This outer limit is separate from the inner Fory
`MaxPayloadBytes` limit and protects the JSON/base64 allocation boundary.

The rollout boundary is the `Namespace` plus codec profile and schema
generation. Every process sharing a namespace must use the same tuple. Mixed
JSON/Fory, fast/compatible, or registration sets in one namespace are
unsupported and are not repaired with fallback decoding.

## Tests

Use test-driven development in `cache/rediscoord/fory`:

- A native-fast registered Go struct round-trips and satisfies
  `rediscoord.Codec[V]`.
- Native-compatible registered structs round-trip and demonstrate a compatible
  added-field fixture.
- Fast accepts neither compatible wrapper bytes nor altered profile/version
  bytes; both profiles reject JSON/raw Fory bytes and wrong declared lengths.
- Malformed/truncated/oversized input returns the typed provider error and its
  formatted text contains no payload, Fory-cause, registration, key, or token
  marker.
- Marshal rejects an oversized Fory result before allocating the `BTFY`
  wrapper; the result envelope is never constructed for that value.
- Invalid options and failed registration return constructor errors before the
  codec is usable.
- Constructor tests cover supported struct/scalar/bytes roots, reject pointer
  and unsupported roots, and verify zero-value codec errors.
- A high-contention concurrent marshal/unmarshal test passes under `-race` and
  verifies each decoded value. No Testcontainers test is needed for the codec
  itself because existing `rediscoord` integration tests own the Redis lock
  protocol; an integration case must cover the optional `MaxResultBytes` guard.
- The provider uses one mutex-guarded runtime. This makes registration failure
  impossible after construction and gives callers a typed error instead of a
  pool-factory panic. #599 measures mutex contention and compares it with JSON;
  a pool is a follow-up only if measured workload justifies it.
- Existing `cache/rediscoord` behavior tests remain passing; tests for the
  additive `MaxResultBytes` guard cover both configured and zero-default paths.
- Add `MaxResultBytes` option tests that reject oversized encoded owner-result
  bytes before JSON decode and before Redis publication when configured; zero
  retains the current unlimited behavior.

## Documentation And Benchmark Scope

Update `cache/rediscoord/README.md` and `README.ko.md` with constructor
examples, `TRUSTED_INTERNAL`, native-only mode, fast versus compatible limits,
registration lifecycle, explicit resource-limit starting values, supported
value shapes, sanitized error reasons, and namespace rotation. Add compile-
checked `ExampleNewNativeFast` and `ExampleNewNativeCompatible` tests and keep
README snippets aligned with those examples. State that Fory is not encryption:
Redis and operators can observe the coordination bytes, so ACL/TLS and
namespace isolation remain required for sensitive values.

The runbook must use a versioned namespace owned by the caller, for example
`<base>:fory-native-fast:<schema-generation>`. Never mix JSON, native-fast, and
native-compatible values in one namespace. For a rollout, publish the new
namespace/configuration, switch readers and writers together, and let the old
namespace drain for at least `LockTTL + ResultTTL + operator safety margin`.
Rollback means switching back to the previous codec/namespace pair; it does
not rewrite or decode new values with the old codec. Cleanup uses a bounded
operator-owned `SCAN MATCH` with a count and TTL-aware deletion, never `KEYS`.
Bad-profile, schema, and payload-limit errors should be inspected through
caller metric labels and sanitized error reasons, without logging payloads,
keys, owner tokens, registration text, or raw Fory causes.

No benchmark results are added in #597. #599 owns a same-condition benchmark
and must publish the raw command/environment, result table, Chart, and written
analysis before any throughput or storage claim.

## Alternatives Rejected

- Adding Fory directly to `cache/rediscoord`: this would make Apache Fory part
  of the normal package import surface and weakens opt-in dependency ownership.
- Using `threadsafe.New(...).RegisterStructByName(...)`: registration reaches
  only one pooled runtime and has no error-returning factory path. The first
  implementation uses a mutex-guarded runtime instead.
- Writing raw Fory bytes with no wrapper: the caller could not distinguish
  native-fast, native-compatible, stale JSON, or malformed cache content before
  entering Fory.
- Treating this as a direct Redis L2 value cache: `rediscoord` is a short-TTL
  stampede-coordination protocol; #598 owns the separate direct-value boundary.
