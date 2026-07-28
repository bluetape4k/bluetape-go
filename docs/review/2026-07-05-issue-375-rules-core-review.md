# Issue #375 Rules Core Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-07-05

Scope:

- `rules/`
- `README.md`
- `README.ko.md`

## 증거

- Issue #375 requires a dependency-free first-party `rules` package with
  `Facts`, `Rule`, functional builders, deterministic `RuleSet`, sequential
  engine config, typed errors, context cancellation, README pairs, unit tests,
  and `testing/concurrency` stress coverage.
- Issue #37 research rejects JVM-shaped DSLs, annotations, script engines, and
  expression-reader dependencies for this first slice.
- The implementation uses no new dependencies and keeps rule execution
  sequential and caller-owned through `context.Context` and `*Facts`.

## 7-Tier 관점

| Lane | Verdict | Notes |
|---|---|---|
| Performance | PASS | P0=0 P1=0. Allocation/sort benchmarks are a future adoption follow-up, not a correctness blocker. |
| Stability | PASS | P0=0 P1=0. `RuleSet` snapshots registration metadata, releases its lock before sorting, and uses cached order metadata during engine runs. |
| Security | PASS | P0=0 P1=0. Failure aggregation returns an error for any failed rule, and docs state the trusted Go-code boundary. |
| Operator/Ops | PASS | P0=0 P1=0. Cancellation sets `Result.Stopped` and `StopReason=StatusCancelled`, including rule-returned context errors. |
| Developer/API | PASS | P0=0 P1=0. `RuleError` is zero-value safe, nil receiver errors are typed, and `StopReason` uses `DetailStatus`. |
| User/Caller | PASS | P0=0 P1=0. README examples are pasteable, guard fact types, use `StopOnFirstFailed`, and check `Result.Failed`. |
| Integration | PASS | P0=0 P1=0. Root README pairs, package README pairs, tests, and issue #375 acceptance criteria align. |

## 검증

- `git diff --check`: PASS
- `go test -count=1 ./rules`: PASS
- `go test -race -count=1 ./rules`: PASS
- `go vet ./rules`: PASS
- `make fmt-check && make tidy-check && make vet && make lint`: PASS
- Full local gate
  `make fmt-check && make tidy-check && make vet && make lint && make test && make race`: PASS

## 발견 사항

- P0: 0
- P1: 0
- P2 addressed before PR: failure aggregation no longer returns nil after rule
  failures, context cancellations use cancellation stop reasons, README examples
  are guarded/pasteable, and rule ordering no longer calls user metadata methods
  while holding the `RuleSet` lock.
