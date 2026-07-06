# compression

[English](README.md) | [한국어](README.ko.md)

`compression` exposes one compressor contract for byte slices and streams.
Gzip, zlib, and raw deflate use Go's standard library. Zstd, lz4, and snappy
use focused Go dependencies behind the same interface.

## Import

```go
import "github.com/bluetape4k/bluetape-go/compression"
```

## Usage

![compression byte and stream flow](../docs/images/readme-diagrams/compression-byte-stream-flow.png)

```go
compressor := compression.Default()
compressed, err := compressor.Compress(payload)
if err != nil {
    return err
}

decompressed, err := compressor.Decompress(compressed)
if err != nil {
    return err
}
```

For externally supplied compressed bytes, bound the expanded output:

```go
decompressed, err := compression.DecompressLimit(compressor, compressed, 8<<20)
if err != nil {
    return err
}
```

## Behavior

- `Default()` currently returns zstd.
- `All()` returns gzip, zlib, deflate, zstd, lz4, and snappy in a stable order.
- `Decompress` is for already-bounded or trusted payloads; use
  `DecompressLimit` for untrusted byte-slice input.
- Stream APIs reject nil readers or writers.
- Level-specific constructors are available for gzip, zlib, deflate, and zstd.

## Test

```bash
go test -count=1 ./compression
```

## Benchmark

```bash
go test -run '^$' -bench '^BenchmarkCompressors' -benchmem ./compression
```

The benchmark runners cover deterministic JSON, text, binary, and random byte
payloads across the compressors returned by `All()`. Use
`docs/benchmarks/2026-07-07-issue-400-go-serde-runners.md` when collecting raw
output artifacts for the 0.14.0 SerDe baseline.
