# id

[English](README.md) | [한국어](README.ko.md)

`id`는 서비스 식별자를 위한 Go-native generator를 제공합니다. 범위는 UUID v4,
UUID v7, random ULID, monotonic ULID, standard seconds-precision KSUID,
Snowflake int64 ID입니다.

## Import

```go
import (
    "time"

    "github.com/bluetape4k/bluetape-go/id"
)
```

## 선택 가이드

| 필요 | 사용 | 메모 |
|---|---|---|
| request/correlation ID | UUID v4 또는 UUID v7 | UUID v4는 random이고 UUID v7은 time ordering을 포함합니다. |
| UUID 저장 DB primary key | UUID v7 | clock 동작을 받아들일 수 있으면 UUID v4보다 정렬성이 좋습니다. |
| monotonic string ID | ULID | 표준 26자 Crockford Base32 문자열입니다. |
| 분산 numeric entity ID | Snowflake | timestamp, machine ID, sequence field를 가진 63-bit non-negative int64입니다. |
| URL-safe string ID | ULID 또는 KSUID | ULID는 26-character Crockford Base32이고 KSUID는 seconds timestamp를 가진 27-character Base62입니다. |
| second-level time sorting이 필요한 log/event ID | KSUID | Segment-compatible canonical 27-character string입니다. |
| deterministic/name-based UUID | Deferred | UUID v5/name-based helper는 0.6.0 범위가 아닙니다. |
| millisecond KSUID compatibility | Deferred | Kotlin-compatible millis KSUID format은 #171에서 별도 추적합니다. |
| future compact UUID string | Base62 deferred | 명시적인 ID rendering API 범위가 정해지기 전까지는 `codec/base62`를 직접 사용합니다. |
| future 128-bit sortable byte/string ID | Flake deferred | 후속 source-parity 후보입니다. |
| future short obfuscation | Hashids deferred | obfuscation은 security가 아닙니다. |

## 사용법

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

## 동작

- UUID v4와 UUID v7은 기본적으로 crypto-grade entropy를 사용합니다.
- UUID v7은 기본적으로 current Unix Epoch millisecond timestamp를 인코딩합니다.
  `WithUUIDTime`은 UUID v7 generator에 deterministic clock을 주입할 때 사용하고
  UUID v4 generator에서는 무시됩니다.
- UUID clock option은 production default를 바꾸지 않고 deterministic boundary 및
  ordering test를 작성할 수 있게 public API로 제공합니다.
- UUID v7 same-tick 및 rollback 상황에서는 generator별 logical tick을
  전진시켜 같은 shared generator에서 나온 값의 lexical order를 유지합니다. 이
  ordering은 별도 generator instance나 process 사이를 조정하지 않습니다. supplied
  millisecond의 12-bit logical tick이 소진되면 order 보존을 위해 encoded
  timestamp가 logical하게 전진할 수 있고, maximum UUID v7 timestamp에서 overflow가
  나면 invalid time option error를 반환합니다.
- Custom UUID entropy reader와 clock function을 shared generator에서 사용할
  때는 caller가 concurrent use safety를 보장해야 합니다.
- Random/monotonic ULID 기본값은 `crypto/rand`를 사용합니다. 이 package는
  `oklog/ulid`의 `math/rand` default entropy를 사용하지 않습니다.
- KSUID generation은 crypto-grade default entropy와 Segment 표준
  seconds-precision format을 사용합니다. Lexical ordering은 encoded timestamp를
  따르지만 same-second ordering은 entropy-dependent이며 monotonic하지 않습니다.
- Custom KSUID entropy reader와 clock function을 shared generator에서 사용할
  때는 caller가 concurrent use safety를 보장해야 합니다.
- KSUID clock은 KSUID epoch부터 maximum 32-bit seconds offset까지의 표준 KSUID
  timestamp range 안에 있어야 합니다.
- Public API는 string 또는 repo-owned value를 반환합니다. Dependency
  UUID/ULID/KSUID concrete type은 stable bluetape-go API가 아닙니다.
- Concrete generator 구현은 export하지 않습니다. Caller는 zero-value concrete
  generator contract에 의존하지 않습니다.
- Public parse helper는 canonical string을 반환하고 invalid input은
  `errors.Is(err, id.ErrInvalidID)`로 확인할 수 있게 감쌉니다. UUID parsing은
  canonical 36-character lowercase UUID string만 허용합니다.
- KSUID parsing은 canonical 27-character Segment-compatible string만 허용하고
  `KSUIDTime`으로 timestamp를 추출합니다.
- 생성된 ID는 identifier입니다. authentication token, authorization secret,
  standalone security boundary가 아닙니다.

## Snowflake 운영 계약

Snowflake ID는 approximate creation time, 10-bit machine ID, 12-bit
per-millisecond sequence를 인코딩합니다. live generator/process/deployment마다
unique machine ID가 필요합니다. 이 package는 machine ID
allocator를 제공하지 않고 MAC address, environment identity, hostname, random
process-local machine ID를 자동 탐지하지 않습니다. 동시 process나 같은
millisecond restart에서 duplicate machine ID를 재사용하면 duplicate ID가 생길 수
있습니다.

유효한 machine ID 범위는 `0..1023`입니다.

## Test

```bash
go test -count=1 ./id
go test -race -count=1 ./id
go test -run '^$' -bench . -benchmem ./id
```
