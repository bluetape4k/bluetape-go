# Issue #117 Cross-Process Stampede Protection Code Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

날짜: 2026-06-04
범위: `cache/rediscoord`, docs, benchmark target
Review type: 7-Tier implementation review

## 판정

PASS. `P0 = 0`, `P1 = 0`, `P2 = 0`.

## 7-Tier 발견 사항

| Tier | Focus | P0 | P1 | P2 | P3 | Finding |
|---|---|---:|---:|---:|---:|---|
| 1 Requirements | #117 acceptance | 0 | 0 | 0 | 0 | Research, API decision, Redis TTL lock, multi-near-cache collapse, cancellation, expiry, docs, and benchmark boundary are implemented. |
| 2 API/UX | Public surface | 0 | 0 | 0 | 1 | `NewStampedeCache`, `Options`, `Codec`, and `JSONCodec` are explicit. Users must opt in and choose serialization. |
| 3 Integration | cache/redisnear/lock/redis | 0 | 0 | 0 | 0 | Waiters fill local state through wrapped `GetOrLoad`, avoiding accidental NearCache invalidation publish. |
| 4 Security/data | Redis payload exposure | 0 | 0 | 0 | 1 | Payload exposure is documented in README; caller controls codec and Redis isolation. |
| 5 Tests/types | Silent failures and races | 0 | 0 | 0 | 0 | Unit, Testcontainers, stress, cancellation, expiry, race, and benchmark smoke coverage pass. |
| 6 Performance/stability | Lock TTL, polling, benchmark | 0 | 0 | 0 | 1 | Polling is context-bound; benchmark target is opt-in. Loader over-lease behavior is documented. |
| 7 Docs/ops | Release notes and WIP | 0 | 0 | 0 | 0 | README pair, CHANGELOG, WIP, research index, verifier, and lessons are updated. |

## 검토 메모

- Token-bound envelopes prevent waiters from accepting stale result data from a
  different owner attempt.
- `ensureOwner` reads the lease's Redis key directly, so user cache keys do not
  need parsing or escaping for owner checks.
- Unlock uses a short background context so caller cancellation does not skip
  cleanup.
- The package intentionally returns an error when result publication fails after
  a local fill; this surfaces a failed cross-process guarantee instead of hiding
  coordination loss.

## 잔여 위험

- A durable Redis L2 cache may still be useful later for applications that want
  shared values outside a cold burst. That is intentionally outside #117.
- Workloads with loaders longer than `LockTTL` must tune the lease or accept
  possible overlapping loads after expiry.

Gate verdict: PASS. No blocker for PR.
