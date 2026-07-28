# Issue #117 Cross-Process Stampede Protection Verifier

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

날짜: 2026-06-04
브랜치: `feat/issue-117-stampede`
범위: `cache/rediscoord`, `README.md`, `README.ko.md`, `CHANGELOG.md`, `Makefile`

## Spec DoD Verification

| Requirement | Status | Evidence |
|---|---:|---|
| New opt-in Redis coordinator package | Done | `cache/rediscoord.NewStampedeCache`. |
| Existing `cache.LoadingCache` contract unchanged | Done | Wrapper implements `cache.LoadingCache[string,V]`; root `cache` API unchanged. |
| `redisnear` remains invalidation-only by default | Done | No changes to `cache/redisnear` source behavior. |
| Owner-token/TTL load lease | Done | Wrapper reuses `lock/redis` acquire/unlock semantics. |
| Shared result envelope | Done | Token-bound versioned Redis result envelope with caller-provided `Codec[V]`. |
| NearCache post-invalidation collapse | Done | `TestStampedeCacheCollapsesNearCacheLoadsAfterInvalidation`. |
| Stress coverage | Done | `TestStampedeCacheSameKeyStressUsesOneLoader` uses `GoroutineStressTester`. |
| Cancellation coverage | Done | `TestStampedeCacheAsyncWaiterCancellation` uses `AsyncJobTester`. |
| Lease expiry recovery | Done | `TestStampedeCacheLeaseExpiryLetsPeerRecover`. |
| Docs updated | Done | Root README pair links to package details; `cache/rediscoord/README.md` contains usage, semantics, benchmark chart, and measured table. CHANGELOG, WIP, and research index are updated. |
| Benchmarks kept opt-in | Done | `BenchmarkStampedeCache*` added to `make bench-cache`, not `make ci`. |

## 검증 증거

| Command | Status | Result |
|---|---:|---|
| `go test -count=1 ./cache/rediscoord` | PASS | 16 tests passed. |
| `go test -race -count=1 ./cache/rediscoord` | PASS | 16 tests passed under race detector. |
| `go test -count=1 ./cache ./cache/redisnear ./cache/rediscoord ./lock/redis` | PASS | 59 tests passed. |
| `go test -run '^$' -bench '^BenchmarkStampedeCache' -benchtime=10ms -benchmem ./cache/rediscoord` | PASS | Hot path and cold winner benchmark smoke passed; cold winner reported `1.000 loads/op`. |
| `go test -run '^$' -bench '^BenchmarkMemory(GetHit\|GetOrLoadCold\|GetOrLoadSameKeyConcurrent)' -benchtime=100ms -benchmem ./cache` | PASS | README benchmark snapshot source. |
| `go test -run '^$' -bench '^BenchmarkNearCache(GetLocalHit\|SetPublish\|GetOrLoadUnderInvalidation)' -benchtime=100ms -benchmem ./cache/redisnear` | PASS | README benchmark snapshot source. |
| `go test -run '^$' -bench '^BenchmarkStampedeCache' -benchtime=100ms -benchmem ./cache/rediscoord` | PASS | README benchmark snapshot source. |
| `go test -count=1 ./...` | PASS | 237 tests passed in 20 packages. |
| `make ci` | PASS | Lint reported 0 issues; full test and race suites passed. |

## Behavioral Boundaries

- The wrapper is a coordination/result transport, not a durable Redis L2 cache.
- Redis can see encoded payload bytes; deployments must use ACL/TLS and
  namespace separation when payloads are sensitive.
- Mutual exclusion is bounded by `LockTTL`. If a loader exceeds the lease,
  another process may acquire and load.
- Result envelopes are short-lived and accepted only for the observed owner
  token.

Verifier verdict: PASS. `P0 = 0`, `P1 = 0`.
