# Issue #456 Benchmark Environment

Generated UTC: 2026-07-07T10:13:07Z
Generated local: 2026-07-07 19:13:07 KST

## Host

- OS/arch: darwin/arm64
- Kernel: Darwin debop.local 25.5.0 Darwin Kernel Version 25.5.0: Mon Apr 27 20:39:29 PDT 2026; root:xnu-12377.121.6~2/RELEASE_ARM64_T8142 arm64
- CPU: Apple M5
- Go: go version go1.26.4 darwin/arm64

## Package Revision

- Branch: issue-456-json-repeated-profile
- Baseline code base: 40d4734fb2ce
- Benchmark artifact issue: #456
- Benchmark runner source issue: #400
- Prior retained SerDe output issue: #401

### Dirty Tree At Capture

```text
 M serialization/json.go
?? docs/research/outputs/issue-456/
```

## Command Inventory

| Purpose | Command | Raw output file |
|---|---|---|
| Baseline repeated JSON rows | `go test -run '^$' -bench '^BenchmarkSerialization(Decode\|RoundTrip)/JSON/serde-repeated-collection-v1$' -benchmem -count=5 ./serialization` | `json-repeated-baseline-bench.txt` |
| Baseline decode profile | `go test -run '^$' -bench '^BenchmarkSerializationDecode/JSON/serde-repeated-collection-v1$' -benchmem -memprofile docs/research/outputs/issue-456/json-repeated-decode.mem.pprof ./serialization` | `json-repeated-decode-profile-bench.txt` |
| Baseline decode profile top | `go tool pprof -top -alloc_space docs/research/outputs/issue-456/json-repeated-decode.mem.pprof` | `json-repeated-decode-mem-top.txt` |
| Baseline round-trip profile | `go test -run '^$' -bench '^BenchmarkSerializationRoundTrip/JSON/serde-repeated-collection-v1$' -benchmem -memprofile docs/research/outputs/issue-456/json-repeated-roundtrip.mem.pprof ./serialization` | `json-repeated-roundtrip-profile-bench.txt` |
| Baseline round-trip profile top | `go tool pprof -top -alloc_space docs/research/outputs/issue-456/json-repeated-roundtrip.mem.pprof` | `json-repeated-roundtrip-mem-top.txt` |
| After-change repeated JSON rows | `go test -run '^$' -bench '^BenchmarkSerialization(Decode\|RoundTrip)/JSON/serde-repeated-collection-v1$' -benchmem -count=5 ./serialization` | `json-repeated-after-unmarshal-bench.txt` |
| After-change decode profile | `go test -run '^$' -bench '^BenchmarkSerializationDecode/JSON/serde-repeated-collection-v1$' -benchmem -memprofile docs/research/outputs/issue-456/json-repeated-decode-after.mem.pprof ./serialization` | `json-repeated-decode-after-profile-bench.txt` |
| After-change decode profile top | `go tool pprof -top -alloc_space docs/research/outputs/issue-456/json-repeated-decode-after.mem.pprof` | `json-repeated-decode-after-mem-top.txt` |
| After-change round-trip profile | `go test -run '^$' -bench '^BenchmarkSerializationRoundTrip/JSON/serde-repeated-collection-v1$' -benchmem -memprofile docs/research/outputs/issue-456/json-repeated-roundtrip-after.mem.pprof ./serialization` | `json-repeated-roundtrip-after-profile-bench.txt` |
| After-change round-trip profile top | `go tool pprof -top -alloc_space docs/research/outputs/issue-456/json-repeated-roundtrip-after.mem.pprof` | `json-repeated-roundtrip-after-mem-top.txt` |

## Output Inventory

| File | Lines | Bytes | SHA-256 |
|---|---:|---:|---|
| `json-repeated-baseline-bench.txt` | 16 | 1880 | `da470f39b8bb9f5957f6410161229949f16b8dc3b05bd086716d77acfafffc99` |
| `json-repeated-after-unmarshal-bench.txt` | 16 | 1880 | `6fdbc94d00fcd49fa3cd9a0a91d3315b4d4f6c9f7651eb02f11547ab4b71c6bd` |
| `json-repeated-decode-profile-bench.txt` | 7 | 331 | `d7f26f0b7e4419955c2c10ed0ed701ee161842e80341dc1beffb987abfcfa0eb` |
| `json-repeated-decode-mem-top.txt` | 30 | 2174 | `8fbceb8355aac2216c211003e0a2e1176bf4262fa99ec2117ac552ad13d922d7` |
| `json-repeated-decode.mem.pprof` | 14 | 4024 | `faedd67a52de3ac1057fcbe6c6339b5b7075bc51c350de6caac26bfd3d95d63b` |
| `json-repeated-decode-after-profile-bench.txt` | 7 | 331 | `3368bf134f173ac9fd130a2586d9430cef359bfb57938180f36c22a7cb6ad1e1` |
| `json-repeated-decode-after-mem-top.txt` | 29 | 2135 | `257f0e5d1152bdbac7c0ada22d0b0a12e3cadf009035e515d2cdc201185060dd` |
| `json-repeated-decode-after.mem.pprof` | 13 | 3555 | `a5cdab398e4186aa6edc79695d4bfba3289a1ad123295b6c7d9aab136181f4ad` |
| `json-repeated-roundtrip-profile-bench.txt` | 7 | 334 | `fbfe606cbddc40d8c97018f78727d37a21c442c20875c26b548419b2084e8860` |
| `json-repeated-roundtrip-mem-top.txt` | 45 | 3412 | `d58db4bb16b2c842f5007a1192e3a647b46f3986c3bd9a2ce23d0b2b4b36c9a2` |
| `json-repeated-roundtrip.mem.pprof` | 16 | 4448 | `adef08d5f96993ad6824fa70b741689f1bb23f99994ceb29e1ce93d1e5dca9e2` |
| `json-repeated-roundtrip-after-profile-bench.txt` | 7 | 334 | `8d5db5cd4596b3a629208841a59b6ed14cb69f4ca4be2cf739d6621632c55a9d` |
| `json-repeated-roundtrip-after-mem-top.txt` | 43 | 3248 | `a1e5024eebef6645eecafacfc5bf6d91375bae7b3a765902692c759731973c88` |
| `json-repeated-roundtrip-after.mem.pprof` | 10 | 4860 | `23c06647543b61ad1fa9eef18770c62a946922a6a6bc5a960fb2c85ea0283c5e` |

## Metric Direction

| Metric | Direction |
|---|---|
| `ns/op` | Lower is better for the same benchmark row and host. |
| `B/op` | Lower is better for allocation volume. |
| `allocs/op` | Lower is better for allocation count. |
| `MB/s` | Higher is better for the same fixture class. |
| `encoded_bytes` | Fixture size evidence, not a standalone performance winner. |

## Interpretation Boundary

This profile compares one local baseline against one narrow implementation
change on the same host and fixture. It supports the default decode path change;
it does not justify pooling decoded values, unsafe reuse, or changing
`WithDisallowUnknownFields`.
