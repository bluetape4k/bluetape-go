# Issue #192 ID generator third comparison 연구

## 범위

이 note는 양쪽 optimization pass 이후의 세 번째 local comparison을 기록한다.

- Go side: issue #192의 `bluetape-go` optimized ID generators.
- Kotlin side: issue #738/#739의 `bluetape4k-idgenerators` candidate 3.
- informed decision: Go implementation, first comparison, Go optimization, second comparison, Kotlin optimization, third comparison을 다루는 `bluetape4k.github.io` article의 final data set.

## Revisions

| Repo | Revision |
|---|---|
| `bluetape-go` | `1599b0b` |
| `bluetape4k-projects` | `2ad25c100` |

## Commands

```bash
make bench-id
```

Kotlin row는 `bluetape4k-projects`의
`docs/benchmarks/raw/issue-738/candidate-3-focused.csv`를 사용하고, throughput을 다음
식으로 normalize한다.

```text
ns/id = 1e9 / (ops/s * 65536)
```

## Environment

- Go raw environment: `outputs/issue-192/third-comparison-environment.txt`
- Go raw benchmark: `outputs/issue-192/go-id-third-comparison-1s.txt`
- Cross-runtime table: `outputs/issue-192/third-comparison-kotlin-go.{csv,md}`
- OS/CPU: macOS arm64, Apple M4 Pro
- Go: 1.26.4
- Kotlin/JMH: GraalVM JDK 21.0.11, `kotlinx-benchmark`, batch size 65,536

## 결과

| Workload | Go ns/id | Kotlin ns/id | Lower | Note |
|---|---:|---:|---|---|
| Snowflake single | 11.98 | 244.02 | Go | Go synthetic clock; Kotlin real clock/batch ceiling |
| Snowflake concurrent | 90.01 | 243.65 | Go | Go synthetic clock; Kotlin real clock/batch ceiling |
| ULID monotonic single | 64.96 | 26.28 | Kotlin |  |
| ULID monotonic concurrent | 194.80 | 336.00 | Go |  |
| KSUID seconds single | 215.60 | 175.86 | Kotlin |  |
| KSUID seconds concurrent | 256.40 | 242.50 | Kotlin |  |
| KSUID millis single | 123.50 | 167.07 | Go | Kotlin full candidate 3 run; targeted repeat was 157.03 ns/id |
| KSUID millis concurrent | 215.90 | 215.81 | Kotlin | Effectively tied |

## 해석

- Snowflake는 수치상 여전히 Go에 유리하지만, 해당 row는 production-equivalent가 아니다. Go는 synthetic clock hook을 사용하고, Kotlin은 real clock을 사용하며 65,536-ID batch에서 4,096/ms sequence ceiling에 닿는다.
- direct entropy/payload 변경 이후 Kotlin은 single-thread monotonic ULID와 KSUID seconds에서 앞선다.
- Go는 여전히 concurrent monotonic ULID와 single-thread KSUID millis에서 앞선다.
- KSUID millis concurrent는 이 local snapshot에서 practical tie다. Go는 215.90 ns/id, Kotlin은 215.81 ns/id로 측정되었다.
- Kotlin Snowflake의 main improvement는 throughput이 아니라 allocation이다. issue #738 profile evidence는 `snowflakeDefaultWithUniqueness` normalized allocation이 약 14.58 MB/op에서 6.97 MB/op로 줄었음을 보여 준다.

## Blog Seed

article은 단순한 "language X is faster" 결론을 피해야 한다. data는 bottleneck이 generator family별로 이동했음을 보여 준다.

- Go optimization은 주로 entropy-read overhead를 제거하고 KSUID/ULID concurrency를 개선했다.
- Kotlin optimization은 Snowflake intermediate allocation을 제거하고 ULID/KSUID entropy staging을 줄였다.
- clock model과 benchmark shape가 Snowflake 해석을 지배한다.
- KSUID millis는 concurrent load에서 명확한 Go lead에서 near parity로 이동했다.
