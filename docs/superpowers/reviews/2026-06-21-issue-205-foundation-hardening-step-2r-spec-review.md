# Issue #205 Foundation Contract Hardening Step 2-R Spec Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #205
Spec: `docs/superpowers/specs/2026-06-21-issue-205-foundation-hardening-design.md`
게이트: Step 2-R, 7-Tier spec review
날짜: 2026-06-21
Worktree: `issue-205-foundation-hardening`
기준: `origin/develop` at `b4b91a8`

## 검토 범위

- `docs/superpowers/specs/2026-06-21-issue-205-foundation-hardening-design.md`
- GitHub issue #205 live metadata
- Current package contracts in `core`, `collections`, `codec`, and `serialization`
- English/Korean package README behavior notes for touched packages

## 증거

| Check | Evidence | Status |
|---|---|---|
| Live issue scope | #205 is open, assigned to `debop`, milestone `0.6.3`, labels include `priority: p0`, `area: core`, and `area: serialization`. | PASS |
| Baseline CI | `make ci` passed before spec work on this worktree. | PASS |
| Baseline targeted tests | `go test -count=1 ./core ./collections ./codec ./serialization` passed before implementation. | PASS |
| Placeholder scan | `rg -n "TBD|TODO|placeholder|fill in|later"` returned only the intentional Step DoD wording. | PASS |
| Whitespace gate | `git diff --check` passed after spec edits. | PASS |

## 6개 검토 관점

| Lane | P0 | P1 | P2 | P3 | Verdict | Evidence |
|---|---:|---:|---:|---:|---|---|
| Performance | 0 | 0 | 0 | 0 | PASS | Spec rejects broad rewrites, new dependencies, and new Testcontainers scope; verification stays targeted plus existing `make ci`. |
| Stability | 0 | 0 | 0 | 0 | PASS | Spec covers nil, empty, malformed input, boundaries, invalid UTF-8, deterministic text/binary behavior, and regression-test shape. |
| Security | 0 | 0 | 0 | 0 | PASS after rerun | Initial P1 found missing invalid UTF-8 coverage for `core.TruncateUTF8Bytes`; spec now requires invalid UTF-8 rejection for codec string decoders, `StringSerializer`, and `TruncateUTF8Bytes`, while byte APIs remain binary. |
| Operator/Ops | 0 | 0 | 0 | 0 | PASS | Spec introduces no Docker or infrastructure expansion and keeps final evidence to targeted tests, `make ci`, `git diff --check`, and GitHub CI status. |
| Developer/API | 0 | 0 | 0 | 0 | PASS | Spec keeps public API narrow, excludes new primitives/dependencies, preserves byte APIs, and makes invalid UTF-8 rejection an intentional text-helper tightening. |
| User/Caller | 0 | 0 | 0 | 0 | PASS after rerun | Initial P1s found missing migration guidance and caller-detectable error contract; spec now requires English/Korean README notes, binary alternatives, and an `errors.Is`-detectable invalid UTF-8 sentinel. |

## 메인 통합 검토

The spec now satisfies #205 and the #204 sequencing constraint:

- It hardens existing `core`, `collections`, `codec`, and `serialization`
  contracts before #206-#208 add new parity primitives.
- It avoids new public parity APIs, new dependencies, broad rewrites, and
  Testcontainers expansion.
- It defines the text/binary boundary clearly: string helpers reject invalid
  UTF-8 with a caller-detectable sentinel; byte helpers remain binary.
- It requires README updates in both English and Korean where package behavior
  changes or needs clarification.
- It preserves TDD evidence for real behavior changes and regression tests for
  already-implemented edge contracts.

## 발견 사항 수렴

| Iteration | Finding | Action | Result |
|---|---|---|---|
| 1 | P1: `core.TruncateUTF8Bytes` invalid UTF-8 handling omitted. | Added explicit invalid UTF-8 rejection contract and RED test requirement for `TruncateUTF8Bytes`. | Security rerun passed with P0=0 P1=0. |
| 1 | P1: invalid UTF-8 tightening lacked migration guidance. | Added English/Korean README migration-note requirement with byte helper and `BytesSerializer` alternatives. | User/Caller rerun passed with P0=0 P1=0. |
| 1 | P1: invalid UTF-8 errors lacked caller-detectable contract. | Added exported sentinel requirement usable with `errors.Is`. | User/Caller rerun passed with P0=0 P1=0. |

## 판정

P0=0 P1=0

Step 2-R verdict: PASS.
