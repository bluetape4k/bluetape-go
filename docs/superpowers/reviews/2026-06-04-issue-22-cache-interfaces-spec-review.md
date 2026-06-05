# Issue 22 Cache Interfaces Spec Review

Spec: `docs/superpowers/specs/2026-06-04-issue-22-cache-interfaces-spec.md`
Research: `docs/superpowers/research/2026-06-04-issue-22-cache-interfaces-research.md`
Gate: Step 2-R
Date: 2026-06-04

## Review Scope

- Public `cache` API shape.
- TTL and cache-miss semantics.
- `singleflight` same-key duplicate-load suppression.
- Context cancellation and loader error behavior.
- Required stress-test and documentation coverage.

Required reference loaded: `/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-2r-spec-review.md`.

## Iteration 1 Findings

| Priority | Perspective | Finding | Required spec edit | Status |
|---|---|---|---|---|
| P1 | Developer / Performance | Spec said `singleflight.Group.Do` needs a stable key representation, but did not forbid collision-prone `fmt.Sprint` style conversion for generic `K comparable`. Distinct keys could incorrectly share a loader result. | Require collision-free cache-instance-scoped flight keys and add a test for distinct keys that stringify similarly. | Fixed |
| P2 | Ops/SRE | Spec did not explicitly say the cache mutex must not be held during loader execution. A slow loader could block unrelated cache operations. | Add behavior/failure-mode requirement that loader runs outside cache mutex. | Fixed |
| P2 | User/caller | Spec did not state what happens when `Delete` or `Clear` races with an already in-flight loader. Callers could assume deletion cancels existing loads. | Document cache-aside ordering: `Delete`/`Clear` do not cancel existing loaders, and a later successful loader may repopulate. | Fixed |

## Applied Spec Edits

- Added collision-free `singleflight` key requirement for generic keys.
- Added loader execution outside cache mutex requirement.
- Added failure-mode mitigations for key collisions and loader blocking.
- Added test requirement for distinct keys with similar string forms.
- Added concurrent caller safety and `Delete`/`Clear` versus in-flight loader
  ordering contract.

## 7-Tier Review

| Tier | Scope | P0 | P1 | P2 | P3 | Evidence |
|---|---|---:|---:|---:|---:|---|
| 1 Security | Public cache API and loader behavior | 0 | 0 | 0 | 0 | No auth, secret, injection, or deserialization surface is introduced. Loader errors are not cached. |
| 2 Ops/SRE reliability | Context, cancellation, failure diagnosis | 0 | 0 | 0 | 0 | Spec requires context propagation, no mutation on canceled context, no cache write after loader failure/cancellation, and explicit delete/clear in-flight ordering. |
| 3 Structural impact | Package boundary and future Redis near-cache | 0 | 0 | 0 | 0 | Root `cache` package is bounded; Redis invalidation/cross-process behavior remains #23. |
| 4 Go/API quality | Generic API and Go idioms | 0 | 0 | 0 | 0 | `context.Context` first, `K comparable`, `errors.Is(ErrCacheMiss)`, no new dependency. Collision-free flight key requirement added. |
| 5 Tests/types/silent failure | Acceptance and stress coverage | 0 | 0 | 0 | 0 | Spec maps issue criteria to unit, stress, cancellation, and example tests, including `GoroutineStressTester` and `AsyncJobTester`. |
| 6 Performance/stability | Stampede, locking, TTL flakiness | 0 | 0 | 0 | 0 | `singleflight` required, loader outside mutex, TTL flakiness mitigation captured. |
| 7 Docs/release/evidence | README, docs, evidence integrity | 0 | 0 | 0 | 0 | Package docs, README English/Korean sync, and public-doc language rules included. |

## Multi-Perspective Review

| Perspective | P0 | P1 | P2 | P3 | Notes |
|---|---:|---:|---:|---:|---|
| Developer | 0 | 0 | 0 | 0 | API is implementable with existing `singleflight` dependency and in-memory map. |
| Security | 0 | 0 | 0 | 0 | No sensitive backend or credential behavior. |
| Ops/SRE | 0 | 0 | 0 | 0 | Context and loader failure behavior are explicit. |
| User/caller | 0 | 0 | 0 | 0 | `ErrCacheMiss`, TTL rules, same-key TTL race behavior, and delete/clear in-flight ordering are documented. |

## Critic Integration

| Priority | Area | Finding | Resolution |
|---|---|---|---|
| P1 | Generic key correctness | Initial spec lacked collision-free `singleflight` key requirement. | Fixed and re-reviewed. |
| P2 | Loader isolation | Initial spec lacked explicit no-mutex-during-loader requirement. | Fixed and re-reviewed. |
| P2 | Concurrent caller semantics | Initial spec lacked delete/clear versus in-flight loader ordering. | Fixed and re-reviewed. |

Open questions: none. No user decision is required before Step 3.

## Gate Verdict

Step 2-R convergence passed.

| Metric | Count |
|---|---:|
| P0 | 0 |
| P1 | 0 |
| P2 | 0 |
| P3 | 0 |

## Step 2-R Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Required reference loaded | Done | `step-2r-spec-review.md`. |
| Multi-perspective review complete | Done | Developer, security, Ops/SRE, user/caller perspectives recorded. |
| Local 7-tier review complete | Done | All seven tiers recorded. |
| Critic integration complete | Done | Initial findings fixed and re-reviewed. |
| P0/P1 fixed and rerun | Done | Latest integrated table has `P0 = 0`, `P1 = 0`. |
| Gate closed | Done | Step 3 may start. |
