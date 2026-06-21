# id

[English](README.md) | [한국어](README.ko.md)

`id`는 서비스 식별자를 위한 Go-native generator를 제공합니다. UUID v4, UUID v7,
random ULID, monotonic ULID, standard seconds-precision KSUID,
Kotlin-compatible millisecond KSUID, Snowflake int64 ID를 한 패키지에서 선택할 수
있습니다.

## 가져오기

```go
import (
    "time"

    "github.com/bluetape4k/bluetape-go/id"
)
```

## 선택 가이드

| 필요 | 사용 | 메모 |
|---|---|---|
| request/correlation ID | UUID v4 또는 UUID v7 | UUID v4는 random ID이고 UUID v7은 time ordering을 포함합니다. |
| UUID 저장 DB primary key | UUID v7 | wall-clock 특성을 받아들일 수 있으면 UUID v4보다 정렬성이 좋습니다. |
| monotonic string ID | ULID | 표준 26자 Crockford Base32 문자열입니다. |
| 분산 numeric entity ID | Snowflake | timestamp, machine ID, sequence field를 가진 63-bit non-negative int64입니다. |
| URL-safe string ID | ULID 또는 KSUID | ULID는 26-character Crockford Base32이고 KSUID 계열은 27-character Base62 string입니다. |
| second-level time sorting이 필요한 log/event ID | KSUID | Segment-compatible canonical 27-character string입니다. |
| Kotlin-compatible millisecond KSUID | KSUID millis | `NewKSUIDMillisGenerator`, `ParseKSUIDMillis`, `KSUIDMillisTime`은 bluetape4k `Ksuid.Millis`의 8-byte millisecond timestamp + 12-byte payload format을 사용합니다. source-compatible이며 Segment-sortable format은 아닙니다. |
| deterministic/name-based UUID | Deferred | UUID v5/name-based helper는 0.6.0 범위가 아닙니다. |
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

## 동작

- UUID v4와 UUID v7은 기본적으로 crypto-grade entropy를 사용합니다. 기본
  entropy path는 per-ID randomness overhead를 줄이기 위해 package-local lock
  뒤에서 `crypto/rand` read를 batch 처리합니다. 생성된 ID는 identifier이며
  authentication 또는 authorization secret이 아닙니다.
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
- Random/monotonic ULID 기본값은 같은 buffered `crypto/rand` entropy path를
  사용합니다. 이 package는 `oklog/ulid`의 `math/rand` default entropy를
  사용하지 않습니다.
- KSUID generation은 buffered crypto-grade default entropy와 Segment 표준
  seconds-precision format을 사용합니다. Lexical ordering은 encoded timestamp를
  따르지만 same-second ordering은 entropy-dependent이며 monotonic하지 않습니다.
- KSUID millis generation은 Kotlin `Ksuid.Millis` compatibility format을
  사용합니다. 20 bytes는 `1400000000000` 기준 8-byte big-endian millisecond
  timestamp와 12-byte payload이고, bluetape4k bit-stream Base62 alphabet으로
  인코딩합니다.
- KSUID millis는 Segment KSUID가 아니며 lexicographic timestamp ordering을
  보장하지 않습니다. Compatibility alphabet은 `A-Z a-z 0-9`이고 Segment의
  sortable `0-9 A-Z a-z` encoding과 다릅니다.
- Segment KSUID seconds와 bluetape4k KSUID millis는 모두 bare 27-character
  Base62 string이라 string만으로 family를 식별할 수 없습니다. `KSUIDTime`은
  Segment seconds string에만, `KSUIDMillisTime`은 millis string에만 사용하세요.
  cross-family parsing은 성공할 수 있지만 다른 family 기준 timestamp를 반환합니다.
- Custom KSUID 및 KSUID millis entropy reader와 clock function을 shared
  generator에서 사용할 때는 caller가 concurrent use safety를 보장해야 합니다.
- KSUID clock은 KSUID epoch부터 maximum 32-bit seconds offset까지의 표준 KSUID
  timestamp range 안에 있어야 합니다.
- Public API는 string 또는 repo-owned value를 반환합니다. Dependency
  UUID/ULID/KSUID concrete type은 stable bluetape-go API가 아닙니다.
- Concrete generator 구현은 export하지 않습니다. Caller는 zero-value concrete
  generator contract에 의존하지 않습니다.
- Public parse helper는 invalid input을 `errors.Is(err, id.ErrInvalidID)`로
  확인할 수 있게 감쌉니다. UUID와 Segment KSUID helper는 canonical string을
  반환하고, KSUID millis는 Kotlin이 encoded bit stream을 27 characters로
  truncate하므로 supplied Kotlin-compatible string을 validate한 뒤 그대로
  반환합니다.
- KSUID parsing은 canonical 27-character Segment-compatible string만 허용하고
  `KSUIDTime`으로 timestamp를 추출합니다.
- KSUID millis parsing은 27-character Kotlin-compatible string을 허용하고
  `KSUIDMillisTime`으로 timestamp를 추출합니다.
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
make bench-id
```

## Benchmark Snapshot

Issue #168은 `bluetape4k-idgenerators`와의 local Go-vs-JVM 비교를 기록합니다.
durable report와 raw output은
[`docs/research/2026-06-10-issue-168-id-generator-benchmark.md`](../docs/research/2026-06-10-issue-168-id-generator-benchmark.md)에
있습니다.
이 section은 해당 pre-optimization snapshot과 chart를 의도적으로 보존합니다.
Issue #192 before/after optimization 결과는
[`docs/research/2026-06-11-issue-192-id-generator-performance.md`](../docs/research/2026-06-11-issue-192-id-generator-performance.md)에
별도로 기록했습니다.

Local Go command:

```bash
make bench-id
```

Environment: macOS arm64, Apple M4 Pro, Go 1.26.4.

![ID generator benchmark summary](../docs/images/readme-charts/id-generator-benchmark-summary.png)

Chart는 Kotlin `kotlinx-benchmark` throughput을 `1e9 / (ops/s * 100)`으로
`ns/id`로 환산해 Go와 Kotlin을 같은 축에서 읽게 합니다. 낮을수록 좋습니다.
Kotlin benchmark row에는 batch uniqueness check 비용이 포함됩니다.

결과 해석: Go Snowflake synthetic-clock row는 single-thread와 concurrent 모두에서
가장 낮은 `ns/id`를 보입니다. 다만 Go benchmark는 deterministic clock hook을
주입하므로 production-equivalent Go-vs-Kotlin Snowflake verdict로 읽으면 안
됩니다. 이번 local snapshot에서는 Go UUID row를 generator 재사용 기준으로 고쳐도
Kotlin row가 UUID v4/v7과 KSUID 계열에서 더 낮은 `ns/id`를 보입니다. ULID는
혼합 결과입니다. single-thread monotonic row는 Kotlin이 낮고, concurrent
comparison에서는 Go monotonic ULID가 낮습니다.

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkSnowflakeNextInt64-12` | 11.98 | 0 | 0 |
| `BenchmarkSnowflakeNextInt64SameMillisecond-12` | 12.23 | 0 | 0 |
| `BenchmarkULIDMonotonic-12` | 65.45 | 48 | 2 |
| `BenchmarkSnowflakeNextInt64Parallel-12` | 92.37 | 0 | 0 |
| `BenchmarkULIDRandom-12` | 104.5 | 48 | 2 |
| `BenchmarkULIDMonotonicParallel-12` | 190.5 | 48 | 2 |
| `BenchmarkUUIDV4ReuseGenerator-12` | 225.8 | 64 | 2 |
| `BenchmarkUUIDV4NewString-12` | 230.1 | 112 | 3 |
| `BenchmarkUUIDV7ReuseGenerator-12` | 251.2 | 64 | 2 |
| `BenchmarkUUIDV7NewString-12` | 271.7 | 112 | 3 |
| `BenchmarkULIDRandomParallel-12` | 280.9 | 48 | 2 |
| `BenchmarkKSUIDMillisNextString-12` | 320.6 | 104 | 3 |
| `BenchmarkUUIDV7ReuseGeneratorParallel-12` | 376.4 | 64 | 2 |
| `BenchmarkKSUIDNextString-12` | 389.7 | 48 | 2 |
| `BenchmarkUUIDV4NewStringParallel-12` | 549.2 | 112 | 3 |
| `BenchmarkUUIDV4ReuseGeneratorParallel-12` | 557.5 | 64 | 2 |
| `BenchmarkUUIDV7NewStringParallel-12` | 569.6 | 112 | 3 |
| `BenchmarkKSUIDMillisNextStringParallel-12` | 632.3 | 104 | 3 |
| `BenchmarkKSUIDNextStringParallel-12` | 656.3 | 48 | 2 |

### Post-Optimization Comparison

Issue #192는 Issue #168 chart를 pre-optimization baseline으로 보존하고,
optimized Go UUID, ULID, KSUID row를 사용한 Kotlin-vs-Go comparison을
추가합니다. Snowflake는 Issue #192의 변경 대상이 아니므로 Issue #168의 Go
synthetic-clock measurement를 그대로 사용합니다.

![Kotlin vs Go optimized ID generator comparison](../docs/images/readme-charts/id-generator-kotlin-go-optimized-comparison.png)

Go entropy buffering 이후 local snapshot의 해석은 꽤 바뀝니다. Go는 UUID v4,
KSUID millis, concurrent ULID/KSUID row에서 앞섭니다. Kotlin은 UUID v7과
single-thread monotonic ULID row에서 여전히 낮은 `ns/id`를 보입니다. entropy
overhead가 줄어든 뒤에도 UUID v7 ordering/lock cost가 남아 있으므로 UUID v7이
가장 명확한 Go follow-up optimization target입니다.

Interpretation boundary: Go row는 per-ID generation을 직접 측정합니다. Chart의
Kotlin row는 `kotlinx-benchmark` batch throughput을 환산한 값이므로 local
snapshot으로는 같은 축에서 비교할 수 있지만, Kotlin benchmark의 batch uniqueness
check 비용은 포함되어 있습니다. UUID chart row는 Go generator 재사용 benchmark를
사용하며, `NewString` row는 convenience function overhead를 보여주는 용도라
cross-runtime chart에는 사용하지 않습니다. Snowflake chart row는 synthetic Go
clock hook을 사용하므로 production-equivalent Go-vs-Kotlin Snowflake claim을
하려면 equivalent clock/batch condition에서 다시 측정해야 합니다.
