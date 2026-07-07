# Issue #455 zstd Allocation Profile

Issue: #455
Milestone: 0.15.0
Date: 2026-07-08

## Goal

Profile and reduce avoidable allocation cost in zstd compression paths used by
SerDe benchmark scenarios, without changing `compression.Default()` or public
API behavior.

## Baseline

Retained #401 evidence showed zstd allocation cost much higher than lz4 or
snappy in serialize-then-compress and compression rows.

Fresh #455 baseline on Apple M5:

- `BenchmarkCompressorsCompress/json/large/zstd`: `19,670,346-19,670,400 B/op`,
  `92 allocs/op`, `44889 compressed_bytes`.
- `BenchmarkSerializationSerializeThenCompress/JSON/serde-repeated-collection-v1/zstd`:
  `19,616,392-19,616,422 B/op`, `79 allocs/op`, `21297 compressed_bytes`.

## Profile Finding

Baseline allocation profiles showed repeated encoder construction and zstd
history setup:

- `zstd.(*fastBase).ensureHist`: about `85%` of alloc space.
- `zstd.encoderOptions.encoder`: about `6.7%`.
- `zstd.(*blockEnc).init`: about `3.1%`.
- `zstd.(*Encoder).Reset` / `zstd.NewWriter`: about `11.8%` cumulative.

The existing `streamCompressor.Compress` path created a new zstd stream writer
for every byte-slice compression call.

## Change

`ZstdLevel` now returns a zstd-specific compressor whose `Compress` method
reuses stream encoders through `sync.Pool`. The public `NewWriter` method still
returns an independent stream writer, so caller-managed stream lifecycle remains
unchanged.

`EncodeAll` was evaluated and rejected: it did reduce allocations, but it did
not preserve current stream-writer output bytes for empty and large tested
payloads.

## After Evidence

After pooling zstd stream encoders:

- `BenchmarkCompressorsCompress/json/large/zstd`: `124,028-130,910 B/op`,
  `29 allocs/op`, `44889 compressed_bytes`.
- `BenchmarkSerializationSerializeThenCompress/JSON/serde-repeated-collection-v1/zstd`:
  `73,317-73,540 B/op`, `16 allocs/op`, `21297 compressed_bytes`.

The compressed byte counts remained unchanged for the targeted benchmark rows.
The new `TestZstdCompressMatchesStreamWriter` pins byte equality between
`Compress` and `NewWriter`, including empty input. `TestZstdCompressConcurrentStress`
uses `GoroutineStressTester` to cover shared-compressor concurrent calls under
the new pool.

## Decision

Accept the narrow `Compress`-only encoder reuse. Do not change
`compression.Default()`, zstd level semantics, `NewWriter`, or public API shape.
Do not use `EncodeAll` unless a separate decision explicitly permits wire-byte
changes.

Raw outputs are stored under
[`docs/research/outputs/issue-455/`](outputs/issue-455/README.md).
