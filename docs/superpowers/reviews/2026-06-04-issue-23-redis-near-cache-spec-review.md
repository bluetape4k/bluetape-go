# Issue 23 Redis Near Cache Spec Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #23
게이트: Step 2-R
날짜: 2026-06-04
Reviewed spec: `docs/superpowers/specs/2026-06-04-issue-23-redis-near-cache-spec.md`

## 범위

Review the public API, invalidation semantics, Redis Pub/Sub lifecycle,
stress/benchmark requirements, and future RESP3 strategy boundary for the first
Redis NearCache implementation.

## 관점별 발견 사항

| Perspective | P0 | P1 | P2 | P3 | Notes |
|---|---:|---:|---:|---:|---|
| Developer/API | 0 | 0 | 0 | 0 | `cache/redisnear` and `NewPubSub` keep the first strategy explicit and Go-native. |
| Security | 0 | 0 | 0 | 0 | JSON payload has no code execution path; malformed payloads are ignored and reported. |
| Ops/SRE | 0 | 0 | 1 | 0 | Receive errors must not create a tight loop. Spec now requires bounded backoff. |
| User/Caller | 0 | 0 | 0 | 0 | Non-goals and consistency model explain stale-read boundaries and bypassed writes. |

## 로컬 7-Tier 검토

| Tier | P0 | P1 | P2 | P3 | Evidence |
|---|---:|---:|---:|---:|---|
| 1 Security | 0 | 0 | 0 | 0 | Message fields are strings/ints; unknown/malformed payloads are not executed. |
| 2 Ops/SRE reliability | 0 | 0 | 1 | 0 | Subscriber receive error handling requires local clear and bounded retry. |
| 3 Structural impact | 0 | 0 | 0 | 0 | New public package matches package layout policy and isolates Redis strategy. |
| 4 API quality | 0 | 0 | 0 | 0 | API uses constructor-per-strategy and implements existing cache contracts for string keys. |
| 5 Tests/types/silent failure | 0 | 0 | 0 | 0 | Spec requires Testcontainers, stress, cancellation, and close behavior tests. |
| 6 Performance/stability | 0 | 0 | 0 | 0 | Benchmarks are linked to #107; stress remains in #23. |
| 7 Docs/release/evidence | 0 | 0 | 0 | 0 | README/CHANGELOG/package docs/lesson requirements are recorded. |

## 통합 발견 사항

| Severity | Finding | Resolution |
|---|---|---|
| P1 | `Close` behavior was initially unspecified, allowing stale local reads after invalidation stopped. | Resolved in spec by adding public `ErrClosed` and requiring all cache operations after `Close` to return it. |
| P2 | Receive errors could spin without a retry policy. | Resolved in spec by requiring bounded backoff after receive errors. |

## 거절한 대안

- Hide Pub/Sub and RESP3 behind a single runtime enum: rejected because the
  lifecycle and consistency guarantees differ materially.
- Add Ristretto/BigCache in #23: rejected because #107 owns benchmark-driven
  local storage decisions.

## 게이트 판정

P0 = 0
P1 = 0

Step 2-R is closed. The plan may proceed if it preserves `ErrClosed`, bounded
receive retry, Testcontainers peer invalidation, stress tests, and #107
benchmark linkage.
