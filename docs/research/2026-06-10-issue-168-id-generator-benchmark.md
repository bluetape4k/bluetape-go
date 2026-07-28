# Issue #168 ID Generator Benchmark 비교

Issue: #168
Milestone: 0.6.1
Date: 2026-06-10
Work type: Research / benchmark comparison

## 연구 질문

`bluetape-go/id`는 runtime-specific benchmark 결과를 API design decision과 혼동하지
않으면서, UUID, ULID, KSUID, KSUID millis, Snowflake generator를 sibling JVM
`bluetape4k-idgenerators` library와 어떻게 비교해야 하는가?

## 현재 결정

이 비교는 cross-runtime ranking이 아니라 재현 가능한 local snapshot으로 유지한다. Go는
per-ID cost를 package benchmark로 측정하고, JVM `bluetape4k-idgenerators`는 기존
`kotlinx-benchmark` benchmark suite로 batch throughput과 uniqueness check를 측정한다.
이 snapshot에서 드러난 두 gap은 follow-up optimization issue로 추적한다. Kotlin
Snowflake는 `bluetape4k-projects#738`, Go UUID/ULID/KSUID는 `bluetape-go#192`다.

## 비교 가능 Surface

| Concern | `bluetape-go/id` | `bluetape4k-idgenerators` | Comparison boundary |
|---|---|---|---|
| UUID v4 | `NewUUIDV4Generator`, `NewUUIDV4` | `Uuid.V4`, `UuidGenerator(Uuid.V4)` | Both produce random UUID v4. Go exposes reader injection; JVM exposes `Random` customization. |
| UUID v7 | `NewUUIDV7Generator`, `NewUUIDV7` | `Uuid.V7`, `UuidGenerator()` default | Both target time-ordered UUIDs. Go owns logical tick behavior and clock hooks; JVM uses the Java UUID Generator epoch strategy. |
| ULID random | `NewULIDGenerator`, `NewULID` | `ULID.randomULID()` / factory helpers | Go has an explicit random constructor; the JVM facade separates factory helpers from `UlidGenerator`. |
| ULID monotonic | `NewMonotonicULIDGenerator` | `UlidGenerator()`, `ULID.statefulMonotonic()` | Go splits random and monotonic constructors; JVM `UlidGenerator` defaults to stateful monotonic behavior. |
| KSUID seconds | `NewKSUIDGenerator`, `NewKSUID` | `Ksuid.Seconds`, `KsuidGenerator()` | Direct family match. |
| KSUID millis | `NewKSUIDMillisGenerator`, `NewKSUIDMillis` | `Ksuid.Millis`, `KsuidGenerator(Ksuid.Millis)` | Direct source-compatible family match; do not cross-parse or cross-compare with seconds KSUID. |
| Snowflake | `NewSnowflakeGenerator(machineID)` | `SnowflakeGenerator`, `Snowflakers.default/global` | Go requires caller-owned machine IDs; JVM includes default/global variants. |

## Commands

### Go

```bash
make bench-id
go test -count=1 ./id
go test -race -count=1 ./id
go test -run '^$' -bench '^Benchmark(UUIDV4NewString|UUIDV4NewStringParallel|UUIDV4ReuseGenerator|UUIDV4ReuseGeneratorParallel|UUIDV7NewString|UUIDV7NewStringParallel|UUIDV7ReuseGenerator|UUIDV7ReuseGeneratorParallel|ULIDRandom|ULIDRandomParallel|ULIDMonotonic|ULIDMonotonicParallel|KSUIDNextString|KSUIDNextStringParallel|KSUIDMillisNextString|KSUIDMillisNextStringParallel|SnowflakeNextInt64|SnowflakeNextInt64SameMillisecond|SnowflakeNextInt64Parallel)$' -benchmem ./id
```

### JVM

sibling project는 JMH JVM backend와 함께 `kotlinx-benchmark`를 사용한다.

```bash
cd /Users/debop/work/bluetape4k/bluetape4k-projects
./gradlew :bluetape4k-idgenerators:singleThreadBenchmark
./gradlew :bluetape4k-idgenerators:concurrentBenchmark
```

generated Gradle task list에는
`:bluetape4k-idgenerators:benchmarkSingleThreadBenchmark`,
`:bluetape4k-idgenerators:benchmarkConcurrentBenchmark` 같은 suffixed task name도
노출되지만, 현재 build에서는 위의 짧은 configuration task name이 유효하다.

## Environment

raw environment와 benchmark output은 `docs/research/outputs/issue-168/` 아래에
저장한다.

| File | Purpose |
|---|---|
| `environment.txt` | Host, OS, Go version, CPU, logical CPU count. |
| `java-version.txt` | JVM version used for the sibling benchmark. |
| `revisions.txt` | Local `bluetape-go` and sibling `bluetape4k-projects` branch/commit IDs. |
| `go-id-test.txt` | Go `id` package test output, including goroutine uniqueness stress coverage. |
| `go-id-race-test.txt` | Go `id` package race-detector output for the same package. |
| `go-id-bench-100ms.txt` | Short Go smoke benchmark. |
| `go-id-bench-1s.txt` | Main Go local snapshot. |
| `jvm-idgenerators-single-thread.txt` | JVM `singleThreadBenchmark` raw output. |
| `jvm-idgenerators-concurrent.txt` | JVM `concurrentBenchmark` raw output. |

관찰된 local environment:

- macOS arm64, Apple M4 Pro, 12 logical CPUs.
- Go 1.26.4.
- Oracle GraalVM Java 25.0.3.

## Chart 요약

아래 chart는 Go와 Kotlin의 같은 ID generator family를 단일 normalized unit인
`ns/id`로 비교한다. 낮을수록 좋다. Go row는 package benchmark `ns/op`를 그대로
사용한다. Kotlin row는 `batchSize=100` row를 사용해 `kotlinx-benchmark` throughput을
`1e9 / (ops/s * batchSize)`로 변환한다. chart는 single-thread와 concurrent 측정을
모두 보여 주므로 단순 generation cost와 shared-generator contention을 함께 다룬다.

![ID generator benchmark summary](../images/readme-charts/id-generator-benchmark-summary.png)

### Normalized Single-Thread Comparison

| Generator | Go ns/id | Kotlin ns/id | Kotlin source |
|---|---:|---:|---|
| Snowflake synthetic clock | 11.98 | 244.06 | `snowflake`, batch 100 |
| ULID monotonic | 65.45 | 37.01 | `ulid`, batch 100 |
| UUID v4 reused generator | 225.80 | 95.09 | `uuidV4`, batch 100 |
| UUID v7 reused generator | 251.20 | 23.59 | `uuidV7`, batch 100 |
| KSUID millis | 320.60 | 167.80 | `ksuidMillis`, batch 100 |
| KSUID seconds | 389.70 | 187.93 | `ksuidSeconds`, batch 100 |

### Normalized Concurrent Comparison

| Generator | Go ns/id | Kotlin ns/id | Kotlin source |
|---|---:|---:|---|
| Snowflake synthetic clock | 92.37 | 373.89 | `snowflake`, batch 100 |
| ULID monotonic | 190.50 | 408.56 | `ulid`, batch 100 |
| UUID v4 reused generator | 557.50 | 329.69 | `uuidV4`, batch 100 |
| UUID v7 reused generator | 376.40 | 115.42 | `uuidV7`, batch 100 |
| KSUID millis | 632.30 | 396.50 | `ksuidMillis`, batch 100 |
| KSUID seconds | 656.30 | 388.70 | `ksuidSeconds`, batch 100 |

## Go Snapshot

validation 및 stress/race evidence:

```bash
go test -count=1 ./id
go test -race -count=1 ./id
```

`./id` test package에는 UUID v4/v7, ULID random/monotonic, KSUID seconds/millis,
Snowflake에 대한 goroutine uniqueness stress coverage가 포함된다. 위 raw output은
benchmark artifact와 함께 보존한다.

Command:

```bash
go test -run '^$' -bench '^Benchmark(UUIDV4NewString|UUIDV4NewStringParallel|UUIDV4ReuseGenerator|UUIDV4ReuseGeneratorParallel|UUIDV7NewString|UUIDV7NewStringParallel|UUIDV7ReuseGenerator|UUIDV7ReuseGeneratorParallel|ULIDRandom|ULIDRandomParallel|ULIDMonotonic|ULIDMonotonicParallel|KSUIDNextString|KSUIDNextStringParallel|KSUIDMillisNextString|KSUIDMillisNextStringParallel|SnowflakeNextInt64|SnowflakeNextInt64SameMillisecond|SnowflakeNextInt64Parallel)$' -benchmem ./id
```

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

## JVM Snapshot

JVM benchmark는 batch throughput을 측정한다. 각 benchmark operation은 `batchSize`개의
ID를 생성하고 batch 내부 또는 benchmark trial 전체에서 uniqueness를 확인한다. 아래
table은 현재 local `kotlinx-benchmark` / JMH backend output을 기록한다.

### Single Thread

| Benchmark | Batch | Throughput ops/s |
|---|---:|---:|
| `uuidV7` | 100 | 423,830.666 |
| `ulid` | 100 | 270,204.172 |
| `uuidV4` | 100 | 105,158.728 |
| `ksuidMillis` | 100 | 59,593.488 |
| `ksuidSeconds` | 100 | 53,210.135 |
| `snowflake` | 100 | 40,973.126 |
| `flake` | 100 | 36,916.947 |
| `uuidV7` | 10000 | 4,163.944 |
| `ulid` | 10000 | 2,546.337 |
| `uuidV4` | 10000 | 1,031.892 |
| `ksuidMillis` | 10000 | 573.599 |
| `ksuidSeconds` | 10000 | 512.679 |
| `snowflake` | 10000 | 409.800 |
| `flake` | 10000 | 351.317 |

### Concurrent

| Benchmark | Batch | Throughput ops/s |
|---|---:|---:|
| `uuidV7` | 100 | 86,637.787 |
| `flake` | 100 | 31,977.885 |
| `uuidV4` | 100 | 30,331.895 |
| `snowflake` | 100 | 26,746.161 |
| `ksuidSeconds` | 100 | 25,726.586 |
| `ksuidMillis` | 100 | 25,220.817 |
| `ulid` | 100 | 24,476.006 |
| `uuidV7` | 10000 | 868.339 |
| `flake` | 10000 | 359.369 |
| `uuidV4` | 10000 | 298.486 |
| `ksuidMillis` | 10000 | 267.744 |
| `snowflake` | 10000 | 260.021 |
| `ksuidSeconds` | 10000 | 257.901 |
| `ulid` | 10000 | 243.299 |

## 해석 경계

- Go와 Kotlin은 Kotlin throughput을 `1e9 / (ops/s * batchSize)`로 `ns/id`에
  normalize한 뒤에만 같은 axis에서 비교한다.
- Kotlin batch result에는 Go package benchmark에 포함되지 않는 collection allocation과
  uniqueness verification이 여전히 포함된다.
- Go parallel benchmark coverage는 이제 UUID v4/v7, ULID random, monotonic ULID,
  KSUID seconds/millis, Snowflake를 포함하므로, concurrent chart row는 양쪽 repo가
  모두 노출하는 같은 generator family를 비교한다.
- Go UUID comparison row는 reused-generator benchmark를 사용한다. convenience
  `NewUUIDV4`와 `NewUUIDV7` row는 factory-plus-ID cost를 드러내기 위해서만 보존하며
  Kotlin-vs-Go chart에는 사용하지 않는다.
- Go Snowflake chart row는 synthetic-clock hot-path measurement다. deterministic
  clock hook과 caller-provided machine ID를 사용한다. JVM Snowflake row는 sibling
  library의 default generator setup을 사용하므로, Snowflake row만으로는
  production-equivalent cross-runtime evidence가 아니다.
- Snowflake benchmark row는 generated ID가 여러 millisecond window에 걸칠 만큼 충분히
  커야 한다. 이 algorithm은 per-millisecond sequence window를 사용하므로 작은 sample은
  timestamp tick 하나에 overfit될 수 있다. follow-up Snowflake work는 operation당
  최소 `4096 * 16` IDs를 측정하거나 equivalent multi-millisecond sample을 문서화해야
  한다.
- Go는 현재 Flake 또는 Hashids를 구현하지 않으므로, 해당 JVM result는 sibling-library
  context로만 기록한다.
- Go UUID v7과 JVM UUID v7은 서로 다른 implementation 및 clock/entropy hook을
  사용한다. performance difference를 standards compliance 차이로 읽으면 안 된다.
- KSUID seconds와 KSUID millis는 두 repo 모두에서 별도 family다. parsing, comparing,
  benchmarking에서 분리해 유지한다.

## 결론

- Go `id` benchmark suite에는 이제 first-class `make bench-id` target이 있고, 다른
  package benchmark suite와 일관되게 allocation을 보고한다.
- Go Snowflake synthetic-clock row는 이 snapshot에서 가장 낮은 measured `ns/id`를
  보였다. single-thread는 11.98 ns/id, concurrent는 92.37 ns/id이며 둘 다
  allocation-free다. 이는 Go implementation ceiling으로 봐야 하며,
  production-equivalent Go-vs-Kotlin Snowflake verdict로 보면 안 된다.
- 이 local normalized comparison에서 Go UUID row를 reuse generator 기준으로 보정한
  뒤에도 Kotlin `bluetape4k-idgenerators` row는 UUID v4/v7 및 KSUID seconds/millis에서
  더 낮은 `ns/id`를 보인다.
- ULID는 workload-sensitive로 남아 있다. single-thread row에서는 Kotlin monotonic
  ULID가 더 낮은 `ns/id`를 보이고, concurrent row에서는 Go monotonic ULID가 더 낮은
  `ns/id`를 보인다.
- Go string ID generator는 string value를 반환하고 entropy/encoding work에 의존하기
  때문에 allocation이 발생한다. 새 parallel row는 shared-generator pressure에서 이
  cost를 드러낸다.
- 이 자료는 universal language ranking이 아니라 local implementation snapshot으로
  다룬다. runtime, workload shape, UUID/ULID/KSUID implementation choice, Kotlin batch
  uniqueness check가 모두 수치에 영향을 준다.
- evidence-backed gap에 대해 follow-up issue를 만들었다.
  equivalent clock/batch condition에서 Snowflake를 재측정하고 최적화하기 위한
  `bluetape4k-projects#738`, UUID, ULID, KSUID를 위한 `bluetape-go#192`다.

## Follow-Up 관찰 항목

- 향후 더 엄격한 cross-runtime benchmark가 필요하면 normalized throughput을 비교하기
  전에 JVM uniqueness-check workload를 mirror하는 Go batch benchmark를 추가한다.
- Snowflake는 여러 millisecond window에 걸치지 않는 sample에서 결론을 내리지 않는다.
  sequence rollover와 timestamp advancement behavior를 exercise할 수 있도록
  `4096 * 16` IDs 같은 충분히 큰 batch를 사용한다.
- allocation pressure가 user-facing concern이 되면 현재의 단순 string-returning
  constructor를 바꾸기보다 byte-oriented 또는 caller-provided-buffer API를 별도로
  평가한다.
