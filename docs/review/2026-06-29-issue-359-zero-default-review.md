# Issue #359 Zero Default Helper Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

이슈: #359
브랜치: `feat/issue-359-zero-default`
날짜: 2026-06-29

## 범위

Small public API addition in `core`: `IfZeroOrDefault[T comparable]`.

## 7-Tier 로컬 검토

| Lane | P0 | P1 | Verdict | Evidence |
|---|---:|---:|---|---|
| Performance | 0 | 0 | PASS | Helper delegates to `IsZero` and adds no allocation or shared state. |
| Stability | 0 | 0 | PASS | Comparable-only generic contract matches existing zero helpers. |
| Security | 0 | 0 | PASS | No input parsing, IO, crypto, auth, or secret handling. |
| Operator/Ops | 0 | 0 | PASS | No runtime configuration, logging, or deployment surface. |
| Developer/API | 0 | 0 | PASS | Exported helper has a Go doc comment and README locale set was updated. |
| User/Caller | 0 | 0 | PASS | Tests cover zero and non-zero fallback behavior. |
| Integration | 0 | 0 | PASS | Targeted `core` tests and race test pass. |

## 검증

- `git diff --check`: PASS
- `go test -count=1 ./core`: PASS
- `go test -race -count=1 ./core`: PASS
- `go test -count=1 ./core -run TestZeroHelpers -v`: PASS

P0=0 P1=0
