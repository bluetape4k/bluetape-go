# Issue #175 Step 6-R Code Review

Issue: #175
Date: 2026-06-14
Branch: `issue-175-jwt-provider-cache-adapters`
Gate: 7-Tier = 6 independent lanes + main integration review
Wait SLA: subagent wait max 10 minutes; long blocking wait is forbidden.
Timeout retry rule: if a lane is closed after exceeding the 10-minute SLA,
rerun that lane up to 3 times before final main-session fallback.

This Step 6-R was redone because the earlier review attempt did not preserve
the required 6 independent lanes + main integration shape consistently enough.

## Reviewed Scope

- JWT provider cache adapters:
  - `jwt/cache_options.go`
  - `jwt/cache_key.go`
  - `jwt/cached_provider.go`
  - `jwt/cached_distributed_provider.go`
- Tests and benchmarks:
  - `jwt/cache_key_test.go`
  - `jwt/cached_provider_test.go`
  - `jwt/cached_distributed_provider_test.go`
  - `jwt/cache_concurrency_test.go`
  - `jwt/cache_failure_test.go`
  - `jwt/cache_benchmark_test.go`
  - `jwt/redis_benchmark_test.go`
- Docs and examples:
  - `jwt/README.md`
  - `jwt/README.ko.md`
  - `jwt/jwt_example_test.go`
  - `jwt/redis/example_test.go`
  - `docs/images/readme-diagrams/jwt-provider-cache-adapter-flow.*`
  - `scripts/generate-jwt-provider-cache-adapter-diagram.mjs`

## Initial Lane Results

| Tier | Perspective | Agent | Verdict | P0 | P1 | P2 | P3 |
|---|---|---|---:|---:|---:|---:|---:|
| 1 | Performance | `019ec203-6040-7c40-a13a-3543203d5eb5` | COMMENT | 0 | 0 | 3 | 0 |
| 2 | Stability | `019ec203-8505-7bf1-b35b-20bd9111601d` | REQUEST_CHANGES | 0 | 1 | 0 | 0 |
| 3 | Security | `019ec203-a02e-78a0-8eb5-8bb364520f87` | REQUEST_CHANGES | 0 | 2 | 0 | 0 |
| 4 | Operator/Ops | `019ec203-bda8-7960-a81c-7fd9c40c5310` | COMMENT | 0 | 0 | 0 | 1 |
| 5 | Developer/API | `019ec203-e7f0-7ad0-8ebd-ff626b484cd4` | APPROVE | 0 | 0 | 0 | 0 |
| 6 | User/Caller | `019ec204-0394-7f72-8c31-8c5976487eaf` | COMMENT | 0 | 0 | 0 | 1 |
| Main | Integration | main session | REQUEST_CHANGES | 0 | 3 | 3 | 2 |

## Blocking Findings And Fixes

| Lane | Severity | Finding | Resolution |
|---|---:|---|---|
| Stability | P1 | `CachedDistributedProvider` lacked bounded stress coverage for cold-burst singleflight and parse/rotation/delete races. | Added `TestCachedDistributedProviderColdBurstUsesSingleflight` and `TestCachedDistributedProviderStressParseRotateAndDelete` with `GoroutineStressTester`. |
| Security | P1 | TTL clipping and non-positive TTL no-cache were not proven. | Added local and distributed table tests for max TTL, token TTL, key TTL, and expired-token no-cache. |
| Security | P1 | Stale cached Reader revalidation branches were under-tested. | Added local and distributed seeded-cache stale hit tests for nil reader, wrong algorithm, unknown kid, successful delete, and reparse. Local test also covers expired-key no-recache. |
| User/Caller | P3 | Default trust scope randomization was not documented. | Documented per-construction random default and stable private scope guidance in EN/KO README. |
| Operator/Ops | P3 | Distributed provider example was not self-contained. | Added `opCtx`, Redis repository creation, provider construction, and token construction to EN/KO README snippets. |
| User/Caller rerun | P3 | Distributed README snippets used nonexistent `redisjwt.NewRepository`. | Replaced with actual `redisjwt.New(redisjwt.Options{...})` API in EN/KO README. |
| Security rerun | P2 | `TestCachedProviderAsyncCancellationDoesNotCache` did not explicitly assert that cancellation left no completed `Set` or cache entries. | Added post-`AsyncJobTester` assertions for `sets == 0` and `entries == 0`. |

Performance lane P2 findings were recorded as non-blocking follow-ups:

- Cache-key/profile construction remains allocation-heavy on warm hits.
- Cold-miss benchmark includes adapter/cache construction.
- Parallel hot-path and counter-reporting benchmarks can be expanded later.

## Affected-Lane Rerun

| Tier | Perspective | Agent | Result | P0 | P1 | P2 | P3 | Notes |
|---|---|---|---:|---:|---:|---:|---:|---|
| 2 | Stability | `019ec20e-e95d-7571-86ba-9dca48fb5031` | PASS | 0 | 0 | 0 | 0 | Distributed stress coverage verified. |
| 3 | Security attempt 1 | `019ec20f-01c1-73b2-b131-5c5ab270691d` | TIMEOUT | N/A | N/A | N/A | N/A | Closed after the 10-minute SLA; retry rule applied. |
| 3 | Security attempt 2 | `019ec222-2403-7ef2-95d8-06d5657880cb` | COMMENT | 0 | 0 | 1 | 0 | P1 resolved; P2 fixed by main integration. |
| 4 | Operator/Ops | `019ec20f-1794-7870-b8db-680e92539845` | PASS | 0 | 0 | 0 | 0 | Self-contained distributed example and diagram label verified. |
| 6 | User/Caller | `019ec20f-3977-78c3-b3e3-2ea3fd444ead` | PASS | 0 | 0 | 0 | 1 | Trust-scope docs fixed; new Redis API-name P3 was fixed by main. |

## Timeout Retry And Main Integration

Security rerun attempt 1 did not return before the enforced wait limit and was
closed. Per the timeout retry rule, the Security lane was rerun instead of
being left as a final timeout/fallback. Attempt 2 completed with P0=0 and P1=0,
so final main-session fallback was not needed for the Security lane.

Main integration still rechecked the Security evidence and fixed the attempt 2
P2 test-proof gap.

Security evidence:

- `jwt/cached_provider_test.go` covers malformed, wrong-key, and wrong-algorithm
  non-caching.
- `jwt/cached_distributed_provider_test.go` covers distributed malformed,
  wrong-key, and wrong-algorithm non-caching.
- `jwt/cached_provider_test.go` covers local TTL clipping and expired-token
  no-cache.
- `jwt/cached_distributed_provider_test.go` covers distributed TTL clipping and
  expired-token no-cache.
- `jwt/cached_provider_test.go` covers local stale cached Reader delete/reparse
  branches for nil, wrong algorithm, unknown kid, and expired-key no-recache.
- `jwt/cached_distributed_provider_test.go` covers distributed stale cached
  Reader delete/reparse branches for nil, wrong algorithm, and unknown kid.
- `jwt/cache_key.go` computes cache TTL as the minimum of max TTL, token expiry,
  and key expiry, and returns non-positive TTLs without setting the cache.
- `jwt/cache_failure_test.go` now asserts that async cancellation does not
  complete a `Set` and leaves no local cache entry.

No remaining P0/P1 security issue is visible after retry plus main integration
review.

## Validation Evidence

- `node scripts/generate-jwt-provider-cache-adapter-diagram.mjs`
  - PASS: `nodes=11 routes=9 segments=18 badEndpointAngle=0 badBends=0 interiorCrossings=0 nodeOverlaps=0 laneClearance=0 marginImbalance=0 margins=L48/R48/T48/B48 titleGap=58`
- `find docs/images/readme-diagrams -name 'jwt-provider-cache-adapter-flow*.svg' -print0 | xargs -0 -n1 xmllint --noout`
  - PASS
- `find docs/images/readme-diagrams -name 'jwt-provider-cache-adapter-flow*.svg' -exec sh -c 'test -f "${1%.svg}.png"' sh {} \;`
  - PASS
- `rg 'docs/images/readme-diagrams/.*\.svg' README*.md jwt/README*.md && exit 1 || true`
  - PASS: README files embed PNG, not SVG.
- `git diff --check`
  - PASS
- `go test -count=1 ./jwt -run 'CachedProviderAsyncCancellationDoesNotCache|TTLClipping|StaleHits|ParseFailureDoesNotCache|CachedDistributedProviderColdBurst|CachedDistributedProviderStress'`
  - PASS after Security attempt 2 P2 fix, `ok github.com/bluetape4k/bluetape-go/jwt 0.602s`
- `go test -count=1 ./jwt`
  - PASS, `ok github.com/bluetape4k/bluetape-go/jwt 14.038s`
- `go test -race -count=1 ./jwt ./cache ./testing/concurrency`
  - PASS after Security attempt 2 P2 fix: `jwt` 14.864s, `cache` 2.027s, `testing/concurrency` 1.632s
- `go test -run '^$' -bench 'RedisCachedDistributedProviderParseContextWarmHit|RedisDistributedProviderParseContext|Cached|CacheKey|Parse' -benchmem ./jwt`
  - PASS
  - `BenchmarkCachedProviderParseHMACWarmHit`: 525.6 ns/op, 1400 B/op, 14 allocs/op
  - `BenchmarkCachedDistributedProviderParseHMACWarmHit`: 606.8 ns/op, 1537 B/op, 16 allocs/op
  - `BenchmarkRedisDistributedProviderParseContext`: 237446 ns/op, 4896 B/op, 91 allocs/op
  - `BenchmarkRedisCachedDistributedProviderParseContextWarmHit`: 222352 ns/op, 2712 B/op, 38 allocs/op
- `make ci`
  - PASS after Security attempt 2 P2 fix.
  - Includes `vet`, `lint`, and repository package tests, including
    Testcontainers-backed Redis/Kafka/MySQL/NATS/Postgres packages.

Stale evidence note:

- A pre-fix `go test -count=1 ./jwt` process timed out in
  `TestCachedProviderAsyncCancellationDoesNotCache` after the test double
  shared one `setBlock` channel across concurrent `AsyncJobTester` workers.
  The test was fixed by bounding the async tester to one worker and adding a
  context-aware wait on `setBlock`. Fresh targeted and full tests passed after
  that fix.
- The first two `make ci` attempts failed before this final pass:
  - `vet` reported an uncovered cancel path in the async cancellation test.
  - `lint` reported SA1012 for intentional nil-context contract tests.
  Both were fixed before the final successful `make ci`.

## Main Integration Verdict

APPROVE.

Final gate:

- P0 = 0
- P1 = 0

P2/P3 items remaining after this gate are non-blocking follow-ups. The blocker
gate for Step 6-R is closed.

## DoD

| Item | Status |
|---|---|
| Six independent lanes run | Done |
| Main integration review run | Done |
| Completed agents closed | Done |
| Timeout retry/fallback policy applied | Done |
| P0/P1 findings fixed or retry-reviewed | Done |
| Goroutine stress tester used for local and distributed cache concurrency | Done |
| AsyncJobTester used for cancellation-aware cache operation | Done |
| Diagram regenerated and validated | Done |
| Convergence P0=0 P1=0 | Done |
