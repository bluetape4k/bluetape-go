# Compression Benchmark Matrix

Issue #195 expanded the existing compression benchmark from two payloads to the
same-condition JSON/Text/Binary/Random matrix used by sibling ecosystem work.

## Lessons

- Use a stable slice for benchmark payloads. Map iteration makes benchmark
  output order nondeterministic, which weakens cross-run comparison.
- Report custom benchmark metrics after `b.ResetTimer()` when needed. Metrics
  reported before reset can disappear from final benchmark rows.
- For decompression benchmarks, validate a pre-timer full byte-equality
  round-trip, not only length, so corrupted same-size output cannot produce
  trustworthy-looking benchmark numbers.
- When raw benchmark output is captured from an uncommitted PR diff, record the
  dirty tree state and diff stat next to the environment metadata.
- If `golangci-lint` reports files from a removed sibling worktree, run
  `golangci-lint cache clean` and rerun the exact CI gate before treating the
  failure as code-related.

## Evidence

- `compression/compression_benchmark_test.go`
- `docs/research/2026-06-12-issue-195-compression-benchmark-matrix.md`
- `docs/research/outputs/issue-195/go-compression-bench.txt`
- `docs/review/2026-06-12-issue-195-compression-benchmark-matrix-review.md`
