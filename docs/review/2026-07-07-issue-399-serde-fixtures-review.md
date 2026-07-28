# Issue #399 SerDe Fixture Matrix Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

이슈: #399
브랜치: `issue-399-serde-benchmark-fixtures`
Review date: 2026-07-07
Scope:

- `docs/benchmarks/2026-07-07-issue-399-serde-fixtures.md`
- `docs/lessons/2026-07-07-serde-benchmark-fixtures.md`

## 수용 기준 검토

| Criterion | Evidence | Verdict |
|---|---|---|
| Fixture definitions are checked into the benchmark/research documentation path chosen for 0.14.0. | `docs/benchmarks/2026-07-07-issue-399-serde-fixtures.md` defines six versioned fixture IDs under `docs/benchmarks`. | PASS |
| Every scenario has metric direction and required raw output format. | The document defines metric direction, Go/Rust/JVM raw output requirements, and a scenario table with required metrics and raw output format per scenario. | PASS |
| Explicit exclusions are documented. | The `Exclusions` section rejects production rankings, symmetric-adapter churn, API churn before evidence, lossy conversion, unlabeled strict/lenient comparisons, and compressed-size-only recommendations. | PASS |
| Compatibility notes cover Go/Rust/JVM field naming, numeric precision, time values, and optional fields. | The `Fixture Rules` and `Compatibility Notes` sections define snake_case manifests, decimal text/minor units, UTC RFC3339 timestamps, and absent-vs-null handling. | PASS |

## P0/P1 발견 사항

P0=0 P1=0

No blocker findings. This is documentation-only benchmark planning evidence;
it does not change production code, public APIs, benchmark runners, or release
state.

## 검증

- `git diff --check`: PASS
- `rg -n "serde-small-object-v1|Scenario Matrix|Metric direction|Raw output requirement|Exclusions|#400|#401|#402|#403" docs/benchmarks/2026-07-07-issue-399-serde-fixtures.md`: PASS
- `rg -n "docs/benchmarks/2026-07-07-issue-399-serde-fixtures.md|compression_benchmark_test.go|serialization/README.md|codec/README.md" docs/lessons/2026-07-07-serde-benchmark-fixtures.md`: PASS

## 잔여 위험

- The document intentionally defers runnable Go benchmark changes to #400 and
  raw evidence preservation to #401.
- Rust/JVM candidates are mapped from current local repository surfaces; later
  runner PRs must re-check each sibling repository before producing raw output.
