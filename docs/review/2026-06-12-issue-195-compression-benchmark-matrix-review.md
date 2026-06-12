# Issue #195 Compression Benchmark Matrix Review

Issue: #195
Branch: `issue/195-compression-benchmark-matrix`
Date: 2026-06-12
Scope: `compression/compression_benchmark_test.go`, research note, index entries,
and raw benchmark evidence under `docs/research/outputs/issue-195/`.

## Verdict

PASS.

P0=0 P1=0

## Review Lanes

| Lane | Result | Evidence |
|---|---|---|
| Verifier | PASS | Initial P1 found missing decompression custom metrics. Re-review confirmed all 72 decompression rows include `compressed/original` and `compressed_bytes`. |
| Code reviewer | PASS | Initial P2/P3 suggested byte-equality setup validation and dirty-tree evidence. Re-review confirmed both were fixed with no remaining P0/P1/P2/P3 findings. |

## Findings Resolved

| Severity | Finding | Resolution |
|---|---|---|
| P1 | Decompression benchmark rows did not emit `compressed_bytes` or `compressed/original` because metrics were reported before `b.ResetTimer()`. | Moved `reportCompressionMetrics` after the timed decompression loop and `b.StopTimer()`. Regenerated full raw benchmark output. |
| P2 | Decompression setup checked only output length, not byte equality. | Added pre-timer `bytes.Equal` round-trip validation for each payload/compressor pair. |
| P3 | Environment evidence recorded only branch and base commit while the benchmark source was an uncommitted PR diff. | Added dirty tree state and diff stat to `environment.txt`, and documented that boundary in the research note. |

## Validation Evidence

| Command | Result |
|---|---|
| `go test -count=1 ./compression` | PASS |
| `go test -run '^$' -bench '^BenchmarkCompressors' -benchmem ./compression` | PASS, raw output stored at `docs/research/outputs/issue-195/go-compression-bench.txt` |
| `git diff --check` | PASS |
| `make ci` | PASS after clearing stale `golangci-lint` cache that referenced a removed sibling worktree |

## Acceptance Coverage

| #195 requirement | Status |
|---|---|
| JSON/Text/Binary/Random payload kinds | PASS |
| small/medium/large sizes | PASS |
| deterministic payload generation | PASS |
| all `compression.All()` algorithms | PASS |
| compression and decompression measured separately | PASS |
| ns/op, throughput, allocations, compressed bytes, and compressed/original ratio | PASS |
| raw output path, environment, and caveats recorded | PASS |
| benchmarks remain opt-in and outside `make ci` | PASS |

## Residual Risk

The benchmark is one local snapshot on macOS arm64 / Apple M4 Pro. It supports
same-condition comparison, but not universal production ranking.
