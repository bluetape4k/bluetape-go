# Issue #192 ID generator third comparison

## Scope

This note records the third local comparison after both optimization passes:

- Go side: `bluetape-go` optimized ID generators from issue #192.
- Kotlin side: `bluetape4k-idgenerators` candidate 3 from issue #738/#739.
- Decision being informed: final data set for the `bluetape4k.github.io` article covering the Go implementation, first comparison, Go optimization, second comparison, Kotlin optimization, and third comparison.

## Revisions

| Repo | Revision |
|---|---|
| `bluetape-go` | `1599b0b` |
| `bluetape4k-projects` | `2ad25c100` |

## Commands

```bash
make bench-id
```

Kotlin rows use `docs/benchmarks/raw/issue-738/candidate-3-focused.csv` from
`bluetape4k-projects` and normalize throughput with:

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

## Results

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

## Interpretation

- Snowflake still favors Go numerically, but the row is not production-equivalent: Go uses synthetic clock hooks while Kotlin uses the real clock and hits the 4,096/ms sequence ceiling in the 65,536-ID batch.
- Kotlin now leads single-thread monotonic ULID and KSUID seconds after direct entropy/payload changes.
- Go still leads concurrent monotonic ULID and single-thread KSUID millis.
- KSUID millis concurrent is a practical tie in this local snapshot: Go measured 215.90 ns/id and Kotlin measured 215.81 ns/id.
- Kotlin Snowflake's main improvement is allocation, not throughput: issue #738 profile evidence shows `snowflakeDefaultWithUniqueness` normalized allocation reduced from about 14.58 MB/op to 6.97 MB/op.

## Blog Seed

The article should avoid a simplistic "language X is faster" conclusion. The data says the bottleneck moved by generator family:

- Go optimization mostly removed entropy-read overhead and improved KSUID/ULID concurrency.
- Kotlin optimization removed intermediate allocation in Snowflake and reduced entropy staging in ULID/KSUID.
- Clock model and benchmark shape dominate Snowflake interpretation.
- KSUID millis moved from a clear Go lead to near parity under concurrent load.
