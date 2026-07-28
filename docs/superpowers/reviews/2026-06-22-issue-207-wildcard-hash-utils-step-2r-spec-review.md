# Issue #207 Wildcard and Hash Utilities Step 2-R Spec Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #207
Spec: `docs/superpowers/specs/2026-06-22-issue-207-wildcard-hash-utils-design.md`
게이트: Step 2-R, 7-Tier spec review
날짜: 2026-06-22
Worktree: `issue-207-wildcard-hash-utils`
기준: `origin/develop` at `0ea2bfc`

## 검토 범위

- GitHub issue #207 objective, acceptance criteria, and parent epic #204
- `docs/research/2026-06-21-issue-202-source-parity-matrix.md`
- `docs/research/2026-06-01-issue-8-core-support-inventory.md`
- Kotlin source references: `Wildcard.kt`, `XXHasher.kt`, `Resourcex.kt`,
  `Systemx.kt`, `ShutdownQueue.kt`, `StringSupport.kt`, `ByteArraySupport.kt`
- Current Go package shape in `core` and `probabilistic`
- Dependency evidence for `github.com/cespare/xxhash/v2`

## 증거

| Check | Evidence | Status |
|---|---|---|
| Live issue scope | #207 is assigned to `debop`, milestone `0.6.3`, labels include `type: task`, `priority: p1`, `area: utilities`, and `area: core`; parent epic is #204. | PASS |
| Baseline worktree | Worktree branch `issue-207-wildcard-hash-utils` is based on `origin/develop` at `0ea2bfc`; baseline `go test ./...` passed before spec work. | PASS |
| Source parity search | Kotlin wildcard/hash/resource/system files inspected; repo search found no existing Go shared wildcard/hash utility and Bloom hash remains package-local. | PASS |
| Dependency check | `go list -m -json github.com/cespare/xxhash/v2` showed v2.3.0 already indirect; GitHub metadata showed repo not archived as of 2026-06-22; module cache contains MIT license. | PASS |
| Placeholder scan | `rg -n "TBD|TODO|placeholder|fill in|FIXME|\?\?"` on spec/plan returned no hits. | PASS |
| Whitespace gate | `git diff --check` passed after moving spec/plan into the feature worktree. | PASS |
| Native subagent state | Prior stale-agent cleanup attempts hung until user interruption; further native lane spawning was explicitly avoided to keep progress moving. | UNAVAILABLE; main-session 7-tier fallback performed. |

## 6개 검토 관점

| Lane | P0 | P1 | P2 | P3 | Verdict | Evidence |
|---|---:|---:|---:|---:|---|---|
| Performance | 0 | 0 | 0 | 0 | PASS | Spec requires DP wildcard matching instead of recursive backtracking and limits hashing to raw bytes/string XXH64. |
| Stability | 0 | 0 | 0 | 0 | PASS | Spec defines malformed trailing escapes as errors, path tokenization rules, deterministic hash inputs, and excludes hidden OS/resource wrappers. |
| Security | 0 | 0 | 0 | 0 | PASS | Spec labels XXH64 as non-cryptographic and excludes token/signature/password/integrity uses from the hash contract. |
| Operator/Ops | 0 | 0 | 0 | 0 | PASS | Spec introduces no runtime service, filesystem inspection, global state, shutdown hook, or OS-dependent path lookup behavior. |
| Developer/API | 0 | 0 | 0 | 0 | PASS | API is small, pattern-first like Go matchers, error-returning for malformed patterns, and avoids generic object hashing. |
| User/Caller | 0 | 0 | 0 | 0 | PASS | README obligations cover syntax, case sensitivity, lexical path matching, portability limits, non-crypto warning, and excluded JVM helpers. |

## 메인 통합 검토

The spec satisfies #207 while staying Go-native:

- Wildcard semantics are fixed and testable: `*`, `?`, escaping, Unicode rune
  matching, path `**`, separators, and case sensitivity.
- Hashing is explicit and stable at raw bytes/string boundaries, instead of
  exposing Kotlin/JVM object hash semantics that Go cannot reproduce safely.
- Resource/system helper parity is intentionally rejected because Go's standard
  library already exposes `os`, `io/fs`, `runtime`, and cleanup ownership
  without hiding errors or creating global lifecycle state.
- The dependency choice is bounded to `github.com/cespare/xxhash/v2`, already
  present indirectly and suited to XXH64 only.

## 발견 사항 수렴

No P0/P1 findings were found. No spec edits were required after review.

## 판정

P0=0 P1=0

Step 2-R verdict: PASS.
