# Issue #117 Cross-Process Stampede Protection Spec Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

날짜: 2026-06-04
범위: `docs/research/2026-06-04-issue-117-cross-process-stampede-protection.md`, `docs/superpowers/specs/2026-06-04-issue-117-cross-process-stampede-protection-spec.md`
Review type: 7-Tier spec gate

## 판정

PASS with implementation watch items. `P0 = 0`, `P1 = 0`.

## 7-Tier 발견 사항

| Tier | Focus | P0 | P1 | P2 | P3 | Finding |
|---|---|---:|---:|---:|---:|---|
| 1 Requirements | Issue acceptance mapping | 0 | 0 | 0 | 0 | Spec maps every #117 acceptance criterion to API, behavior, tests, and docs. |
| 2 API/UX | Public API stability | 0 | 0 | 1 | 0 | `Codec[V]` is explicit and avoids hidden serialization; implementation must keep constructor naming narrow to avoid promising a durable Redis L2 cache. |
| 3 Integration | `cache`, `redisnear`, `lock/redis`, Redis | 0 | 0 | 1 | 0 | Wrapper integration is sound. Watch Redis key construction and token/result race handling in code. |
| 4 Data/security | Result payload and Redis trust boundary | 0 | 0 | 1 | 0 | Redis sees encoded payloads. Docs must state ACL/TLS/isolation expectations and that payload confidentiality is caller responsibility. |
| 5 Tests/types/silent failure | Test coverage | 0 | 0 | 0 | 0 | Unit, Testcontainers, stress, cancellation, and expiry tests are required with clear claims. |
| 6 Performance/stability | Stampede, polling, TTL | 0 | 0 | 1 | 0 | Polling is bounded by context and interval. Implementation must avoid busy loops and must not add benchmarks to normal CI. |
| 7 Docs/operations | README/CHANGELOG/lessons | 0 | 0 | 0 | 0 | Public docs and failure semantics are required before PR. |

## Watch Items For Plan

- Define exact behavior for result publish/unlock errors after a successful
  loader result.
- Ensure waiters only accept a result token observed for the current lock owner.
- Use a background cleanup context for unlock so caller cancellation does not
  leave the lock until TTL unnecessarily.
- Avoid adding `OnError` unless implementation evidence shows it is necessary;
  simpler error returns are preferable for the first API.

## Gate

Spec gate passes. Proceed to Step 3 implementation plan with the watch items
addressed explicitly.
