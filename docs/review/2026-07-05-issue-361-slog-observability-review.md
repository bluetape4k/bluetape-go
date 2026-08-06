# Issue #361 slog Observability Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-07-05

Scope:

- `README.md`
- `README.ko.md`
- `resilience/README.md`
- `resilience/README.ko.md`
- `resilience/resilience_example_test.go`
- `money/README.md`
- `money/README.ko.md`
- `workflow/README.md`
- `workflow/README.ko.md`
- `workreport/README.md`
- `workreport/README.ko.md`
- `docs/research/2026-07-05-issue-361-slog-observability.md`

## 증거

- Issue #361 requires standard-library `log/slog` examples, a package-local
  bridge for existing hook-based observability, and no global logging facade.
- `resilience.Event` already exposes stable fields such as `PolicyName`,
  `PolicyType`, `Kind`, `Category`, `Attempt`, `Delay`, and `ErrorCategory`.
- `resilience.OnEvent` handlers are synchronous and caller-owned, so `slog`
  bridge examples must keep handlers fast and avoid package-owned global
  logging state.

## 7-Tier 관점

| Lane | Verdict | Notes |
|---|---|---|
| Performance | PASS | P0=0 P1=0. Added high-volume success event filter/sample guidance after P2 review note. |
| Stability | PASS | P0=0 P1=0. Compile-checked `ExampleRetryOptions_slogOnEvent` keeps caller-owned synchronous hook contract. |
| Security | PASS | P0=0 P1=0. Replaced raw `RefreshError` logging with low-cardinality `refresh_status`. |
| Operator/Ops | PASS | P0=0 P1=0. Docs state application-owned handlers/levels and fast non-blocking synchronous hooks. |
| Developer/API | PASS | P0=0 P1=0. No logging facade, logger registry, MDC-shaped API, or global default mutation. |
| User/Caller | PASS | P0=0 P1=0. Added import/context notes and compile-checked example links for `slog` snippets. |
| Integration | PASS | P0=0 P1=0. README pairs, example test, and decision note align with issue #361. |

## 검증

- `git diff --check`: PASS
- `go test -count=1 ./resilience ./money ./workflow ./workreport`: PASS
- Public README `log.Printf`/`log.Print`/`log.Fatal` scan: PASS
- Full local gate
  `make fmt-check && make tidy-check && make vet && make lint && make test && make race`: PASS
- Security scan for raw public error logging: PASS after replacing
  `refresh_error` with `refresh_status`.

## 발견 사항

- P0: 0
- P1: 0
- P2 addressed before PR: high-volume synchronous hook logging guidance,
  beginner import/context notes, and raw provider refresh error removal.
