# Issue #455 Review: zstd Allocation Profile

## Scope

- `compression.ZstdLevel` and zstd `Compress` implementation.
- zstd output byte compatibility with `NewWriter`.
- zstd concurrent shared-compressor behavior.
- Benchmark/profile evidence for compression and SerDe zstd rows.
- README and research notes.

## Diagram Decision

No diagram change is required. The change is an internal compressor allocation
optimization. It does not add a public topology, lifecycle, class relationship,
or sequence that would be clearer as a README diagram.

## Lane Findings

### Performance

P0: 0
P1: 0

- `json/large/zstd` allocation dropped from about `19.67 MB/op` to
  `124-131 KB/op`.
- `serde-repeated-collection-v1/zstd` allocation dropped from about
  `19.62 MB/op` to `73 KB/op`.
- `compressed_bytes` stayed unchanged on both target rows.

### Stability

P0: 0
P1: 0

- `TestZstdCompressMatchesStreamWriter` pins byte equality between `Compress`
  and `NewWriter`.
- `TestZstdCompressConcurrentStress` covers shared pooled encoder use with
  `GoroutineStressTester`.

### Security

P0: 0
P1: 0

- No unsafe reuse, caller-visible buffer reuse, or decompression trust-boundary
  change was introduced.
- Returned compressed byte slices are backed by the per-call output buffer, not
  a pooled caller-visible buffer.

### Operator/Ops

P0: 0
P1: 0

- Raw benchmark and pprof artifacts are retained under
  `docs/research/outputs/issue-455/`.
- Environment and checksum data are recorded.

### Developer/API

P0: 0
P1: 0

- `compression.Default()` still returns zstd.
- `Zstd`, `ZstdLevel`, `NewWriter`, `NewReader`, `Compress`, and `Decompress`
  public API shape is unchanged.
- `NewWriter` still returns an independent stream writer.

### User/Caller

P0: 0
P1: 0

- Output byte compatibility with stream writer is tested.
- README notes clarify that the encoder reuse is internal to `Compress`.

## Integration Verdict

P0: 0
P1: 0

The change is narrow and evidence-backed. It removes repeated zstd encoder
allocation from byte-slice `Compress` without changing public API behavior or
the targeted benchmark output sizes.

## Evidence

- `gh issue view 455`
- `ctx_batch_execute` over #455, #404, SerDe benchmark docs, and compression
  implementation.
- `go doc github.com/klauspost/compress/zstd.Encoder.Reset`
- `go doc github.com/klauspost/compress/zstd.Encoder.EncodeAll`
- `go doc github.com/klauspost/compress/zstd.WithEncoderConcurrency`
- `go doc github.com/klauspost/compress/zstd.WithLowerEncoderMem`
- `go test -run '^$' -bench '^BenchmarkCompressorsCompress/json/large/zstd$' -benchmem -count=5 ./compression`
- `go test -run '^$' -bench '^BenchmarkSerializationSerializeThenCompress/JSON/serde-repeated-collection-v1/zstd$' -benchmem -count=5 ./serialization`
- `go test -run '^$' -bench '^BenchmarkCompressorsCompress/json/large/zstd$' -benchmem -memprofile docs/research/outputs/issue-455/zstd-compress-json-large-baseline.mem.pprof ./compression`
- `go tool pprof -top -alloc_space docs/research/outputs/issue-455/zstd-compress-json-large-baseline.mem.pprof`
- `go test -run '^$' -bench '^BenchmarkSerializationSerializeThenCompress/JSON/serde-repeated-collection-v1/zstd$' -benchmem -memprofile docs/research/outputs/issue-455/zstd-serde-repeated-baseline.mem.pprof ./serialization`
- `go tool pprof -top -alloc_space docs/research/outputs/issue-455/zstd-serde-repeated-baseline.mem.pprof`
- `gofmt -w compression/zstd.go compression/compression_test.go`
- `go test -count=1 ./compression ./serialization`
- `go test -race -count=1 ./compression ./serialization`
- After-change benchmark/profile commands for the same target rows.
- `go test -run '^$' -bench '^BenchmarkCompressorsCompress/json/large/zstd$' -benchmem -benchtime=1x ./compression`
- `go test -run '^$' -bench '^BenchmarkSerializationSerializeThenCompress/JSON/serde-repeated-collection-v1/zstd$' -benchmem -benchtime=1x ./serialization`
- `golangci-lint cache clean`
- `make ci`
