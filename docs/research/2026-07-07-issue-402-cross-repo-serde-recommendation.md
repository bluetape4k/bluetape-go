# Issue #402 Cross-Repo SerDe Recommendation Matrix

Issue: #402
Parent: #398
Milestone: 0.14.0
Date: 2026-07-07
Work type: Recommendation matrix

## Goal

Publish the 0.14.0 cross-repo SerDe and compression recommendation matrix
without turning one benchmark window into a production ranking.

The matrix separates measured findings, caveats, and follow-up hypotheses for
`bluetape-go`, `bluetape-rs`, and JVM `bluetape4k-projects`.

## Evidence Inventory

| Evidence | Scope | Environment and raw output |
|---|---|---|
| Go #399 fixture contract | Scenario and fixture boundary for serialization, codec, and compression. | [docs/benchmarks/2026-07-07-issue-399-serde-fixtures.md](../benchmarks/2026-07-07-issue-399-serde-fixtures.md) |
| Go #401 retained outputs | Current Go serialization, codec, and compression raw outputs. | [docs/research/outputs/issue-401/environment.md](outputs/issue-401/environment.md), [go-serialization-bench.txt](outputs/issue-401/go-serialization-bench.txt), [go-codec-bench.txt](outputs/issue-401/go-codec-bench.txt), [go-compression-bench.txt](outputs/issue-401/go-compression-bench.txt) |
| Rust/JVM/Go same-condition compression snapshot | Prior normalized compression comparison on shared payload bytes. | [bluetape-rs compression report](https://github.com/bluetape4k/bluetape-rs/blob/8ab2bc46288dbec5982d9a9f00968c3cd0a984ee/docs/benchmark/compression-same-condition-benchmark.md), [metadata](https://github.com/bluetape4k/bluetape-rs/blob/8ab2bc46288dbec5982d9a9f00968c3cd0a984ee/docs/benchmark/compression-same-condition-metadata.md) |
| JVM serializer guidance | Fory/Kryo/JDK/Jackson throughput notes and fast-mode caveats. | [bluetape4k-projects io README](https://github.com/bluetape4k/bluetape4k-projects/blob/a7dcf538e624709fa8d46fc7ea0647f30068578a/io/io/README.md) |
| JVM trust boundary | Trusted-internal, allow-listed, and no-dynamic-type-loading guidance. | [serialization trust profiles](https://github.com/bluetape4k/bluetape4k-projects/blob/a7dcf538e624709fa8d46fc7ea0647f30068578a/docs/security/serialization-trust-profiles.md) |
| JVM compressor artifact | JVM same-condition compressor benchmark commands and raw artifact list. | [same-condition IO compressor benchmark](https://github.com/bluetape4k/bluetape4k-projects/blob/a7dcf538e624709fa8d46fc7ea0647f30068578a/docs/benchmarks/2026-06-11-io-same-condition-compressor-benchmark.md) |

## Metric Direction

| Metric | Direction | Applies to |
|---|---|---|
| `ns/op` | Lower is better for the same benchmark row and host. | Go benchmark rows |
| `B/op` | Lower is better. | Go allocation rows |
| `allocs/op` | Lower is better. | Go allocation rows |
| `MB/s` or `MiB/s` | Higher is better for the same fixture class. | Go, Rust, JVM throughput rows |
| `encoded_bytes` | Lower is denser, not a standalone performance winner. | Go codec/serialization rows |
| `compressed_bytes` | Lower is denser. | Go compression rows |
| `compressed/original` and `compressed/serialized` | Lower is denser. | Compression and serialize-then-compress rows |
| JVM `ops/s` | Higher is better within the same JMH shape. | JVM serializer README rows |

## Recommendation Matrix

| Use case | Go guidance | Rust guidance | JVM guidance | Measured evidence | Excluded interpretation |
|---|---|---|---|---|---|
| Go object payloads across storage/cache/message boundaries | Use `serialization.JSONSerializer` for JSON compatibility and `VersionedSerializer` when Go-local `BTGS` envelope metadata is needed. | No cross-runtime adapter recommendation from Go evidence. | Do not infer JVM wire compatibility from Go JSON/versioned rows. | Go #401 reports JSON encode/decode/round-trip rows for small, medium, and repeated fixtures plus `BTGS` versioned envelope rows. | These rows do not choose Fory, Kryo, Protobuf, or Avro for Go. |
| Go byte/text transport encoding | Use Base64/Base64URL/hex for general byte transport. Keep Base58/Base62/URL62 for ID/key-sized values and UUID rendering. | Rust has matching codec families, but #402 does not have a retained Rust codec benchmark matrix. | JVM codec rows are not normalized with Go #401 codec rows. | Go #401 codec output shows Base64/Base64URL/hex covering object, binary, and repeated fixtures; Base58/Base62/URL62 are intentionally limited to small and UUID fixtures. | Do not promote Base58/Base62/URL62 as large binary transport codecs. |
| Object serialize-then-compress in Go | Evaluate zstd first when density matters; evaluate lz4 or snappy first when CPU/latency matters. | Use the Rust compression same-condition report for broad direction, then rerun in Rust before changing defaults. | JVM has matching compressor evidence, but serializer-compressor choices must respect trust profiles. | Go #401 serialize-then-compress rows show zstd/deflate/gzip denser on many JSON object fixtures while lz4/snappy are much faster on medium/repeated paths. | Compression density alone is not a production default. Decode/decompress cost and trust boundary still apply. |
| Large structured payload compression | Prefer zstd as the first density candidate; evaluate lz4/snappy when throughput dominates. | Rust same-condition report shows zstd best ratio for structured payloads and lz4/snappy as high-throughput candidates. | JVM same-condition report shows the same broad zstd-density and lz4/snappy-throughput split. | Go #401 large JSON/text/binary rows: zstd ratios `0.05706`, `0.009397`, `0.01992`; snappy/lz4 often lead compression throughput. Rust report large-payload rows preserve the cross-ecosystem comparison. | Do not compare current Go #401 numbers directly against the older Rust/JVM same-condition snapshot as a strict ranking. |
| Large payload decompression | Use lz4 as the first candidate when decompression latency dominates, then confirm with service payloads. | Same broad direction from the same-condition compression report. | Same broad direction from the JVM same-condition report. | Go #401 large JSON/text/binary/random decompression rows show lz4 as the fastest listed decompressor for those retained rows. | Decompression speed does not imply best compression ratio or smallest storage footprint. |
| Interoperability with common external systems | Keep gzip and deflate available as compatibility choices. | Keep gzip/deflate optional and explicit. | Keep gzip/deflate for compatibility with Java ecosystem tools. | Go #401 and same-condition reports retain gzip/deflate rows. | Compatibility-oriented codecs are not usually throughput or density winners in the retained snapshots. |
| Random or already-compressed payloads | Avoid compressing by default; measure before adding CPU cost. | Same-condition report shows ratios near or above `1.0`. | Same-condition report shows ratios near or above `1.0`. | Go #401 random large rows report `compressed/original` around `1.000`. Rust/JVM same-condition rows show the same broad behavior. | Random-payload rows must not be generalized to structured JSON/text/binary payloads. |
| JVM fixed-schema volatile caches | Not a Go recommendation. | Not a Rust recommendation until concrete adapters exist. | `ForyBinarySerializer.fast()` is the leading JVM throughput candidate for fixed-schema volatile caches; compressed FastFory variants trade throughput for size. | JVM README reports `ForyBinarySerializer.fast()` around `116K ops/s`, `BinarySerializers.Fory` around `68K ops/s`, and compressed FastFory variants around `12K-30K ops/s`. | FastFory uses a different wire mode from default Fory and is for trusted internal fixed-schema boundaries, not shared untrusted input. |
| JVM persistent or shared boundaries | Not a Go recommendation. | Not a Rust recommendation. | Prefer allow-listed or no-dynamic-type-loading boundaries. Use Fory/Kryo only where trust profile and schema evolution are explicit. | JVM trust profile document marks Kryo/Fory binary serializers as `TrustedInternal` by default and documents secure factories for shared boundaries. | Benchmark speed must not override deserialization safety. JDK remains compatibility/deprecated, not a new default. |
| Rust serialization adapters | Go does not define Rust adapters. | Keep the current `bluetape-rs-serialization` contract-only stance until adapter issues add concrete JSON/CBOR/MessagePack/Protobuf/Avro/Fory evidence. | JVM evidence does not fill Rust adapter gaps. | Rust README and serialization docs describe metadata, trust profiles, typed errors, safe config defaults, and serde-compatible traits; concrete adapters are follow-up work. | Do not create Rust adapter optimization issues from Go/JVM measurements alone. |

## Stable User-Facing Guidance

- `compression.Default()` remains zstd in Go. The retained evidence supports
  zstd as a density-first candidate, not a universal production winner.
- lz4 and snappy are the first candidates when latency or throughput dominates
  and a larger output is acceptable.
- gzip and deflate remain compatibility choices.
- Base64/Base64URL/hex are general byte transport encodings. Base58/Base62/URL62
  remain compact ID/key surfaces.
- Go `serialization` stays small and safe: JSON, bytes, strings, and the Go
  `BTGS` envelope. JVM Fory/Kryo choices are separate wire formats.
- JVM Fory/Kryo guidance must carry trust-profile language. Trusted-internal
  serializer speed is not enough for shared or untrusted boundaries.
- Rust serialization remains contract-first until concrete adapter evidence
  exists.

## Caveats

- Go #401 was captured on Apple M5, darwin/arm64, Go 1.26.4.
- Rust/JVM same-condition compression evidence is from a 2026-06-11 local
  snapshot with shared payload bytes. It is useful for broad direction, not for
  strict regression thresholds.
- Current Go #401 compression rows and the older Rust/JVM same-condition
  snapshot are not the same run. This report therefore names broad candidates
  instead of cross-runtime winners.
- JVM serializer rows use documented JMH throughput shapes and different
  fixtures from the Go #399 object fixtures.
- Allocation data is Go-only in the retained #401 evidence.

## Follow-Up Hypotheses

These are evidence-backed candidates for #403 triage, not issues created by this
report:

| Candidate | Evidence | Suggested #403 handling |
|---|---|---|
| Go zstd allocation cost on serialize-then-compress | Go #401 serialize-then-compress zstd rows allocate far more bytes than lz4/snappy rows while often improving density. | Profile zstd writer/buffer reuse before changing public defaults. |
| Go JSON decode allocation cost for repeated collections | Go #401 repeated JSON decode/round-trip rows allocate more than simple byte/string serializer rows. | Profile fixture-specific allocation sources before adding pooling. |
| Go codec large-payload guidance | Base64/Base64URL/hex have broad byte-payload coverage; Base58/Base62/URL62 are small/UUID scoped. | Keep documentation boundary; no optimization issue unless a real large-ID/key workload appears. |
| Cross-repo full rerun | Rust/JVM same-condition compression evidence predates Go #401. | If a release note needs strict cross-runtime ranking, rerun all three ecosystems from one committed fixture contract. |

## No New Optimization Issues From #402

#403 already owns creating follow-up optimization issues. This report provides
the candidate evidence and intentionally does not open new narrow optimization
issues before #403 ranks them.
