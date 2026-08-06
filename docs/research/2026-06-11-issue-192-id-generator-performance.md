# Issue #192 ID generator performance optimization 연구

## 범위

Issue #192는 Issue #168 Kotlin-vs-Go benchmark snapshot을 따른다. Issue #168 chart와
raw output은 pre-optimization baseline으로 유지한다. 이 note는 UUID v4/v7, random
ULID, monotonic ULID, KSUID seconds, Kotlin-compatible KSUID millis에 대한 Go-side
optimization pass를 기록한다.

## Environment

- Raw environment: `outputs/issue-192/environment.txt`
- Baseline benchmark: `outputs/issue-192/go-id-baseline-count10.txt`
- Baseline CPU profile: `outputs/issue-192/id-baseline.cpu.pprof`
- Baseline memory profile: `outputs/issue-192/id-baseline.mem.pprof`
- Final benchmark: `outputs/issue-192/go-id-default-buffer-count10.txt`
- Final CPU profile: `outputs/issue-192/id-after-default-buffer.cpu.pprof`
- Final memory profile: `outputs/issue-192/id-after-default-buffer.mem.pprof`
- Before/after chart:
  `../images/readme-charts/id-generator-optimization-before-after.png`
- Post-optimization Kotlin-vs-Go chart:
  `../images/readme-charts/id-generator-kotlin-go-optimized-comparison.png`

![ID generator optimization before and after](../images/readme-charts/id-generator-optimization-before-after.png)

![Kotlin vs Go optimized ID generator comparison](../images/readme-charts/id-generator-kotlin-go-optimized-comparison.png)

## 명령

Baseline benchmark:

```bash
go test -run '^$' -bench 'Benchmark(UUID|ULID|KSUID)' -benchmem -count=10 ./id \
  | tee docs/research/outputs/issue-192/go-id-baseline-count10.txt
```

Baseline profile:

```bash
go test -run '^$' \
  -bench 'Benchmark(UUIDV4ReuseGenerator|UUIDV7ReuseGenerator|ULIDRandom|ULIDMonotonic|KSUIDNextString|KSUIDMillisNextString)$' \
  -benchmem -benchtime=3s \
  -cpuprofile docs/research/outputs/issue-192/id-baseline.cpu.pprof \
  -memprofile docs/research/outputs/issue-192/id-baseline.mem.pprof ./id \
  | tee docs/research/outputs/issue-192/go-id-baseline-profile-bench.txt
```

Final benchmark:

```bash
go test -run '^$' -bench 'Benchmark(UUID|ULID|KSUID)' -benchmem -count=10 ./id \
  | tee docs/research/outputs/issue-192/go-id-default-buffer-count10.txt
```

Final profile:

```bash
go test -run '^$' \
  -bench 'Benchmark(UUIDV4ReuseGenerator|UUIDV7ReuseGenerator|ULIDRandom|ULIDMonotonic|KSUIDNextString|KSUIDMillisNextString)$' \
  -benchmem -benchtime=3s \
  -cpuprofile docs/research/outputs/issue-192/id-after-default-buffer.cpu.pprof \
  -memprofile docs/research/outputs/issue-192/id-after-default-buffer.mem.pprof ./id \
  | tee docs/research/outputs/issue-192/go-id-after-default-buffer-profile-bench.txt
```

Comparison and profile summaries:

```bash
$(go env GOPATH)/bin/benchstat \
  docs/research/outputs/issue-192/go-id-baseline-count10.txt \
  docs/research/outputs/issue-192/go-id-default-buffer-count10.txt \
  | tee docs/research/outputs/issue-192/benchstat-default-buffer.txt

go tool pprof -top docs/research/outputs/issue-192/id-baseline.cpu.pprof \
  | tee docs/research/outputs/issue-192/id-baseline-cpu-top.txt

go tool pprof -top docs/research/outputs/issue-192/id-after-default-buffer.cpu.pprof \
  | tee docs/research/outputs/issue-192/id-after-default-buffer-cpu-top.txt
```

## Baseline Findings

10회 실행한 `benchstat` 결과는 generator reuse만으로는 핵심 performance lever를 설명할
수 없음을 보여 주었다.

| Benchmark | Baseline ns/op | Baseline B/op | Baseline allocs/op |
|---|---:|---:|---:|
| UUID v4 reused | 224.10 | 64 | 2 |
| UUID v7 reused | 255.60 | 64 | 2 |
| ULID random | 104.50 | 48 | 2 |
| ULID monotonic | 65.80 | 48 | 2 |
| KSUID seconds | 393.10 | 48 | 2 |
| KSUID millis | 316.80 | 104 | 3 |

baseline CPU profile은 sampled time의 약 4분의 1이 crypto randomness path를 통한
`io.ReadFull`에 있음을 보여 주었다. memory profile은 UUID, ULID, KSUID의 안정적인
string/allocation cost와 KSUID millis encoder의 추가 allocation을 보여 주었다.

## 채택한 변경

### Buffered crypto entropy

default UUID, ULID, KSUID seconds, KSUID millis generation은 이제 `crypto/rand` 위의
package-local locked buffered reader를 공유한다.

이 방식은 default entropy source를 crypto-grade로 유지하면서 per-ID randomness
overhead를 줄인다. 생성된 ID는 authentication 또는 authorization secret이 아니라
identifier로 남는다. custom reader를 주입하는 caller는 이전 behavior를 유지하며,
generator를 공유할 때는 여전히 concurrency-safe reader를 제공해야 한다.

### KSUID millis prefix encoding

KSUID millis `NextString`은 temporary output slice를 allocation한 뒤 resulting string을
truncating하지 않고, 필요한 27-character prefix를 fixed local buffer에 encode한다.

## 거절 또는 보류한 변경

- generator instance reuse만 적용: API shape와 allocation에는 유용하지만, 큰 gap을
  단독으로 설명하지 못했다.
- Segment KSUID string output stack-buffering: single-thread KSUID seconds를 약
  1.12% regressed시켰고 allocation을 줄이지 못했다.
- ULID string marshaling을 main lever로 취급: direct `MarshalTextTo`는 random ULID를
  약간 개선했지만 allocation count를 줄이지 못했다. 더 큰 ULID 개선은 entropy
  buffering에서 나왔다.
- UUID v7 sharding 또는 per-goroutine sequence allocation: parallel UUID v7에 대한
  가능한 follow-up이지만, 첫 bottleneck은 entropy reads였다.

## 최종 결과

baseline 대비 최종 `benchstat`다. `NewString` row는 convenience generator construction을
포함하고, `reused` row는 existing generator의 `NextString()` path를 측정한다.

| Benchmark | Baseline ns/op | Final ns/op | Change |
|---|---:|---:|---:|
| UUID v4 NewString | 238.45 | 58.43 | -75.50% |
| UUID v4 NewString parallel | 533.20 | 169.10 | -68.29% |
| UUID v4 reused | 224.10 | 45.57 | -79.67% |
| UUID v4 reused parallel | 500.20 | 145.00 | -71.01% |
| UUID v7 NewString | 267.60 | 88.04 | -67.10% |
| UUID v7 NewString parallel | 562.90 | 192.20 | -65.86% |
| UUID v7 reused | 255.60 | 73.95 | -71.07% |
| UUID v7 reused parallel | 343.60 | 202.80 | -40.98% |
| ULID random | 104.50 | 63.12 | -39.60% |
| ULID random parallel | 278.50 | 166.10 | -40.36% |
| ULID monotonic | 65.80 | 65.31 | -0.74% |
| ULID monotonic parallel | 181.50 | 180.20 | -0.74% |
| KSUID seconds | 393.10 | 217.90 | -44.58% |
| KSUID seconds parallel | 668.20 | 244.20 | -63.46% |
| KSUID millis | 316.80 | 122.80 | -61.23% |
| KSUID millis parallel | 621.00 | 209.00 | -66.35% |

geomean latency는 58.34% 개선되었다. KSUID millis allocation은
`104 B/op, 3 allocs/op`에서 `56 B/op, 2 allocs/op`로 개선되었다. 다른 string ID
family는 같은 allocation count를 유지했으므로, main improvement는 string allocation
removal이 아니라 read-path latency였다.

변경 이후 crypto/sysrand profile share는 크게 낮아졌고, visible hot spot은 string
encoding, time access, UUID v7 ordering, dependency encoder로 이동했다. 이들은 별도
follow-up optimization topic이다.

## 최적화 이후 Kotlin 비교

Issue #168 Kotlin row는 JVM baseline으로 유지하며, `kotlinx-benchmark` throughput을
`1e9 / (ops/s * 100)`으로 normalize한다. 아래 Go UUID, ULID, KSUID row는 Issue #192
optimized local snapshot을 사용한다. Snowflake는 이 issue에서 변경되지 않았으므로 Go
row는 Issue #168 synthetic-clock hot-path value로 남고, production-equivalent
cross-runtime Snowflake verdict로 읽으면 안 된다.

### Single Thread

| Generator | Go ns/id | Kotlin ns/id | Lower row |
|---|---:|---:|---|
| Snowflake synthetic clock | 11.98 | 244.06 | Go |
| ULID monotonic | 65.31 | 37.01 | Kotlin |
| UUID v4 reused generator | 45.57 | 95.09 | Go |
| UUID v7 reused generator | 73.95 | 23.59 | Kotlin |
| KSUID millis | 122.80 | 167.80 | Go |
| KSUID seconds | 217.90 | 187.93 | Kotlin |

### Concurrent

| Generator | Go ns/id | Kotlin ns/id | Lower row |
|---|---:|---:|---|
| Snowflake synthetic clock | 92.37 | 373.89 | Go |
| ULID monotonic | 180.20 | 408.56 | Go |
| UUID v4 reused generator | 145.00 | 329.69 | Go |
| UUID v7 reused generator | 202.80 | 115.42 | Kotlin |
| KSUID millis | 209.00 | 396.50 | Go |
| KSUID seconds | 244.20 | 388.70 | Go |

updated chart는 해석을 크게 바꾼다. Go entropy를 buffering한 뒤 이 local snapshot에서
Go는 UUID v4, KSUID millis, concurrent ULID/KSUID row를 앞선다. Kotlin은 여전히 UUID
v7과 single-thread monotonic ULID row를 앞선다. entropy overhead를 줄인 뒤에도
parallel row가 ordering/lock cost를 보여 주므로 UUID v7은 가장 명확한 Go follow-up으로
남는다.

## Blog Seed

이 issue는 초기 hypothesis가 흥미로운 방식으로 틀렸기 때문에 좋은 blog topic이다. Go가
압도할 것으로 예상했지만 generator reuse는 조금만 도움이 되었고, profiling은 random
entropy reads와 string encoding이 실제 constraint임을 보여 주었다. Issue #168 chart를
baseline visual로 유지하고 Issue #192 before/after table을 optimization story로
추가한다.
