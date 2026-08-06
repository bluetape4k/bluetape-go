# Issue 425 P1 Fix Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-07-07 KST

## 범위

- Issue: #425
- Baseline: `4dca212` (`Record blockers before 0.13.0 fixes`)
- Fixes:
  - `cache.Memory.GetOrLoad` same-key cancellation isolation.
  - `ratelimit/redis` caller-owned logical key preservation.

## 원인

| Finding | Root cause | Fix |
|---|---|---|
| P1-1 cache same-key cancellation | The shared `singleflight` load captured the first caller's context and returned its context error to all waiters. | Live waiters retry when a shared result fails with `context.Canceled` or `context.DeadlineExceeded` while their own context remains live. Context-error paths also drop the per-key flight mapping. |
| P1-2 Redis rate-limit key trimming | `normalizeKey` returned `strings.TrimSpace(key)`, so whitespace-distinct caller keys mapped to one Redis bucket. | Blank validation still trims for inspection, but byte-length validation and storage use the original key. |

## 7-Tier 검토

| Lane | Verdict | Evidence |
|---|---|---|
| Performance | Pass | Cache retry occurs only after a shared context-cancellation result; normal cache hits and successful singleflight collapse remain unchanged. Redis key preservation changes validation only and does not add Redis round trips. |
| Stability | Pass | `TestMemorySameKeyCanceledOwnerDoesNotCancelLiveWaiter` proves a live waiter recovers after owner cancellation. Existing same-key stress and distinct-key tests remain in package scope. |
| Security | Pass | No auth, secret, serialization, command construction, or trust-boundary behavior changed. Redis Lua script remains static and still receives one bucket key. |
| Operator/Ops | Pass | Redis bucket TTL and namespace behavior are unchanged; exact caller keys may create the distinct buckets the API already documented. |
| Developer/API | Pass | Public API signatures are unchanged. Behavior now matches the README and prior Redis caller-key lesson. |
| User/Caller | Pass | Callers no longer inherit unrelated same-key cancellation, and whitespace-distinct Redis logical keys no longer share rate-limit state. |
| Integration | Pass | Targeted package tests and race tests passed for both touched packages. |

## 검증

- `go test -count=1 ./cache -run TestMemorySameKeyCanceledOwnerDoesNotCancelLiveWaiter`: RED before fix, PASS after fix.
- `go test -count=1 ./ratelimit/redis -run TestLimiterPreservesCallerOwnedKeys`: RED before fix, PASS after fix.
- `go test -count=1 ./cache ./ratelimit/redis`: PASS.
- `go test -race -count=1 ./cache ./ratelimit/redis`: PASS.
- `git diff --check`: PASS.
- `make ci`: initial `lint` attempt hit a stale golangci-lint cache entry that
  referenced the removed `issue-424-retrospective-review` worktree. After
  `golangci-lint cache clean`, rerun PASS: `tidy-check`, `fmt-check`, `vet`,
  `lint`, `test`, and `race`.

## 발견 사항

- P0: 0
- P1: 0
- P2: 0
- P3: 0

P0=0 P1=0

## 판정

Pass. The two #424 retrospective P1 blockers routed to #425 are fixed with
regression tests and targeted race validation.
