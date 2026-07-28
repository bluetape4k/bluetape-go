# Issue #23 Redis NearCache Verifier

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

날짜: 2026-06-04
브랜치: `feat/issue-23-near-cache`
범위: `cache/redisnear`, `README.md`, `README.ko.md`, `CHANGELOG.md`

## Spec DoD Verification

| Requirement | Status | Evidence |
|---|---:|---|
| Pub/Sub-first Redis NearCache, RESP3 deferred | Done | `cache/redisnear.NewPubSub`; RESP3 follow-up tracked in #110. |
| Implements `cache.LoadingCache[string,V]` | Done | Compile-time assertion in `cache/redisnear/near_cache.go`. |
| Redis is invalidation-only, not value storage | Done | `GetOrLoad` delegates only to local cache; `Set/Delete/Clear` publish invalidation messages only. |
| Defaults for namespace, channel, origin ID, local cache | Done | `normalizeOptions` applies defaults; missing client is rejected. |
| Subscribe readiness before constructor returns | Done | `NewPubSub` calls `pubsub.Receive(ctx)` after `Subscribe`. |
| Self-origin messages ignored | Done | `TestNearCacheIgnoresOwnOrigin`. |
| Peer `Set/Delete/Clear` invalidates local entries | Done | `TestNearCacheInvalidatesPeerEntries`. |
| Malformed/unknown messages reported and ignored | Done | `TestNearCacheReportsMalformedMessages`, `TestDecodeMessageRejectsMalformedPayload`. |
| Receive errors clear local cache and call `OnError` | Done | `TestNearCacheClearsLocalOnReceiveError`. |
| Blocking or panicking `OnError` does not stop invalidation | Done | `TestNearCacheOnErrorDoesNotBlockSubscriber`, `TestNearCacheOnErrorPanicIsRecovered`. |
| Close is idempotent and public operations return `ErrClosed` after close | Done | `TestNearCacheCloseIsIdempotentAndBlocksOperations`. |
| `Close` waits for already entered operations before returning | Done | `TestNearCacheCloseWaitsForInflightOperation`. |
| Stress/cancellation tests included | Done | `TestNearCacheConcurrentStress`, `TestNewPubSubPropagatesCanceledContext`. |
| Benchmark follow-up tracked | Done | GitHub issue #107 body updated with Redis NearCache benchmark acceptance criteria. |

## Plan Task Verification

| Task | Status | Evidence |
|---|---:|---|
| T1 scaffold package/API | Done | `cache/redisnear/doc.go`, `near_cache.go`. |
| T2 message contract | Done | `message.go`, `message_test.go`. |
| T3 subscriber lifecycle | Done | Subscribe ack, receive loop, close, backoff, receive-error clear. |
| T4 cache operations | Done | `Get`, `Set`, `Delete`, `Clear`, `GetOrLoad`, publish behavior. |
| T5 Testcontainers peer proof | Done | Redis-backed two-client peer invalidation test. |
| T6 stress/cancellation | Done | `GoroutineStressTester` and `AsyncJobTester` tests. |
| T7 examples/docs | Done | README pair, changelog, compile-only example. |
| T8 lessons/review prep | Done | This verifier plus Step 6-R review and lessons artifacts. |

## 검증 증거

| Command | Status | Result |
|---|---:|---|
| `go test -count=1 ./cache/redisnear` | PASS | 13 tests passed. |
| `go test -race -count=1 ./cache/redisnear` | PASS | 13 tests passed under race detector. |
| `go test -count=1 ./cache ./cache/redisnear` | PASS | 26 tests passed. |
| `go test -count=1 ./...` | PASS | 204 tests passed. |
| `make ci` | PASS | Lint reported 0 issues; full test and race suites passed. |

## Open Items

| 항목 | 상태 | Tracking |
|---|---:|---|
| Redis NearCache benchmark suite | Follow-up | #107 |
| RESP3 CLIENT TRACKING strategy | Follow-up | #110 |
| Cross-process stampede protection | Future scope | Roadmap / future issue |

Verifier verdict: PASS. Spec and plan requirements are implemented with no known P0/P1 gaps.
