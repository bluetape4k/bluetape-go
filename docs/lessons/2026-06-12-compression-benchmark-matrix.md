# Compression Benchmark Matrix 교훈

Issue #195는 기존 compression benchmark를 두 payload에서 sibling ecosystem
작업과 같은 조건의 JSON/Text/Binary/Random matrix로 확장했다.

## 교훈

- benchmark payload에는 stable slice를 사용한다. map iteration은 benchmark
  output 순서를 nondeterministic하게 만들어 cross-run comparison을 약하게 한다.
- 필요하면 custom benchmark metric은 `b.ResetTimer()` 뒤에 보고한다. reset 전에
  보고한 metric은 최종 benchmark row에서 사라질 수 있다.
- decompression benchmark에서는 length만 보지 말고 timer 전 full byte-equality
  round-trip을 검증한다. 그래야 같은 크기의 손상된 output이 신뢰할 수 있어 보이는
  benchmark number를 만들지 못한다.
- uncommitted PR diff에서 raw benchmark output을 캡처했다면 environment metadata
  옆에 dirty tree state와 diff stat을 함께 기록한다.
- research note에 benchmark result table을 넣는다면 측정 table 옆에 실제 chart
  asset도 추가한다. reviewer는 chart를 numeric source of truth로 보지 않으면서
  throughput과 density pattern을 훑을 수 있어야 한다. reviewer에게 visual
  comparison이 필요할 때 numeric cell heatmap이나 matrix는 benchmark chart의
  대체물이 아니다. bar length, axis, 또는 다른 실제 visual encoding이 비교
  signal을 담아야 한다.
- `golangci-lint`가 제거된 sibling worktree의 file을 보고하면 그 failure를
  code-related로 보기 전에 `golangci-lint cache clean`을 실행하고 정확한 CI gate를
  다시 실행한다.

## 증거

- `compression/compression_benchmark_test.go`
- `docs/research/2026-06-12-issue-195-compression-benchmark-matrix.md`
- `docs/research/outputs/issue-195/go-compression-bench.txt`
- `docs/images/readme-charts/compression-large-payload-benchmark-bars.svg`
- `docs/images/readme-charts/compression-large-payload-benchmark-bars.png`
- `docs/review/2026-06-12-issue-195-compression-benchmark-matrix-review.md`
