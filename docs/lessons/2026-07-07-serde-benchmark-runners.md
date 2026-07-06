# SerDe Benchmark Runner Scope

Issue #400 turns the #399 fixture matrix into runnable Go benchmark entry
points without changing production APIs.

## Lessons

- Keep benchmark fixtures in `_test.go` when the package does not own reusable
  fixture APIs. Cross-repo fixture names are a documentation contract, not a
  reason to export production symbols.
- Document codec exclusions beside the command. Large Base58/Base62 byte-array
  rows would measure the current division-based alphabet implementation more
  than a realistic SerDe transport path.
- Make artifact-producing commands explicit even before the artifact format is
  finalized. `tee docs/research/outputs/issue-400/...` gives #401 a concrete
  retention target without settling metadata too early.

## Evidence

- `serialization/serialization_benchmark_test.go`
- `codec/codec_benchmark_test.go`
- `compression/compression_benchmark_test.go`
- `docs/benchmarks/2026-07-07-issue-400-go-serde-runners.md`
