# Issue #401 Benchmark Environment

Generated UTC: 2026-07-06T20:55:05Z
Generated local: 2026-07-07 05:55:05 KST

## Host

- OS/arch: darwin/arm64
- Kernel: Darwin debop 25.5.0 Darwin Kernel Version 25.5.0: Mon Apr 27 20:39:29 PDT 2026; root:xnu-12377.121.6~2/RELEASE_ARM64_T8142 arm64
- CPU: Apple M5
- Physical CPUs: 10
- Logical CPUs: 10
- Go: go version go1.26.4 darwin/arm64

## Package Revision

- Branch: issue-401-benchmark-artifact-retention
- Package code base: 80f4a0fd68f3f564fef54950c73fca5604fa6272
- Base ref: origin/develop 80f4a0fd68f3f564fef54950c73fca5604fa6272
- Benchmark artifact issue: #401
- Benchmark runner source issue: #400

### Dirty Tree At Capture

```text
 M docs/benchmarks/2026-07-07-issue-400-go-serde-runners.md
 M docs/research/README.ko.md
 M docs/research/README.md
?? docs/lessons/2026-07-07-benchmark-artifact-retention.md
?? docs/research/2026-07-07-issue-401-benchmark-artifact-retention.md
?? docs/research/benchmark-artifact-template.md
?? docs/research/outputs/issue-401/
?? docs/review/2026-07-07-issue-401-benchmark-artifact-retention-review.md
```

### Diff Stat At Capture

```text
 docs/benchmarks/2026-07-07-issue-400-go-serde-runners.md | 5 +++--
 docs/research/README.ko.md                               | 1 +
 docs/research/README.md                                  | 1 +
 3 files changed, 5 insertions(+), 2 deletions(-)
```

## Command Inventory

| Package | Command | Raw output file |
|---|---|---|
| serialization | `go test -run '^$' -bench '^BenchmarkSerialization' -benchmem ./serialization` | `docs/research/outputs/issue-401/go-serialization-bench.txt` |
| codec | `go test -run '^$' -bench '^BenchmarkCodec' -benchmem ./codec` | `docs/research/outputs/issue-401/go-codec-bench.txt` |
| compression | `go test -run '^$' -bench '^BenchmarkCompressors' -benchmem ./compression` | `docs/research/outputs/issue-401/go-compression-bench.txt` |

## Output Inventory

| File | Lines | Bytes | SHA-256 |
|---|---:|---:|---|
| `go-serialization-bench.txt` | 48 | 9572 | `8b615aeaff7bf1193f066eafd70280466e30d41ec1ee4156254b0dbd8c4e5891` |
| `go-codec-bench.txt` | 62 | 9579 | `fa28ba0fdc301985db6da8041004d76bcff4178af3366390a5e770d35da6704e` |
| `go-compression-bench.txt` | 150 | 28551 | `099ed6ca261347a44cfcb18ce8f87e2d4ce9374ad6cdf426a3fd420e9e44b3f6` |

## Fixture Versions

| Fixture | Scope |
|---|---|
| `serde-small-object-v1` | serialization encode/decode/round-trip fixture |
| `serde-medium-nested-v1` | serialization nested object fixture |
| `serde-binary-payload-v1` | serialization binary payload fixture |
| `serde-repeated-collection-v1` | serialization repeated collection fixture |
| `serde-versioned-envelope-v1` | serialization versioned envelope fixture |
| `serde-text-base58-v1` | codec Base58 text fixture |
| `serde-text-base62-v1` | codec Base62 text fixture |
| `serde-url-token-v1` | codec URL62 token fixture |
| `serde-uuid-url62-v1` | codec UUID URL62 fixture |

## Metric Direction

| Metric | Direction |
|---|---|
| `ns/op` | Lower is better for the same benchmark row and host. |
| `B/op` | Lower is better for allocation volume. |
| `allocs/op` | Lower is better for allocation count. |
| `MB/s` | Higher is better for same fixture class. |
| `encoded_bytes` | Lower is denser; not a standalone performance winner. |
| `serialized_bytes` | Lower is denser before compression. |
| `compressed_bytes` | Lower is denser after compression. |
| `compressed/original` | Lower is denser against original payload. |
| `compressed/serialized` | Lower is denser against serialized payload. |

## Interpretation Boundary

These files are local benchmark snapshots. Use them as traceable evidence for #402, not as production rankings or default-selection authority by themselves.
