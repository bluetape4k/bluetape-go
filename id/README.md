# id

[English](README.md) | [한국어](README.ko.md)

`id` provides Go-native generators for service identifiers: UUID v4, UUID v7,
random ULID, monotonic ULID, standard seconds-precision KSUID, and Snowflake
int64 IDs.

## Import

```go
import "github.com/bluetape4k/bluetape-go/id"
```

## Selection Guide

| Need | Use | Notes |
|---|---|---|
| Request or correlation ID | UUID v4 or UUID v7 | UUID v4 is random. UUID v7 includes time ordering. |
| Database primary key with UUID storage | UUID v7 | Sorts better than UUID v4 when wall-clock behavior is acceptable. |
| Monotonic string ID | ULID | Canonical 26-character Crockford Base32 string. |
| Distributed numeric entity ID | Snowflake | 63-bit non-negative int64 with timestamp, machine ID, and sequence fields. |
| URL-safe string ID | ULID or KSUID | ULID is 26-character Crockford Base32. KSUID is 27-character Base62 with a seconds timestamp. |
| Log/event ID with second-level time sorting | KSUID | Canonical Segment-compatible 27-character string. |
| Deterministic or name-based UUID | Deferred | UUID v5/name-based helpers are not part of 0.6.0. |
| Millisecond KSUID compatibility | Deferred | Kotlin-compatible millis KSUID format is tracked separately in #171. |
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
```

## Behavior

- UUID v4 and UUID v7 generation use crypto-grade default entropy.
- UUID v7 and ULID ordering may degrade during wall-clock rollback;
  generation must not be treated as a clock-monotonicity guarantee.
- Random and monotonic ULID defaults use `crypto/rand`; this package does not
  use the `oklog/ulid` `math/rand` default entropy.
- KSUID generation uses crypto-grade default entropy and the standard Segment
  seconds-precision format. Lexical ordering follows the encoded timestamp, but
  same-second ordering is entropy-dependent and not monotonic.
- Custom KSUID entropy readers and clock functions must be safe for concurrent
  use when a generator instance is shared across goroutines.
- KSUID clocks must stay within the standard KSUID timestamp range, from the
  KSUID epoch through the maximum 32-bit seconds offset.
- Public APIs return strings or repo-owned values. Dependency UUID/ULID/KSUID
  concrete types are not part of the stable bluetape-go API.
- Concrete generator implementations are unexported. Callers do not depend on a
  zero-value concrete generator contract.
- Public parse helpers return canonical strings and wrap invalid input so callers
  can use `errors.Is(err, id.ErrInvalidID)`. UUID parsing accepts only canonical
  36-character lowercase UUID strings.
- KSUID parsing accepts canonical 27-character Segment-compatible strings and
  exposes timestamp extraction through `KSUIDTime`.
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
go test -run '^$' -bench . -benchmem ./id
```
