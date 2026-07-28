# Issue #421 Rules Benchmark Evidence 교훈

## Lesson

Benchmark follow-up issue에서는 public Make target이 빠른 one-shot smoke command가
아니라 보존된 evidence command를 재현해야 한다. Issue #421의 최종
`bench-rules` target은 phony이고 `make help`에 표시되며,
`docs/research/outputs/issue-421/rules-benchmark.txt`에 보존된 동일한 `-count=5`
command를 실행한다.

## What Changed

- composite activation, unit, conditional, bounded inference, sequential engine
  path에 대한 focused rules benchmark를 추가했다.
- bounded inference는 한 row 안에서 workload를 섞지 않고 안정적인 `Count0`과
  `Count1` sub-benchmark로 나눴다.
- raw benchmark output과 sanitized environment metadata를
  `docs/research/outputs/issue-421/` 아래 보존했다.

## Next Time

- benchmark command, raw output, metric direction, interpretation boundary를 함께
  기록한다.
- durable benchmark artifact에는 host fingerprint를 넣지 않는다.
- benchmark row는 측정한 특정 workload shape의 증거로만 취급한다. fresh-request
  workload가 중요하면 별도 row를 추가한다.
