# Issue #455 zstd Allocation Profile

Issue: #455
Milestone: 0.15.0
Date: 2026-07-08

## Goal

`compression.Default()` 또는 public API behavior를 바꾸지 않고 SerDe benchmark scenario가 사용하는 zstd compression path의
피할 수 있는 allocation cost를 profile하고 줄인다.

## Baseline

보존된 #401 evidence는 serialize-then-compress 및 compression row에서 zstd allocation cost가 lz4 또는 snappy보다 훨씬 크다고
보였다.

Apple M5의 fresh #455 baseline:

- `BenchmarkCompressorsCompress/json/large/zstd`: `19,670,346-19,670,400 B/op`, `92 allocs/op`,
  `44889 compressed_bytes`.
- `BenchmarkSerializationSerializeThenCompress/JSON/serde-repeated-collection-v1/zstd`:
  `19,616,392-19,616,422 B/op`, `79 allocs/op`, `21297 compressed_bytes`.

## Profile Finding

baseline allocation profile은 반복 encoder construction과 zstd history setup을 보여 줬다.

- `zstd.(*fastBase).ensureHist`: alloc space의 약 `85%`.
- `zstd.encoderOptions.encoder`: 약 `6.7%`.
- `zstd.(*blockEnc).init`: 약 `3.1%`.
- `zstd.(*Encoder).Reset` / `zstd.NewWriter`: cumulative 약 `11.8%`.

기존 `streamCompressor.Compress` path는 byte-slice compression call마다 새 zstd stream writer를 만들었다.

## Change

`ZstdLevel`은 이제 `sync.Pool`로 stream encoder를 재사용하는 zstd-specific compressor를 반환한다. public `NewWriter` method는
여전히 independent stream writer를 반환하므로 caller-managed stream lifecycle은 바뀌지 않는다.

`EncodeAll`은 평가 후 거부했다. allocation은 줄였지만 empty 및 large tested payload에서 현재 stream-writer output byte를 보존하지
못했다.

## After Evidence

zstd stream encoder pooling 뒤:

- `BenchmarkCompressorsCompress/json/large/zstd`: `124,028-130,910 B/op`, `29 allocs/op`,
  `44889 compressed_bytes`.
- `BenchmarkSerializationSerializeThenCompress/JSON/serde-repeated-collection-v1/zstd`:
  `73,317-73,540 B/op`, `16 allocs/op`, `21297 compressed_bytes`.

target benchmark row의 compressed byte count는 그대로다. 새 `TestZstdCompressMatchesStreamWriter`는 empty input을 포함해
`Compress`와 `NewWriter`의 byte equality를 고정한다. `TestZstdCompressConcurrentStress`는 새 pool 아래 shared-compressor
concurrent call을 `GoroutineStressTester`로 덮는다.

## 결정

좁은 `Compress`-only encoder reuse를 수용한다. `compression.Default()`, zstd level semantics, `NewWriter`, public API shape는
바꾸지 않는다. 별도 결정으로 wire-byte change를 허용하기 전에는 `EncodeAll`을 쓰지 않는다.

raw output은
[`docs/research/outputs/issue-455/`](outputs/issue-455/README.md)에 저장한다.
