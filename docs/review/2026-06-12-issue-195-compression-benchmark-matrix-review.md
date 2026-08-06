# Issue #195 Compression Benchmark Matrix Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

이슈: #195
브랜치: `issue/195-compression-benchmark-matrix`
날짜: 2026-06-12
범위: `compression/compression_benchmark_test.go`, research note, index entries,
raw benchmark evidence under `docs/research/outputs/issue-195/`, and generated
chart assets under `docs/images/readme-charts/`.

## 판정

PASS.

P0=0 P1=0

## 검토 관점

| 관점 | 결과 | 증거 |
|---|---|---|
| Verifier | PASS | Initial P1 found missing decompression custom metrics. Re-review confirmed all 72 decompression rows include `compressed/original` and `compressed_bytes`. |
| Code reviewer | PASS | Initial P2/P3 suggested byte-equality setup validation and dirty-tree evidence. Re-review confirmed both were fixed with no remaining P0/P1/P2/P3 findings. |
| Chart verifier | PASS | `generate-compression-charts.mjs` parsed 48 large-payload rows, rendered SVG/PNG bar charts, and the PNG was visually inspected for nonblank panels, axes, bars, and legend. |
| Visual correction | PASS | Replaced the rejected heatmap/matrix-style output with small-multiple horizontal bar charts where bar length and axes carry the comparison signal. |

## 해결한 발견 사항

| Severity | Finding | Resolution |
|---|---|---|
| P1 | Decompression benchmark rows did not emit `compressed_bytes` or `compressed/original` because metrics were reported before `b.ResetTimer()`. | Moved `reportCompressionMetrics` after the timed decompression loop and `b.StopTimer()`. Regenerated full raw benchmark output. |
| P2 | Decompression setup checked only output length, not byte equality. | Added pre-timer `bytes.Equal` round-trip validation for each payload/compressor pair. |
| P3 | Environment evidence recorded only branch and base commit while the benchmark source was an uncommitted PR diff. | Added dirty tree state and diff stat to `environment.txt`, and documented that boundary in the research note. |

## 검증 증거

| 명령 | 결과 |
|---|---|
| `go test -count=1 ./compression` | PASS |
| `go test -run '^$' -bench '^BenchmarkCompressors' -benchmem ./compression` | PASS, raw output stored at `docs/research/outputs/issue-195/go-compression-bench.txt` |
| `node docs/images/readme-charts/generate-compression-charts.mjs` | PASS, `rows=48 panels=3 smallMultiples=12 bars=72 payloads=4 algorithms=6` |
| `rsvg-convert docs/images/readme-charts/compression-large-payload-benchmark-bars.svg -o docs/images/readme-charts/compression-large-payload-benchmark-bars.png` | PASS, PNG rendered at 1280x1760 |
| `git diff --check` | PASS |
| `make ci` | PASS after clearing stale `golangci-lint` cache that referenced a removed sibling worktree |

## 수용 기준 커버리지

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

## 잔여 위험

The benchmark is one local snapshot on macOS arm64 / Apple M4 Pro. It supports
same-condition comparison, but not universal production ranking.
