# Issue #207 Wildcard and Hash Utilities Step 6-R Code Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #207
게이트: Step 6-R, implemented diff 7-Tier review
날짜: 2026-06-22
Worktree: `issue-207-wildcard-hash-utils`
기준: `origin/develop` at `0ea2bfc`

## 검토 범위

- `core/wildcard.go`
- `core/wildcard_test.go`
- `core/hash.go`
- `core/hash_test.go`
- `core/doc.go`
- `core/README.md`
- `core/README.ko.md`
- `go.mod`
- #207 spec/plan/review artifacts under `docs/superpowers`

## 증거

| Check | Evidence | Status |
|---|---|---|
| TDD RED wildcard | `go test -count=1 ./core` failed with undefined `MatchWildcard`, `FirstWildcardMatch`, `MatchWildcardPath`, `FirstWildcardPathMatch`, and `ErrMalformedWildcardPattern`. | PASS |
| TDD GREEN wildcard | `go test -count=1 ./core` passed after `core/wildcard.go`. | PASS |
| TDD RED hash | `go test -count=1 ./core` failed with undefined `XXH64Bytes` and `XXH64String`. | PASS |
| TDD GREEN hash | `go mod tidy && go test -count=1 ./core` passed after `core/hash.go`. | PASS |
| Targeted race | `go test -race -count=1 ./core` passed after final path-escape adjustment. | PASS |
| Full tests | `go test ./...` passed after final path-escape adjustment. | PASS |
| Quality gates | `make fmt-check`, `make tidy-check`, `make vet`, and `make lint` passed; lint printed `0 issues.` | PASS |
| CI gate | `make ci` exited 0 after lint, normal tests, and race tests, including Testcontainers packages. | PASS |
| Whitespace gate | `git diff --check` passed after implementation and docs updates. | PASS |
| Production quick scan | `rg -n "context\\.TODO\\(|context\\.Background\\(|go func|time\\.Tick\\(|http\\.ListenAndServe\\(|panic\\(|RealIP|X-Forwarded-For" core` returned no hits. | PASS |
| Native subagent state | Prior stale-agent cleanup attempts hung until user interruption; further native lane spawning was avoided per user-visible fallback decision. | UNAVAILABLE; main-session 7-tier fallback performed. |

## 6개 검토 관점

| Lane | P0 | P1 | P2 | P3 | Verdict | Evidence |
|---|---:|---:|---:|---:|---|---|
| Performance | 0 | 0 | 0 | 0 | PASS | Wildcard matching uses dynamic programming over runes/segments rather than recursive backtracking (`core/wildcard.go:107-130`, `core/wildcard.go:178-203`). No goroutines, locks, network, filesystem, or Testcontainers code was added. |
| Stability | 0 | 0 | 0 | 0 | PASS | Malformed trailing escapes are explicit for string patterns (`core/wildcard.go:83-104`, `core/wildcard_test.go:48-54`); path matching is lexical and does not clean/read the filesystem (`core/wildcard.go:54-64`). |
| Security | 0 | 0 | 0 | 0 | PASS | XXH64 helpers are explicitly named and documented as non-cryptographic (`core/hash.go:5-18`, `core/README.md` behavior section). No auth, secret, SQL, deserialization, or external-input trust boundary is introduced. |
| Operator/Ops | 0 | 0 | 0 | 0 | PASS | No runtime service, config, shutdown, logging, metrics, or migration behavior changed. Dependency promotion is narrow and verified through `go mod tidy`, `make tidy-check`, and `make ci`. |
| Developer/API | 0 | 0 | 0 | 0 | PASS | Public API is small and localized to `core`; wildcard functions return errors instead of panicking; path and string helpers have separate contracts; `xxhash` is a direct dependency only because it is imported by production code. |
| User/Caller | 0 | 0 | 0 | 0 | PASS | README and Korean README document syntax, lexical path behavior, escaped literals, deterministic XXH64, non-crypto warning, and excluded JVM/resource/system/generic helpers. |

## 발견 사항 수렴

| Iteration | Finding | Action | Result |
|---|---|---|---|
| 1 | P2: Path pattern backslash behavior could make wildcard segments and escaped literals ambiguous. | Kept `/` and `\` separators, added escaped `*`, `?`, and `\` support inside slash-separated pattern segments, added tests, and updated docs/spec wording. | Targeted `go test -count=1 ./core`, `go test -race -count=1 ./core`, `go test ./...`, and `git diff --check` passed. |

No P0/P1 findings were found. The P2 ambiguity was fixed inside scope before PR preparation.

## 판정

P0=0 P1=0

Step 6-R verdict: PASS.
