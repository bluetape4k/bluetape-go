# Issue #359 Core Helper Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-07-03

Scope:

- `core/string.go`
- `core/uuid.go`
- `core/string_test.go`
- `core/uuid_test.go`
- `core/core_example_test.go`
- `core/README.md`
- `core/README.ko.md`

## 증거

- Source scope is constrained by #359 and the #354 parity matrix.
- Kotlin source references reviewed:
  - `support/StringSupport.kt`
  - `support/RequireSupport.kt`
  - `support/AssertSupport.kt`
  - `support/UuidSupport.kt`
- Current Go package boundaries reviewed:
  - existing `core` helpers
  - existing `id` UUID parsing/generation behavior
- Downstream scan found repeated `strings.TrimSpace(...) == ""` checks, but
  package rewrites were left out of this PR to avoid expanding the blast radius.

## 7-Tier 관점

| Lane | Verdict | Notes |
|---|---|---|
| Performance | Pass | Helpers are small string/regexp operations. `CommonPrefix` and `CommonSuffix` allocate rune slices only for explicit text-affix work; no hot-path replacement was made. |
| Stability | Pass | UUID helpers validate hyphenated text and return errors wrapping `core.ErrInvalidArgument`; no panic-based contracts were introduced. |
| Security | Pass | UUID helpers validate text shape only and do not create auth/security boundaries. `Mask` is presentation redaction only and is not documented as secret handling. |
| Operator/Ops | Pass | No runtime configuration, logging, or external service behavior changed. |
| Developer/API | Pass | Public APIs are narrow, documented, and avoid exposing `github.com/google/uuid.UUID` from `core`. Broad Kotlin extension parity and UUID numeric conversions remain non-goals. |
| User/Caller | Pass | README pairs and executable examples document nil/empty/blank distinctions, rune-aware helpers, and UUID canonicalization. |
| Integration | Pass | `go test -count=1 ./id ./core` passed, and downstream replacements were deferred to keep current package behavior stable. |

## 검증

- `git diff --check`: PASS
- `go test -count=1 ./core`: PASS
- `go test -race -count=1 ./core`: PASS
- `go test -count=1 ./id ./core`: PASS
- `make fmt-check`: PASS
- `make tidy-check`: PASS
- `make vet`: PASS
- `make lint`: PASS
- `make test`: PASS
- `make race`: PASS

## 발견 사항

- P0: 0
- P1: 0

## 잔여 위험

The new helpers are intentionally not applied across downstream packages in
this PR. Follow-up replacements should happen only where the new `core`
contract reduces a concrete local validation surface.
