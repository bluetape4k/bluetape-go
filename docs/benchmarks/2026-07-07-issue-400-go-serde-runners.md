# Issue #400 Go SerDe Benchmark Runners

> 한국어 벤치마크 경계: 이 문서는 벤치마크 목적과 해석 한계를 한국어 독자가 추적할 수 있도록 정리한다. 벤치마크 이름, 명령, raw output 경로, fixture 이름, 수치 증거는 원문의 재현성 앵커로 보존한다.

이슈: #400
상위: #398
마일스톤: 0.14.0
날짜: 2026-07-07
작업 유형: Benchmark runner implementation

## 목표

Standardize Go benchmark entry points for `serialization`, `codec`, and
`compression` so the #399 fixture and scenario contract can produce comparable
raw output for later preservation in #401 and interpretation in #402.

This document records commands and runner scope only. It does not preserve the
canonical raw output set or make a production ranking.

## 러너 매핑

| 패키지 | 벤치마크 진입점 | fixture 커버리지 | 메모 |
|---|---|---|---|
| `serialization` | `BenchmarkSerializationEncode`, `BenchmarkSerializationDecode`, `BenchmarkSerializationRoundTrip`, `BenchmarkSerializationSerializeThenCompress` | Small object, medium nested object, repeated collection, binary bytes, text bytes, and Go `BTGS` versioned envelope. | JSON, raw byte, raw string, and versioned serializer contracts stay package-local to `_test.go`. |
| `codec` | `BenchmarkCodecEncode`, `BenchmarkCodecDecode`, `BenchmarkCodecRoundTrip`, `BenchmarkCodecUUIDURL62` | Base64/Base64URL/Hex cover small, medium, binary, and repeated payloads. Base58/Base62/URL62 cover small payload plus UUID-sized bytes. | Large Base58/Base62 rows are intentionally excluded because the current byte-array alphabet implementation is not the cross-repo large-payload codec path. |
| `compression` | `BenchmarkCompressorsCompress`, `BenchmarkCompressorsDecompress` | Existing deterministic JSON, text, binary, and random payloads at small, medium, and large sizes. | Compression runners already report compressed bytes and compressed/original ratio. |

## 재현 가능한 명령

Create a local artifact directory before collecting raw output:

```bash
mkdir -p docs/research/outputs/issue-400
```

Run the package benchmarks with allocation reporting:

```bash
go test -run '^$' -bench '^BenchmarkSerialization' -benchmem ./serialization | tee docs/research/outputs/issue-400/go-serialization-bench.txt
go test -run '^$' -bench '^BenchmarkCodec' -benchmem ./codec | tee docs/research/outputs/issue-400/go-codec-bench.txt
go test -run '^$' -bench '^BenchmarkCompressors' -benchmem ./compression | tee docs/research/outputs/issue-400/go-compression-bench.txt
```

Use short smoke runs while editing benchmark code:

```bash
go test -run '^$' -bench '^BenchmarkSerialization' -benchmem -benchtime=1x ./serialization
go test -run '^$' -bench '^BenchmarkCodec' -benchmem -benchtime=1x ./codec
go test -run '^$' -bench '^BenchmarkCompressors' -benchmem -benchtime=1x ./compression
```

## 산출물 경계

#400 makes the commands produce raw output files through `tee`. #401 owns the
durable retention format at `docs/research/outputs/issue-401/`, environment
metadata, revision capture, and links from recommendations back to accepted
output files.

## 해석 한계

- Treat every output as a local snapshot until #401 records OS, CPU, Go version,
  command, git SHA, dirty-tree state, fixture sizes, and metric direction.
- Do not compare Base58/Base62 large-payload absence as a throughput result.
  Their Go byte API remains useful for IDs and compact key material, not as the
  default large binary transport encoding.
- Do not compare compression density alone. Decode/decompress cost and
  correctness gates remain part of the #399 scenario matrix.
