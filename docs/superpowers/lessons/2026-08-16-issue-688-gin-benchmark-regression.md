# Issue #688 Gin benchmark 회귀 게이트 교훈

## 결정

- parser-only `FullAdapter`와 no-backoff `MaxAttempts=2` retry policy를 적용한
  `FullAdapterRetry`를 같은 `BenchmarkGinAdapter` matrix에서 별도 행으로 측정한다.
- baseline 비교는 CPU 1 serial의 두 FullAdapter 행만 gate로 사용한다. `ns/op`는
  15%, `B/op`와 `allocs/op`는 10% 초과 증가를 회귀로 판정한다.
- baseline과 candidate가 fixture identity, Go/Gin 버전, OS/arch, CPU matrix 및
  clean provenance를 모두 공유하지 않으면 `inconclusive`와
  `no_regression: N/A`를 반환한다. 이기종 호스트의 숫자를 회귀 없음으로
  합치지 않는다.

## 재현 명령

```bash
BENCH_COUNT=5 BENCH_CPU=1,2,4 make bench-web-gin
python3 scripts/compare-gin-adapter-benchmark.py \
  --baseline docs/research/outputs/issue-543/bench-baseline.json \
  --candidate docs/research/outputs/issue-543/bench-results.json \
  --output docs/research/outputs/issue-543/bench-regression.json
```

`make bench-web-gin-regression`은 위 capture와 비교를 하나의 opt-in 명령으로
묶는다. CI 기본 경로는 실제 benchmark 시간을 소비하지 않고
`make check-bench-web-gin`의 parser/comparator/chart contract만 실행한다.

## 증거

- clean capture SHA: `3e14c7e0ad027c7d0187e7a0524f0a5db976e126`
- 결과: 14개 benchmark 행 × CPU 3개 × sample 5개 = 210행
- 비교 report: `status: passed`, `no_regression: passed`
- comparator contract: threshold pass/fail, dirty candidate, environment mismatch,
  missing baseline 경로를 각각 검증

## 후속 경계

chart renderer timeout/watchdog, stderr 보존, 실패 artifact 진단은 Issue #689에서
별도로 처리한다. benchmark 회귀 gate와 chart 운영 진단을 한 번에 섞지 않는다.
