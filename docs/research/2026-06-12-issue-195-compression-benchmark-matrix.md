# Issue #195 Compression Benchmark Matrix

Issue: #195
Milestone: 0.6.1
Date: 2026-06-12
Work type: Benchmark evidence

## Research Question

How should `bluetape-go/compression` extend its existing compressor benchmarks
so the Go results can be compared with `bluetape-rs` and `bluetape4k-io`
without changing compressor defaults or overstating one local run?

## Current Decision

Keep the benchmark as an opt-in package benchmark and treat the output as a
same-condition local snapshot. The matrix now covers JSON, text, structured
binary, and low-compressibility random payloads at small, medium, and large
sizes. It measures compression and decompression separately for every
`compression.All()` algorithm.

Do not change `compression.Default()` from this benchmark alone. If a future
cross-ecosystem report wants a recommendation, it must combine the Go raw output
with sibling Rust and JVM outputs and preserve runtime caveats.

## Payload Matrix

| Kind | Small | Medium | Large | Shape |
|---|---:|---:|---:|---|
| JSON | about 1 KiB | about 48 KiB | about 768 KiB | service-event objects in a valid JSON document |
| Text | 1 KiB | 48 KiB | 768 KiB | deterministic UTF-8 service-log and prose lines |
| Binary | 1 KiB | 48 KiB | 768 KiB | deterministic mixed bytes with repeated low-entropy regions |
| Random | 1 KiB | 48 KiB | 768 KiB | deterministic PCG low-compressibility bytes |

The benchmark uses a stable payload slice rather than map iteration so output
order is deterministic across local runs.

## Commands

```bash
go test -count=1 ./compression
go test -run '^$' -bench '^BenchmarkCompressors' -benchmem ./compression
```

Raw environment and benchmark outputs are stored under
`docs/research/outputs/issue-195/`.

| File | Purpose |
|---|---|
| `environment.txt` | Host, OS, Go version, CPU, logical CPU count, branch, base commit, and dirty tree state for the benchmarked PR diff. |
| `go-compression-bench.txt` | Full Go benchmark output for compression and decompression paths. |

Observed local environment:

- macOS arm64, Apple M4 Pro, 12 logical CPUs.
- Go 1.26.4.
- Branch `issue/195-compression-benchmark-matrix`, based on
  `aba437a50d0a2a2246e1dd0e2271c6672458143f`, with the issue #195 working-tree
  diff recorded in `environment.txt`.

## Snapshot Highlights

The table below records representative large-payload rows from the raw output.
Lower `ns/op` is faster, higher `MB/s` is higher throughput, and lower
`compressed/original` is better compression density.

### Compression - Large Payloads

| Payload | gzip ns/op | zstd ns/op | lz4 ns/op | snappy ns/op | Best density among listed |
|---|---:|---:|---:|---:|---|
| JSON large | 3,480,521 | 652,046 | 506,451 | 366,504 | zstd, 0.05706 |
| Text large | 1,149,411 | 365,913 | 221,996 | 182,951 | zstd, 0.009397 |
| Binary large | 3,603,652 | 614,392 | 578,394 | 496,773 | zstd, 0.01992 |
| Random large | 6,873,184 | 405,417 | 108,362 | 189,504 | no useful compression, about 1.000 |

### Decompression - Large Payloads

| Payload | gzip ns/op | zstd ns/op | lz4 ns/op | snappy ns/op | Fastest among listed |
|---|---:|---:|---:|---:|---|
| JSON large | 675,140 | 566,753 | 206,585 | 290,147 | lz4 |
| Text large | 352,075 | 413,314 | 156,334 | 258,709 | lz4 |
| Binary large | 671,708 | 490,537 | 207,478 | 325,814 | lz4 |
| Random large | 188,242 | 279,243 | 159,177 | 191,426 | lz4 |

## Interpretation Boundary

- This is one local run, not a production ranking.
- The Go benchmark reports package-level `ns/op`, throughput, allocations,
  compressed bytes, and compressed/original ratio. JVM and Rust harnesses may
  expose different allocation or runtime metrics.
- Zstd often provides the best compression density on structured payloads in
  this snapshot, while lz4 and snappy tend to be faster on many paths.
- Random payload rows demonstrate overhead when the data cannot be usefully
  compressed; they should not be interpreted as a default-algorithm winner.
- Cross-ecosystem comparison should keep each runtime's raw table separate
  before presenting normalized summaries.

## Linked Evidence

- `bluetape-go` issue #76 established deterministic opt-in compression
  benchmarks with compressed byte and ratio metrics.
- `bluetape4k-projects` issue #746 added the sibling JVM same-condition matrix.
- `bluetape-rs` issue #82 records the broader cross-ecosystem comparison shape.
