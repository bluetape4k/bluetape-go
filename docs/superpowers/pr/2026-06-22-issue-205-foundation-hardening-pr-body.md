## 요약

Closes #205

이 PR은 기존 `core`, `collections`, `codec`, `serialization` foundation
contract를 강화한다. milestone `0.6.3`에서 새로운 parity API를 추가하기
전이다.

## 배경

이슈 #205는 `0.6.3` foundation milestone의 P0 audit 작업이다. 현재 package는
이미 core helper, collections transform, codec helper, raw serializer를
제공하지만 text/binary, nil/empty, malformed input, documentation contract
여러 개가 암묵적이었다.

## 해결 내용

- 잘못된 UTF-8 text failure를 `core.ErrInvalidUTF8`을 통해 caller가 확인할 수
  있게 한다.
- byte codec helper와 `BytesSerializer`가 임의 payload에 대해 binary-safe하게
  동작하도록 유지한다.
- malformed codec input failure와 decoded invalid-text failure를 분리한다.
- 오류를 반환하지 않는 string encoder helper를 invalid UTF-8을 보고할 수 없는
  호환성 string-to-byte 변환으로 문서화한다.
- collection nil/empty 및 nil callback precedence 동작의 regression coverage를
  추가한다.

## 작업 내용

- `core`: `ErrInvalidUTF8`을 추가하고 rune-boundary truncation은 보존하면서
  `TruncateUTF8Bytes`가 잘못된 UTF-8을 거부하도록 했다.
- `codec`: string decoder가 UTF-8 validation을 거치도록 연결하고 byte
  decoder는 binary-capable 상태로 유지했다.
- `serialization`: `StringSerializer`가 marshal/unmarshal 시 UTF-8을
  validation하도록 하고 `BytesSerializer`는 binary-capable 상태로 유지했다.
- `collections`: 기존 helper 동작은 바꾸지 않고 nil/empty/callback
  regression coverage를 추가했다.
- Docs/examples: `errors.Is(err, core.ErrInvalidUTF8)` 및 byte fallback
  경로에 관한 English/Korean README와 example을 갱신했다.
- Workflow 증거: spec, plan, Step 2-R, Step 3-R, Step 5, Step 6-R,
  lesson 산출물을 추가했다.

## 검증

- `go test -count=1 ./core ./collections ./codec ./serialization`: PASS
- `go test -run Example -count=1 ./codec ./serialization`: PASS
- `go list -deps ./codec ./serialization | rg '^github.com/bluetape4k/bluetape-go/core$'`: PASS
- `go test -race -count=1 ./codec`: PASS
- `go test -race -count=1 ./serialization`: PASS
- `make ci`: PASS
- `git diff --check`: PASS

## 검토 메모

- P0/P1: 0
- P2/P3: 후속 기록 없음
- 검토 증거:
  - `docs/superpowers/reviews/2026-06-21-issue-205-foundation-hardening-step-2r-spec-review.md`
  - `docs/superpowers/reviews/2026-06-21-issue-205-foundation-hardening-step-3r-plan-review.md`
  - `docs/superpowers/reviews/2026-06-22-issue-205-foundation-hardening-step-5-verifier.md`
  - `docs/superpowers/reviews/2026-06-22-issue-205-foundation-hardening-step-6r-code-review.md`
  - `docs/superpowers/reviews/2026-06-22-issue-205-foundation-hardening-step-7r-pr-review.md`

## 메타데이터

- 이슈: #205, milestone `0.6.3`, assignee `debop`
- PR 메타데이터: #252, milestone `0.6.3`, assignee `debop`
- CI: GitHub check 대기 중

## DoD Status

| 단계 | 상태 | 증거 |
|------|--------|----------|
| Step 0 - Worktree | PASS | `.worktrees/issue-205-foundation-hardening`, PR 전에 branch가 `origin/develop`보다 두 commit 앞섬. |
| Step 1/1-R - 요구사항 및 연구 | PASS | 이슈 #205 metadata 확인; 현재 package source, test, README, 이전 문서 확인. |
| Step 2 - Spec | PASS | `docs/superpowers/specs/2026-06-21-issue-205-foundation-hardening-design.md` |
| Step 2-R - Spec review | PASS | `docs/superpowers/reviews/2026-06-21-issue-205-foundation-hardening-step-2r-spec-review.md`, P0=0 P1=0 |
| Step 3 - Plan | PASS | `docs/superpowers/plans/2026-06-21-issue-205-foundation-hardening-plan.md` |
| Step 3-R - Plan review | PASS | `docs/superpowers/reviews/2026-06-21-issue-205-foundation-hardening-step-3r-plan-review.md`, P0=0 P1=0 |
| Step 4 - TDD implementation | PASS | `core.ErrInvalidUTF8` 누락에 대한 RED failure 확인; implementation commit `7f80b73`. |
| Step 4-T - Tests | PASS | 대상 test, example, race check, dependency check, `make ci`, `git diff --check` 통과. |
| Step 5 - Verifier | PASS | `docs/superpowers/reviews/2026-06-22-issue-205-foundation-hardening-step-5-verifier.md` |
| Step 6-R - Code review | PASS | `docs/superpowers/reviews/2026-06-22-issue-205-foundation-hardening-step-6r-code-review.md`, P0=0 P1=0 |
| Step 7 - Lessons | PASS | `docs/lessons/2026-06-22-issue-205-foundation-hardening.md`, PR 생성 전에 commit. |
| Step 7-P - PR | PASS | PR #252 생성; assignee `debop`; milestone `0.6.3`; label이 이슈 #205와 일치. |
| Step 7-R - PR review | PASS | `docs/superpowers/reviews/2026-06-22-issue-205-foundation-hardening-step-7r-pr-review.md`, P0=0 P1=0. |
| Step 8 - CI | PENDING | PR 생성 후 확인. |

최종 상태: PR #252는 검토 및 CI 대기 중이다.
