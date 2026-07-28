# Issue #23 Redis NearCache Step 6-R Code Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

날짜: 2026-06-04
브랜치: `feat/issue-23-near-cache`
Reviewed scope:

- `cache/redisnear/doc.go`
- `cache/redisnear/message.go`
- `cache/redisnear/near_cache.go`
- `cache/redisnear/*_test.go`
- `README.md`
- `README.ko.md`
- `CHANGELOG.md`
- GitHub issue #107 benchmark follow-up body

Required references loaded:

- `bluetape4k-full-feature/references/step-6r-code-review.md`
- `bluetape4k-full-feature/references/step-4p-perf-scan.md`

## 통합 발견 사항

| Priority | File:Line | Tier | Finding | Resolution |
|---|---|---|---|---|
| P2 | `cache/redisnear/near_cache_test.go` | Tier 5/Tier 6 | Receive-error local clear behavior was specified but not directly tested in the first implementation pass. | Fixed by adding `TestNearCacheClearsLocalOnReceiveError`; `go test -count=1 ./cache/redisnear` and `go test -race -count=1 ./cache/redisnear` passed. |
| P2 | `cache/redisnear/example_test.go` | Tier 7 | `make ci` errcheck reported unchecked `Close` errors in the compile-only example. | Fixed by wrapping deferred closes with `_ = ...`; rerun `make ci` passed with 0 lint issues. |
| P1 | `cache/redisnear/near_cache.go` | Tier 2/Tier 6 | Blocking or panicking `OnError` could stop or delay subscriber invalidation processing. | Fixed by adding a bounded asynchronous error reporter and panic recovery; verified by `TestNearCacheOnErrorDoesNotBlockSubscriber` and `TestNearCacheOnErrorPanicIsRecovered`. |
| P1 | `cache/redisnear/near_cache.go` | Tier 2/Tier 3 | Public operations only checked `closed` before releasing the lifecycle lock, so `Close` could race with local reads/writes. | Fixed with a closed gate plus in-flight wait counter and bounded shutdown error; verified by `TestNearCacheCloseWaitsForInflightOperation`. |
| P1 | `cache/redisnear/near_cache_test.go` | Tier 5/Tier 6 | Stress coverage used one NearCache instance and did not exercise peer invalidation under concurrent operations. | Fixed by changing `TestNearCacheConcurrentStress` to run two Redis-backed peers concurrently. |
| P2 | `cache/redisnear/near_cache.go` | Tier 3/Tier 5 | Public `Client` interface required `redis.Cmdable`, which is much wider than the implementation needs. | Fixed by narrowing `Client` to `Publish` and `Subscribe`. |
| P2 | `README.md`, `README.ko.md` | Tier 1/Tier 7 | README did not document publish-failure divergence, best-effort `OnError`, or Redis channel trust assumptions. | Fixed in both README locale files. |

Final blocking counts: P0 = 0, P1 = 0.

## Tier Review

| Tier | Focus | P0 | P1 | P2 | P3 | Evidence |
|---|---|---:|---:|---:|---:|---|
| 1 Security | Redis Pub/Sub payload, user input, secrets | 0 | 0 | 0 | 0 | README now states Pub/Sub messages are invalidation commands and Redis ACL/TLS/channel isolation is required. |
| 2 Ops/SRE reliability | Lifecycle, shutdown, error paths | 0 | 0 | 0 | 0 | Subscribe ack before constructor return; `Close` is idempotent, gates new operations, and waits for in-flight operations with a bounded timeout; receive errors clear local cache; `OnError` is bounded async with panic recovery; bounded backoff avoids spin. |
| 3 Structural impact | Public API, module boundaries | 0 | 0 | 0 | 0 | New package depends on existing `cache` contract; Redis client interface is narrowed to `Publish` and `Subscribe`. |
| 4 Code quality | Go idioms, comments, maintainability | 0 | 0 | 0 | 0 | Small package, explicit option normalization, typed operation constants, no broad global state besides constants. |
| 5 Tests/types/silent failure | Assertions, cancellation, hidden failures | 0 | 0 | 0 | 0 | Message tests, close semantics, malformed and blocking/panicking error hooks, Testcontainers peer invalidation, receive-error clear, peer stress, cancellation. |
| 6 Performance/stability | Hot path, waits, retries, cleanup | 0 | 0 | 0 | 0 | No value serialization in Redis path; bounded receive backoff; Testcontainers readiness uses `PING`; race test passed. Benchmarks tracked in #107. |
| 7 Docs/release/evidence | README locale, changelog, follow-ups | 0 | 0 | 0 | 0 | README/README.ko and CHANGELOG updated; #107 and #110 cover benchmark/RESP3 follow-ups. |

## 수렴

| Iteration | P0 | P1 | P2 | P3 | Action |
|---|---:|---:|---:|---:|---|
| Initial review | 0 | 0 | 1 | 0 | Added direct receive-error clear test. |
| CI lint review | 0 | 0 | 1 | 0 | Fixed unchecked deferred close calls in example. |
| Hard PR review | 0 | 3 | 4 | 1 | Fixed P1 lifecycle/error/stress findings and P2 API/docs findings; deferred no blocking issues. |
| Final review | 0 | 0 | 0 | 0 | Gate closed. |

Step 6-R verdict: PASS with P0 = 0 and P1 = 0.
