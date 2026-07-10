# Go-native Apache Fory Codecs for Redis Stampede Coordination

Issue: #597
Parent: #596
Research: #595
Status: Design approved in principle; implementation plan pending written-spec review
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
- Fory v1.3.0's `threadsafe.Fory` pools independent runtimes. Calling its
  registration proxy is insufficient because it registers only the acquired
  runtime. The provider must register every runtime in the pool factory.

## Package And API

Create `cache/rediscoord/fory` with package name `rediscoordfory`. Keeping the
provider in a child package prevents existing `rediscoord` consumers from
importing Apache Fory unless they select this codec explicitly.

```go
type Registration func(*fory.Fory) error

type Options struct {
    Register        Registration
    MaxPayloadBytes int
    MaxDepth        int
}

type Codec[V any] struct { /* immutable after construction */ }

func NewNativeFast[V any](options Options) (*Codec[V], error)
func NewNativeCompatible[V any](options Options) (*Codec[V], error)

func (c *Codec[V]) Marshal(value V) ([]byte, error)
func (c *Codec[V]) Unmarshal(payload []byte) (V, error)
```

`Options.Register` is mandatory and must be deterministic: it registers the
root and all declared Fory struct/enum/extension types. The constructor applies
it once to validate errors, then `threadsafe.NewWithFactory` creates every
pooled native Fory instance with the identical options and registration. A
factory registration failure after validation is an invariant violation and
must not be silently ignored.

The provider has no mutation API. Registration happens before the codec becomes
available to callers, so concurrent marshal/unmarshal operations cannot race
with type registration.

## Wire Contract

The inner codec writes a small binary envelope before Fory bytes:

| Field | Size | Purpose |
| --- | --- | --- |
| Magic | 4 bytes: `BTFY` | Reject JSON, raw Fory, and unrelated values before Fory decode. |
| Codec version | 1 byte: `1` | Evolves this wrapper independently from Fory. |
| Profile | 1 byte | `1` native-fast; `2` native-compatible. |
| Payload length | 4-byte unsigned big-endian | Allows exact truncation/trailing-byte and maximum-size validation. |
| Fory payload | declared length | Bytes returned by the thread-safe Fory runtime. |

`NewNativeFast` uses exactly `WithXlang(false)`, `WithCompatible(false)`,
`WithTrackRef(false)`, and caller-selected bounded `WithMaxDepth`. It is for
fixed-schema volatile cache values; callers rotate the `rediscoord` namespace
when changing the struct/schema generation.

`NewNativeCompatible` uses exactly `WithXlang(false)`, `WithCompatible(true)`,
the same explicit reference policy, and caller-selected bounded `WithMaxDepth`.
It supports Fory-compatible field evolution but does not make semantic changes
or incompatible type changes safe. Callers still rotate the namespace for
those changes.

`MaxPayloadBytes` applies before Fory decode and excludes the wrapper header.
The codec rejects non-positive configuration, an input larger than header plus
the configured maximum, unknown magic/version/profile, mismatched profile,
declared-length mismatch, truncation, and trailing bytes.

## Error Contract

Introduce a provider-specific typed error with operation (`marshal` or
`unmarshal`) and profile metadata. Its formatted message must not include
payload bytes, caller Redis key, owner token, or the supplied registration
function. It may unwrap the Fory cause so callers can classify it with
`errors.Is`/`errors.As`; Redis operation error wrapping remains owned by
`cache/rediscoord` and `btredis.OpError`.

## Data Flow

1. The load owner calls `Codec.Marshal(value)`.
2. The Fory codec serializes a value through its thread-safe pooled runtime,
   receiving an owned byte slice.
3. The codec adds the `BTFY` inner envelope.
4. Existing `rediscoord.encodeResult` wraps those bytes in its current JSON
   owner-token envelope and writes the short-TTL Redis result key.
5. A waiter reads the existing result envelope, validates the Fory envelope,
   decodes the payload, then fills its local cache as before.

The outer JSON/base64 envelope is deliberately untouched in this issue. It is
measured separately by #599 and is not a direct Redis value-cache format.

## Tests

Use test-driven development in `cache/rediscoord/fory`:

- A native-fast registered Go struct round-trips and satisfies
  `rediscoord.Codec[V]`.
- Native-compatible registered structs round-trip and demonstrate a compatible
  added-field fixture.
- Fast accepts neither compatible wrapper bytes nor altered profile/version
  bytes; both profiles reject JSON/raw Fory bytes and wrong declared lengths.
- Malformed/truncated/oversized input returns the typed provider error and its
  formatted text contains no payload marker.
- Invalid options and failed registration return constructor errors before the
  codec is usable.
- A high-contention concurrent marshal/unmarshal test passes under `-race` and
  verifies each decoded value. No Testcontainers test is needed for the codec
  itself because existing `rediscoord` integration tests already own the Redis
  lock/result protocol and this change does not alter it.
- Existing `cache/rediscoord` tests remain unchanged and pass.

## Documentation And Benchmark Scope

Update `cache/rediscoord/README.md` and `README.ko.md` with constructor
examples, `TRUSTED_INTERNAL`, native-only mode, fast versus compatible limits,
registration lifecycle, and namespace rotation. State that Fory changes only
the inner coordination payload and does not make the outer result envelope
binary.

No benchmark results are added in #597. #599 owns a same-condition benchmark
and must publish the raw command/environment, result table, Chart, and written
analysis before any throughput or storage claim.

## Alternatives Rejected

- Adding Fory directly to `cache/rediscoord`: this would make Apache Fory part
  of the normal package import surface and weakens opt-in dependency ownership.
- Using `threadsafe.New(...).RegisterStructByName(...)`: registration reaches
  only one pooled runtime and does not establish an immutable pool-wide type
  registry.
- Writing raw Fory bytes with no wrapper: the caller could not distinguish
  native-fast, native-compatible, stale JSON, or malformed cache content before
  entering Fory.
- Treating this as a direct Redis L2 value cache: `rediscoord` is a short-TTL
  stampede-coordination protocol; #598 owns the separate direct-value boundary.
