# Issue #36 Probabilistic Bloom Filter Spec

Issue: #36
Title: Port probabilistic data structures
Date: 2026-06-09
Milestone: 0.6.0
Work type: Type A full feature
Target package: `probabilistic`

## Goal

Add a first-party Go Bloom filter package that matches the useful current
surface of `bluetape4k-projects/utils/probabilistic` while staying idiomatic to
Go. The `0.6.0` slice is dependency-free, in-memory, deterministic, and safe to
use from ordinary goroutine-heavy service code.

## Source Evidence

- `bluetape4k-projects/utils/probabilistic/src/main/kotlin/.../BloomFilter.kt`
  defines the public Bloom filter contract: `mightContain`, `put`,
  `approximateElementCount`, `expectedFpp`, `clear`, configuration metadata,
  and `bitCount`.
- `BloomFilterConfig.kt` validates `expectedInsertions > 0`,
  `falsePositiveProbability in (0, 1)`, calculates `bitSize` and
  `hashFunctionCount`, and rejects unsupported bitset sizes.
- `InMemoryBloomFilter.kt` uses a bitset plus SHA-256 double hashing and merges
  compatible filters with bitwise OR.
- Kotlin docs explicitly state false positives are possible, false negatives
  should not occur for successfully inserted elements, deletion is unsupported,
  and the Kotlin implementation does not guarantee thread-safe writes.
- Redis-backed Bloom, Cuckoo, and HyperLogLog implementations live under
  `bluetape4k-projects/infra/lettuce`. They are tracked separately by #182 and
  are not part of the #36 closure.

## API Contract

Package: `probabilistic`

Public types and functions:

- `type BloomFilter[T any] interface`
- `type Config struct`
- `type Hasher[T any] struct`
- `func NewHasher[T any](key string, sum func(T) []byte) (Hasher[T], error)`
- `func NewBloomFilter[T any](cfg Config, hasher Hasher[T]) (BloomFilter[T], error)`
- `func NewStringBloomFilter(cfg Config) (BloomFilter[string], error)`
- `func NewBytesBloomFilter(cfg Config) (BloomFilter[[]byte], error)`
- `func DefaultConfig() Config`
- `func NewConfig(expectedInsertions uint64, falsePositiveProbability float64) (Config, error)`

Methods:

- `ExpectedInsertions() uint64`
- `FalsePositiveProbability() float64`
- `BitSize() uint64`
- `HashFunctionCount() uint64`
- `BitCount() uint64`
- `IsEmpty() bool`
- `MightContain(value T) bool`
- `Put(value T) bool`
- `PutAll(other BloomFilter[T]) error`
- `ApproximateElementCount() uint64`
- `ExpectedFPP() float64`
- `Clear()`

The exported API intentionally avoids a broad Kotlin DSL shape. The concrete
implementation remains unexported so callers cannot accidentally construct a
zero-value filter with missing config/hasher state. Constructor failures are
therefore explicit instead of silently turning `Put` into a no-op.

## Behavior

- `MightContain` returns `false` only when the value is definitely absent.
- `MightContain` returning `true` means the value may exist; false positives are
  expected by design.
- `Put` returns `true` when at least one bit changed. `false` means all target
  bits were already set, not that the value definitely existed.
- `PutAll` OR-merges only compatible filters: same expected insertions, target
  FPP, calculated bit size, hash count, and hasher identity.
- `Clear` removes all bits and resets `BitCount`.
- Deletion is not supported; Redis-backed or deletion-capable variants are
  deferred to #182.
- The Go implementation is goroutine-safe for concurrent `MightContain`, `Put`,
  `Clear`, and metadata reads. `PutAll` is also goroutine-safe: it snapshots the
  source filter under the source read lock, then applies the OR merge under the
  target write lock. Self-merge is a no-op. This differs from the Kotlin caveat
  deliberately because Go service code commonly shares values across goroutines.

## Hashing

- Default string and byte-slice constructors use stable byte representations.
- Generic construction requires an explicit `Hasher[T]`, avoiding reflection,
  `encoding/gob`, or `fmt.Stringer` surprises in the core API.
- A hasher includes a caller-supplied comparable compatibility key. `PutAll`
  accepts custom-hasher filters only when the keys match. This replaces the
  Kotlin `hasher == other.hasher` check because Go function values cannot be
  compared for identity.
- `BloomFilter[T]` is a sealed interface: callers use package-created filters,
  while external implementations are intentionally unsupported. `PutAll`
  therefore accepts only filters created by this package.
- Index calculation uses SHA-256 and Kirsch-Mitzenmacher double hashing:
  `hash1 + i * hash2`, floor-modulo `bitSize`.
- `hash2 == 0` is replaced with a fixed odd constant to avoid repeated offsets.
- Hasher compatibility key is recorded at construction so `PutAll` can reject
  filters that cannot be proven compatible.
- Source parity note: Kotlin's default hasher accepts `String`, `Int`, `Long`,
  `ByteArray`, `Serializable`, and a `toString()` fallback. Go intentionally
  provides only string/byte-slice convenience constructors plus explicit generic
  hashers so callers own stable binary representation decisions.

## Config Math

- Default `expectedInsertions`: `1_000_000`
- Default `falsePositiveProbability`: `0.03`
- `bitSize = ceil(-n * ln(p) / ln(2)^2)`
- `hashFunctionCount = max(1, round((bitSize / n) * ln(2)))`
- Reject configurations that require more than `math.MaxInt` machine words or
  overflow intermediate integer calculations.
- `ExpectedFPP` uses the current fill ratio raised to the hash count.
- `ApproximateElementCount` follows the standard Bloom estimate and returns
  `math.MaxUint64` when all bits are set.

## Errors

Sentinel errors:

- `ErrInvalidConfig`
- `ErrIncompatibleFilter`
- `ErrNilFilter`
- `ErrNilHasher`
- `ErrEmptyHasherKey`

Error wrapping must support `errors.Is`.

## Non-Goals

- No Redis, Cuckoo, HyperLogLog, Redisson, Lettuce, or RedisBloom module support
  in #36. Those are #182.
- No deletion, counting Bloom filter, scalable Bloom filter, or count-min
  sketch.
- No serialization in #36. Serialization would require a stable wire contract
  and compatibility story; add it only through a follow-up issue.
- No external dependency and no probabilistic test that can flake under normal
  CI variance.
- No context-aware API. The in-memory operations are short CPU/memory work and
  do not own goroutines, I/O, timers, or cancellation boundaries.

## Test Requirements

- Config validation:
  - invalid expected insertions;
  - invalid FPP values;
  - oversized unsupported bitset;
  - deterministic bit size/hash count sanity range.
- Behavior:
  - inserted values have no false negatives;
  - `Put` return value is bit-change based;
  - `Clear` resets bits and count;
  - `ApproximateElementCount` and `ExpectedFPP` are monotonic enough for
    bounded assertions;
  - compatible merge succeeds, including custom hashers with matching keys;
  - incompatible merge rejects with `ErrIncompatibleFilter`;
  - nil filter and nil/empty hasher errors are sentinel-compatible.
- False-positive check:
  - use a deterministic inserted/missing corpus and a loose upper bound such as
    `3x` target FPP to avoid flaky tests.
- Concurrency:
  - use `GoroutineStressTester` with at least `max(32, GOMAXPROCS*4)` workers
    and hundreds of rounds;
  - concurrent put/read/metadata paths must pass under normal `go test` and
    `go test -race`;
  - concurrent `PutAll`, reciprocal merge, self-merge, and `Clear` interaction
    must be covered or the contract must be narrowed before implementation.
- Async/cancellation:
  - `AsyncJobTester` is N/A for #36 because the package has no context-aware
    goroutine, I/O, timer, or cancellation boundary. Record the N/A rationale in
    a review note.
- Examples:
  - constructor;
  - put/might-contain semantics;
  - merge;
  - FPP/approximate count introspection.

## Documentation Requirements

- Add `probabilistic/README.md` and `probabilistic/README.ko.md`.
- Update root `README.md` and `README.ko.md` package tables and portable utility
  links.
- Update `CHANGELOG.md` under `[Unreleased]`.
- Update `WIP.md` to mark #35 delivered and #36 as the final 0.6.0 closure
  item, and record #182 as a 0.6.1 follow-up.
- README must explicitly state:
  - false positives are possible;
  - successful inserts must not produce false negatives;
  - deletion is unsupported;
  - the Go implementation is goroutine-safe;
  - Redis-backed Bloom/Cuckoo/HyperLogLog support is deferred to #182.

## Acceptance Criteria

- `probabilistic` package provides the Bloom filter API above.
- Targeted tests pass:
  - `go test -count=1 ./probabilistic`
  - `go test -race -count=1 ./probabilistic`
- Full local gate passes with `make ci`.
- Step 6-R 7-Tier review records `P0=0 P1=0`.
- PR metadata matches issue #36: assignee `debop`, milestone `0.6.0`, labels
  `type: task`, `priority: p1`, `area: utilities`.
- GitHub CI passes before merge.
