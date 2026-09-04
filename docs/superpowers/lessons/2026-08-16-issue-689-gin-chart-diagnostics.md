# Issue #689 Gin chart watchdog와 실패 진단

## 배경

Gin adapter benchmark capture는 benchmark subprocess의 output bound를 이미
검사했지만, 후속 chart renderer는 직접 foreground 실행했다. renderer가 멈추거나
비정상 종료하면 capture가 종료되지 않거나 원인이 `chart generation failed` 한
줄로만 남는 운영 공백이 있었다.

## 결정

- chart renderer를 직접 child process로 실행하고 60초 기본 watchdog을 둔다.
- `BLUETAPE_GIN_BENCH_CHART_TIMEOUT_SECONDS`는 1~600초, chart output bound는
  `BLUETAPE_GIN_BENCH_CHART_MAX_OUTPUT_BYTES`로 1~10 MiB 범위에서 설정한다.
- chart child 종료 전 direct child를 함께 정리하고, timeout은 status `124`,
  signal은 `128+signal`, 일반 non-zero는 원래 status를 진단 artifact에 기록한다.
- output-limit에서는 허용 크기까지만 log를 보존하고 marker를 붙인다.
- benchmark와 동일한 redaction 함수를 chart stderr에도 적용한다. redaction
  검증을 통과한 경우에만 `chart_stderr_begin/end`를 publish한다.
- chart 실패는 capture exit `125`로 유지하고, 기존 canonical 결과·chart 파일은
  atomic publication 경계를 넘지 않게 보존한다.

## 검증

```bash
shellcheck scripts/capture-gin-adapter-benchmark.sh \
  scripts/capture-gin-adapter-benchmark_test.sh
bash -n scripts/capture-gin-adapter-benchmark.sh \
  scripts/capture-gin-adapter-benchmark_test.sh
scripts/capture-gin-adapter-benchmark_test.sh
make check-bench-web-gin
```

fixture contract는 chart exit `42`, timeout `124`, signal `143`, output-limit
`125`, malformed input exit를 각각 확인하고, failure phase/reason/status,
stderr, marker, canonical 파일 미생성을 함께 검증한다.

## 후속 주의

chart renderer가 새 외부 child를 추가하면 direct child 정리와 output redaction
contract를 먼저 확장해야 한다. benchmark 회귀 비교기의 `inconclusive` 의미를
바꾸지 말고, renderer 실패는 계속 별도 capture failure로 유지한다.
