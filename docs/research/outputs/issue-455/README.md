# Issue #455 zstd Allocation Profile Outputs

Issue: #455
Milestone: 0.15.0
Generated: 2026-07-08 KST

## Commands

- `go test -run '^$' -bench '^BenchmarkCompressorsCompress/json/large/zstd$' -benchmem -count=5 ./compression`
- `go test -run '^$' -bench '^BenchmarkSerializationSerializeThenCompress/JSON/serde-repeated-collection-v1/zstd$' -benchmem -count=5 ./serialization`
- `go test -run '^$' -bench '^BenchmarkCompressorsCompress/json/large/zstd$' -benchmem -memprofile docs/research/outputs/issue-455/zstd-compress-json-large-baseline.mem.pprof ./compression`
- `go test -run '^$' -bench '^BenchmarkSerializationSerializeThenCompress/JSON/serde-repeated-collection-v1/zstd$' -benchmem -memprofile docs/research/outputs/issue-455/zstd-serde-repeated-baseline.mem.pprof ./serialization`
- Equivalent after-change commands for the same benchmark rows and pprof files.

## Files

- `zstd-compress-json-large-baseline-bench.txt`
- `zstd-compress-json-large-after-bench.txt`
- `zstd-compress-json-large-baseline.mem.pprof`
- `zstd-compress-json-large-after.mem.pprof`
- `zstd-compress-json-large-baseline-mem-top.txt`
- `zstd-compress-json-large-after-mem-top.txt`
- `zstd-serde-repeated-baseline-bench.txt`
- `zstd-serde-repeated-after-bench.txt`
- `zstd-serde-repeated-baseline.mem.pprof`
- `zstd-serde-repeated-after.mem.pprof`
- `zstd-serde-repeated-baseline-mem-top.txt`
- `zstd-serde-repeated-after-mem-top.txt`
- `environment.md`

## Interpretation

Baseline profiles showed repeated zstd encoder allocation and history setup on
each `Compress` call. After pooling stream encoders inside zstd `Compress`, the
remaining allocation is dominated by output buffer growth and small warmup
costs. `EncodeAll` was rejected because it did not preserve stream-writer output
bytes for every tested payload.
