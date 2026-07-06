# Issue #400 Go SerDe Benchmark Runners

Issue: #400
Parent: #398
Milestone: 0.14.0
Date: 2026-07-07
Work type: Benchmark runner implementation

## Goal

Standardize Go benchmark entry points for `serialization`, `codec`, and
`compression` so the #399 fixture and scenario contract can produce comparable
raw output for later preservation in #401 and interpretation in #402.

This document records commands and runner scope only. It does not preserve the
canonical raw output set or make a production ranking.

## Runner Map

| Package | Benchmark entry points | Fixture coverage | Notes |
|---|---|---|---|
| `serialization` | `BenchmarkSerializationEncode`, `BenchmarkSerializationDecode`, `BenchmarkSerializationRoundTrip`, `BenchmarkSerializationSerializeThenCompress` | Small object, medium nested object, repeated collection, binary bytes, text bytes, and Go `BTGS` versioned envelope. | JSON, raw byte, raw string, and versioned serializer contracts stay package-local to `_test.go`. |
| `codec` | `BenchmarkCodecEncode`, `BenchmarkCodecDecode`, `BenchmarkCodecRoundTrip`, `BenchmarkCodecUUIDURL62` | Base64/Base64URL/Hex cover small, medium, binary, and repeated payloads. Base58/Base62/URL62 cover small payload plus UUID-sized bytes. | Large Base58/Base62 rows are intentionally excluded because the current byte-array alphabet implementation is not the cross-repo large-payload codec path. |
| `compression` | `BenchmarkCompressorsCompress`, `BenchmarkCompressorsDecompress` | Existing deterministic JSON, text, binary, and random payloads at small, medium, and large sizes. | Compression runners already report compressed bytes and compressed/original ratio. |

## Reproducible Commands

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

## Artifact Boundary

#400 makes the commands produce raw output files through `tee`. #401 owns the
durable retention format at `docs/research/outputs/issue-401/`, environment
metadata, revision capture, and links from recommendations back to accepted
output files.

## Interpretation Limits

- Treat every output as a local snapshot until #401 records OS, CPU, Go version,
  command, git SHA, dirty-tree state, fixture sizes, and metric direction.
- Do not compare Base58/Base62 large-payload absence as a throughput result.
  Their Go byte API remains useful for IDs and compact key material, not as the
  default large binary transport encoding.
- Do not compare compression density alone. Decode/decompress cost and
  correctness gates remain part of the #399 scenario matrix.
