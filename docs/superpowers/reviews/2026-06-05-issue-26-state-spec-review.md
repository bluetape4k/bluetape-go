# Issue #26 State Spec Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #26
Milestone: 0.4.0
날짜: 2026-06-05
게이트: Step 2-R
Reviewed spec: `docs/superpowers/specs/2026-06-05-issue-26-state-spec.md`
Research: `docs/superpowers/research/2026-06-05-issue-26-state-inventory.md`

## Gate Recovery Note

The draft plan at
`docs/superpowers/plans/2026-06-05-issue-26-state-plan.md` existed before this
gate, but it is treated as provisional and not gate-approved. This Step 2-R
review covers only the research and spec. Step 3 plan work must be rewritten or
reconfirmed from the reviewed spec after this gate passes.

Required reference loaded:
`/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-2r-spec-review.md`.

Native subagents were not used because the current session tool contract allows
spawning only when the user explicitly requests sub-agent or parallel-agent
work. The gate therefore uses the skill's allowed local-equivalent independent
lanes.

## 범위

- Public `state` package API for states, events, transitions, guards, results,
  and errors.
- Concurrency behavior for transition attempts.
- Guard execution and context cancellation semantics.
- Package docs, README, examples, stress tests, and race validation.
- Evidence against #26, #135, #136, #137, current repository patterns, and
  Kotlin `utils/states` reference behavior.

Out of scope:

- Implementation code.
- The provisional Step 3 plan draft.
- Later `workreport` and `workflow` packages.

## Perspective Reviews

| Perspective | P0 | P1 | P2 | P3 | Findings |
|---|---:|---:|---:|---:|---|
| Developer/API | 0 | 1 | 0 | 0 | `CanTransition` guard execution was underspecified for side-effecting guards. |
| Security | 0 | 0 | 0 | 0 | No auth, secrets, injection, deserialization, or external input boundary is introduced. |
| Ops/SRE | 0 | 1 | 0 | 0 | Context cancellation around guard execution needed an explicit before-commit check and guard responsibility statement. |
| User/caller | 0 | 1 | 0 | 0 | `AllowedEvents` could be misread as guard-approved events; it needed explicit structural-query semantics. |

## 7-Tier Review - Iteration 1

| Tier | P0 | P1 | P2 | P3 | Evidence |
|---|---:|---:|---:|---:|---|
| Tier 1 Security | 0 | 0 | 0 | 0 | Framework-free in-memory package; no credential, network, parser, or auth boundary. |
| Tier 2 Ops/SRE reliability | 0 | 1 | 0 | 0 | Cancellation contract checked before lookup but not explicitly after guard/before commit. |
| Tier 3 Structural impact | 0 | 1 | 0 | 0 | Spec lacked required design option comparison and rejection rationale for a non-trivial public package. |
| Tier 4 Go/API quality | 0 | 1 | 0 | 0 | `CanTransition` evaluates guard code but did not define side-effect expectations. |
| Tier 5 Tests/types/silent failure | 0 | 1 | 0 | 0 | Tests did not require `TransitionError` to support both sentinel and wrapped-cause `errors.Is` behavior. |
| Tier 6 Performance/stability | 0 | 1 | 0 | 0 | Guard execution outside lock is specified, but context recheck and concurrent loser behavior needed tighter test requirements. |
| Tier 7 Docs/release/evidence | 0 | 1 | 0 | 0 | README/Go doc must distinguish registered events from guard-approved transitions. |

## 통합 발견 사항

| ID | Severity | Finding | Required spec change | Resolution |
|---|---|---|---|---|
| S2R-1 | P1 | `CanTransition` can run caller-supplied guard code, so side effects could happen during an inquiry call. | State that guards used with `CanTransition` must be safe for inquiry calls and docs/tests must demonstrate this contract. | Added behavior, risk, and test requirements. |
| S2R-2 | P1 | `AllowedEvents` implied "allowed" could mean guard-approved even though no context is available. | Define it as a structural registry query in registration order that does not evaluate guards. | Added behavior and test requirements. |
| S2R-3 | P1 | The spec missed required approach comparison and rejected-approach rationale. | Add materially different options and explicit rejection rationale grounded in #26/#135 and prior states comparison. | Added `Design Options`. |
| S2R-4 | P1 | Context cancellation was only checked before reading state, leaving post-guard cancellation before commit underspecified. | Require a second cancellation check before commit and clarify guard responsibility during execution. | Added behavior contract text. |
| S2R-5 | P1 | Error inspection requirements did not prove callers can check both package sentinel and guard/context causes. | Define `TransitionError` `Kind`, `Cause`, `Is`, and `Unwrap` expectations and add tests. | Added error contract and test requirement. |

## 7-Tier Review - Iteration 2

Affected lanes rerun after the spec edits:

| Tier | P0 | P1 | P2 | P3 | Evidence |
|---|---:|---:|---:|---:|---|
| Tier 2 Ops/SRE reliability | 0 | 0 | 0 | 0 | Spec now requires cancellation check before lookup and before commit, and assigns guard context observation to the guard. |
| Tier 3 Structural impact | 0 | 0 | 0 | 0 | `Design Options` records adopted mutable machine and rejected stateless, DSL/callback/runtime, and dependency approaches. |
| Tier 4 Go/API quality | 0 | 0 | 0 | 0 | `CanTransition` inquiry-side-effect contract is explicit and stays aligned with #135's guard-evaluating requirement. |
| Tier 5 Tests/types/silent failure | 0 | 0 | 0 | 0 | Tests now require `TransitionError` `errors.Is` coverage for sentinel and wrapped causes. |
| Tier 6 Performance/stability | 0 | 0 | 0 | 0 | Guard-outside-lock, state recheck, concurrent loser error, and race validation are all testable requirements. |
| Tier 7 Docs/release/evidence | 0 | 0 | 0 | 0 | README/Go doc tasks must document `AllowedEvents` as a structural query and guard inquiry safety. |

Unchanged lanes:

| Tier | P0 | P1 | P2 | P3 | Evidence |
|---|---:|---:|---:|---:|---|
| Tier 1 Security | 0 | 0 | 0 | 0 | No security-sensitive boundary added by the spec edits. |

## Critic Integration

No open contradictions remain:

- #135 says `CanTransition` evaluates guard logic; #26 keeps that behavior but
  documents side-effect expectations.
- `AllowedEvents` remains in the API because #135 requires it, but the spec now
  avoids overclaiming guard approval.
- The Go API remains first-party, dependency-free, and framework-free.
- The Kotlin `utils/states` CAS/concurrent-conflict concept is adapted without
  porting Kotlin DSL, coroutine, reactive, callback, or nested-transition
  layers.

Rejected during review:

- Renaming `AllowedEvents` in #26. It would diverge from #135 and Kotlin
  reference naming without enough benefit; documentation is sufficient.
- Removing guard evaluation from `CanTransition`. It would conflict with #135's
  explicit requirement that the method evaluates the current transition and
  guard without mutating state.

Open questions for user: none.

## 수렴 판정

P0=0 P1=0

P2=0 P3=0

Step 2-R gate status: PASS. The spec may proceed to Step 3 plan authoring.

### Step 2-R Checklist Completion Report

| 항목 | 상태 | Notes |
|------|--------|-------|
| Four perspective reviews complete | Done | Developer/API, security, Ops/SRE, and user/caller perspectives recorded. |
| Selected Step 2-R native subagent lanes complete or local-equivalent reason recorded | Done | Local-equivalent used because subagent tool requires explicit user request for sub-agent work. |
| Step 2-R Tier 1 security spec review complete | Done | No security boundary introduced. |
| Step 2-R Tier 2 Ops/SRE reliability spec review complete | Done | Context and guard cancellation contract revised and rerun. |
| Step 2-R Tier 3 structural impact spec review complete | Done | Design options added and rerun. |
| Step 2-R Tier 4 Kotlin/API quality spec review complete | Done | Go/API inquiry guard contract revised and rerun; Kotlin-specific code rules N/A for this Go package. |
| Step 2-R Tier 5 testability/silent failure spec review complete | Done | `TransitionError` `errors.Is` tests added to spec. |
| Step 2-R Tier 6 performance/stability spec review complete | Done | Guard-outside-lock and concurrent-loser behavior covered. |
| Step 2-R Tier 7 documentation/release/evidence spec review complete | Done | README/Go doc semantics clarified. |
| Critic integration complete | Done | Integrated findings and rejected alternatives recorded. |
| Review findings normalized into P0/P1/P2/P3 | Done | Tables above. |
| P0 items revised, re-reviewed, and approved when user approval is required | N/A | No P0 findings. |
| P1 items revised and re-reviewed | Done | Five P1 findings revised and affected lanes rerun. |
| Convergence verification passed with P0 = 0 and P1 = 0 | Done | `P0=0 P1=0`. |
| Step 2-R closure declared only after P0/P1 reached 0 | Done | Closure appears after iteration 2. |
| P2/P3 items revised or explicitly deferred with reason | N/A | No P2/P3 findings after normalization. |
| Open questions surfaced to user instead of guessed | Done | No open questions remain. |
