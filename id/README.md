# id

[English](README.md) | [한국어](README.ko.md)

`id` provides Go-native generators for service identifiers: UUID v4, UUID v7,
random ULID, monotonic ULID, standard seconds-precision KSUID,
Kotlin-compatible millisecond KSUID, and Snowflake int64 IDs.

## Import

```go
import (
    "time"

    "github.com/bluetape4k/bluetape-go/id"
)
```

## Selection Guide

| Need | Use | Notes |
|---|---|---|
| Request or correlation ID | UUID v4 or UUID v7 | UUID v4 is random. UUID v7 includes time ordering. |
| Database primary key with UUID storage | UUID v7 | Sorts better than UUID v4 when wall-clock behavior is acceptable. |
| Monotonic string ID | ULID | Canonical 26-character Crockford Base32 string. |
| Distributed numeric entity ID | Snowflake | 63-bit non-negative int64 with timestamp, machine ID, and sequence fields. |
| URL-safe string ID | ULID or KSUID | ULID is 26-character Crockford Base32. KSUID variants are 27-character Base62 strings. |
| Log/event ID with second-level time sorting | KSUID | Canonical Segment-compatible 27-character string. |
| Kotlin-compatible millisecond KSUID | KSUID millis | `NewKSUIDMillisGenerator`, `ParseKSUIDMillis`, and `KSUIDMillisTime` use the bluetape4k `Ksuid.Millis` 8-byte millisecond timestamp plus 12-byte payload format. It is source-compatible, not Segment-sortable. |
| Deterministic or name-based UUID | Deferred | UUID v5/name-based helpers are not part of 0.6.0. |
| Future compact UUID string | Base62 deferred | Use `codec/base62` directly until an explicit ID rendering API is scoped. |
| Future 128-bit sortable byte/string ID | Flake deferred | Follow-up source-parity candidate. |
| Future short obfuscation | Hashids deferred | Obfuscation is not security. |

## Usage

```go
requestID, err := id.NewUUIDV7()
if err != nil {
    return err
}

monotonic, err := id.NewMonotonicULIDGenerator()
if err != nil {
    return err
}
messageID, err := monotonic.NextString()
if err != nil {
    return err
}

snowflake, err := id.NewSnowflakeGenerator(7)
if err != nil {
    return err
}
entityID, err := snowflake.NextInt64()
if err != nil {
    return err
}

ksuidGenerator, err := id.NewKSUIDGenerator()
if err != nil {
    return err
}
eventID, err := ksuidGenerator.NextString()
if err != nil {
    return err
}
eventTime, err := id.KSUIDTime(eventID)
if err != nil {
    return err
}

ksuidMillis, err := id.NewKSUIDMillisGenerator()
if err != nil {
    return err
}
millisID, err := ksuidMillis.NextString()
if err != nil {
    return err
}
millisTime, err := id.KSUIDMillisTime(millisID)
if err != nil {
    return err
}

fixed := time.Date(2026, 6, 8, 1, 2, 3, 0, time.UTC)
deterministicUUIDs, err := id.NewUUIDV7Generator(
    id.WithUUIDTime(func() time.Time { return fixed }),
)
if err != nil {
    return err
}
testID, err := deterministicUUIDs.NextString()
if err != nil {
    return err
}
```

## Behavior

- UUID v4 and UUID v7 generation use crypto-grade default entropy.
- UUID v7 encodes the current Unix Epoch millisecond timestamp by default.
  `WithUUIDTime` may inject a deterministic clock for UUID v7 generators and is
  ignored by UUID v4 generators.
- The UUID clock option is public so callers can write deterministic boundary
  and ordering tests without changing production defaults.
- UUID v7 same-tick and rollback generation advances a per-generator logical
  tick so values from the same shared generator keep lexical order. This does
  not coordinate ordering across separate generator instances or processes.
  When a supplied millisecond exhausts its available 12-bit logical ticks, the
  encoded timestamp may advance logically to preserve order; at the maximum
  UUID v7 timestamp, overflow returns an invalid time option error.
- Custom UUID entropy readers and clock functions must be safe for concurrent
  use when a generator instance is shared across goroutines.
- Random and monotonic ULID defaults use `crypto/rand`; this package does not
  use the `oklog/ulid` `math/rand` default entropy.
- KSUID generation uses crypto-grade default entropy and the standard Segment
  seconds-precision format. Lexical ordering follows the encoded timestamp, but
  same-second ordering is entropy-dependent and not monotonic.
- KSUID millis generation uses the Kotlin `Ksuid.Millis` compatibility format:
  20 bytes as 8-byte big-endian milliseconds since `1400000000000` plus a
  12-byte payload, encoded with bluetape4k's bit-stream Base62 alphabet.
- KSUID millis is not a Segment KSUID and does not claim lexicographic timestamp
  ordering. The compatibility alphabet is `A-Z a-z 0-9`, not Segment's
  sortable `0-9 A-Z a-z` encoding.
- Segment KSUID seconds and bluetape4k KSUID millis are both bare 27-character
  Base62 strings, so the string alone does not identify the family. Use
  `KSUIDTime` only for Segment seconds strings and `KSUIDMillisTime` only for
  millis strings; cross-family parsing may succeed but returns the other
  family's timestamp interpretation.
- Custom KSUID and KSUID millis entropy readers and clock functions must be safe
  for concurrent use when a generator instance is shared across goroutines.
- KSUID clocks must stay within the standard KSUID timestamp range, from the
  KSUID epoch through the maximum 32-bit seconds offset.
- Public APIs return strings or repo-owned values. Dependency UUID/ULID/KSUID
  concrete types are not part of the stable bluetape-go API.
- Concrete generator implementations are unexported. Callers do not depend on a
  zero-value concrete generator contract.
- Public parse helpers wrap invalid input so callers can use
  `errors.Is(err, id.ErrInvalidID)`. UUID and Segment KSUID helpers return
  canonical strings; KSUID millis validates and returns the supplied
  Kotlin-compatible string because Kotlin truncates the encoded bit stream to 27
  characters.
- KSUID parsing accepts canonical 27-character Segment-compatible strings and
  exposes timestamp extraction through `KSUIDTime`.
- KSUID millis parsing accepts 27-character Kotlin-compatible strings and
  exposes timestamp extraction through `KSUIDMillisTime`.
- Generated IDs are identifiers, not authentication tokens, authorization
  secrets, or a standalone security boundary.

## Snowflake Operator Contract

Snowflake IDs encode approximate creation time, a 10-bit machine ID, and a
12-bit per-millisecond sequence. A unique machine ID is required per live
generator/process/deployment. This package does not provide a machine ID
allocator and does not auto-discover MAC addresses, environment identity, host
names, or random process-local machine IDs. Reusing a duplicate machine ID across
concurrent processes or same-millisecond restarts can produce duplicate IDs.

Valid machine IDs are `0..1023`.

## Test

```bash
go test -count=1 ./id
go test -race -count=1 ./id
make bench-id
```

## Benchmark Snapshot

Issue #168 records a local Go-vs-JVM comparison against
`bluetape4k-idgenerators`. The durable report and raw outputs are in
[`docs/research/2026-06-10-issue-168-id-generator-benchmark.md`](../docs/research/2026-06-10-issue-168-id-generator-benchmark.md).

Local Go command:

```bash
make bench-id
```

Environment: macOS arm64, Apple M4 Pro, Go 1.26.4.

![ID generator benchmark summary](../docs/images/readme-charts/id-generator-benchmark-summary.png)

The chart normalizes Kotlin `kotlinx-benchmark` throughput to `ns/id` with
`1e9 / (ops/s * 100)`, so Go and Kotlin can be read on the same axis. Lower is
better. Kotlin benchmark rows include batch uniqueness checks.

Result reading: Go Snowflake is the clear hot-path winner in both single-thread
and concurrent runs. Kotlin is faster for UUID v4/v7 and KSUID families in this
snapshot. ULID is mixed: Kotlin is faster in the single-thread monotonic row,
while Go monotonic ULID is faster under the concurrent comparison.

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkSnowflakeNextInt64-12` | 12.13 | 0 | 0 |
| `BenchmarkSnowflakeNextInt64SameMillisecond-12` | 12.46 | 0 | 0 |
| `BenchmarkULIDMonotonic-12` | 67.77 | 48 | 2 |
| `BenchmarkSnowflakeNextInt64Parallel-12` | 85.74 | 0 | 0 |
| `BenchmarkULIDRandom-12` | 108.4 | 48 | 2 |
| `BenchmarkULIDMonotonicParallel-12` | 191.4 | 48 | 2 |
| `BenchmarkUUIDV4-12` | 241.1 | 112 | 3 |
| `BenchmarkUUIDV7-12` | 270.9 | 112 | 3 |
| `BenchmarkULIDRandomParallel-12` | 302.2 | 48 | 2 |
| `BenchmarkKSUIDMillisNextString-12` | 342.5 | 104 | 3 |
| `BenchmarkKSUIDNextString-12` | 393.1 | 48 | 2 |
| `BenchmarkUUIDV4Parallel-12` | 576.8 | 112 | 3 |
| `BenchmarkUUIDV7Parallel-12` | 580.4 | 112 | 3 |
| `BenchmarkKSUIDMillisNextStringParallel-12` | 644.2 | 104 | 3 |
| `BenchmarkKSUIDNextStringParallel-12` | 664.1 | 48 | 2 |

Interpretation boundary: Go rows measure per-ID generation directly. Kotlin
rows in the chart are normalized from `kotlinx-benchmark` batch throughput, so
they are comparable as a local snapshot but still include Kotlin benchmark
batch uniqueness-check work.
