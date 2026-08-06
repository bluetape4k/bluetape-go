# Issue 212 scoped test resource helpers review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-06-23
PR: #259
이슈: #212
범위: `testing` package scoped temp output, environment, stdout/stderr capture helpers, examples, and README updates.

## 판정

P0=0 P1=0

## 증거

- `go test -count=1 ./testing/...`
- `go test -race -count=1 ./testing`
- `make fmt-check tidy-check vet lint`
- `make test`
- `make race`
- GitHub CI for initial PR commit: success before review-artifact repair commit.

## 7-Tier 메모

- Performance: output capture uses a package mutex and short-lived OS pipes only around the caller function; no production hot path is touched.
- Stability: env restoration, output restoration, nil callback diagnostics, and panic restoration are covered by tests; full race gate passed.
- Security: temp output path helpers reject empty roots, absolute path parts, and parent traversal after cleaning.
- Operator/Ops: README and Go doc explicitly state process-global stdout/stderr and environment caveats.
- Developer/API: helpers use `testing.TB`, keep `t.TempDir` and `t.Setenv` as the preferred simple-case APIs, and avoid broad test framework abstractions.
- User/caller: examples cover path validation and output capture without suggesting unsafe parallel use.
- Integration: change is scoped to `testing` package docs/tests/helpers and does not alter other package behavior.

## 후속 위험

None blocking. Do not use env or stdout/stderr capture helpers from parallel tests unless the implementation no longer mutates process-global state.
