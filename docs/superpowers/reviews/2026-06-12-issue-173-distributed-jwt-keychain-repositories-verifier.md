# Issue #173 Verifier: Redis Distributed JWT KeyChain Repositories

Gate: PASS

- P0: 0
- P1: 0
- P2: 0

## Validation Commands

| Command | Result |
| --- | --- |
| `gofmt -w jwt/*.go jwt/redis/*.go` | PASS |
| `go test -p 1 -count=1 ./jwt ./jwt/redis` | PASS: `jwt` 12.367s, `jwt/redis` 0.271s after lint fixes |
| `go test -race -p 1 -count=1 ./jwt ./jwt/redis` | PASS: `jwt` 12.972s, `jwt/redis` 1.432s |
| `go test -p 1 -run '^$' -bench 'BenchmarkRedis(Repository\|Distributed)' -benchtime=100ms -benchmem ./jwt` | PASS |
| `go test -p 1 -count=1 ./...` | PASS: all repo packages |
| `golangci-lint config verify` | PASS |
| `golangci-lint run ./jwt/...` | PASS: `0 issues` |
| `go test -p 1 -count=1 ./ratelimit/redis -run TestLimiterRefillsFromRedisServerTime` | PASS after one unrelated transient `make ci` failure in `ratelimit/redis` |
| `make ci` | PASS on retry |
| `git diff --check` | PASS |

## Benchmark Snapshot

Raw output: `docs/research/outputs/issue-173/distributed-jwt-redis-bench.txt`

| Benchmark | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| `BenchmarkRedisRepositoryFind` | 239841 | 1224 | 24 |
| `BenchmarkRedisRepositoryRotateCurrentHit` | 243367 | 1840 | 35 |
| `BenchmarkRedisRepositoryRotateExpired` | 479239 | 5507 | 96 |
| `BenchmarkRedisRepositoryForcedRotate` | 311732 | 3621 | 60 |
| `BenchmarkRedisDistributedProviderComposeContext` | 248355 | 5580 | 88 |
| `BenchmarkRedisDistributedProviderParseContext` | 242631 | 4880 | 91 |

Chart assets:

- `docs/images/readme-charts/distributed-jwt-redis-benchmark.svg`
- `docs/images/readme-charts/distributed-jwt-redis-benchmark.png`
- `docs/images/readme-charts/distributed-jwt-redis-benchmark.vl.json`

Chart evidence:

```text
chart gate: rows=6 panels=3 bars=18
PNG image data, 1280 x 1240, 8-bit/color RGBA, non-interlaced
```

Visual inspection confirmed the chart is a three-panel horizontal bar chart
with readable axes, values, and no overlapping panel labels.

## Acceptance Matrix

| Acceptance | Evidence | Status |
| --- | --- | --- |
| `DistributedProvider` composes `*Provider` and adds context-aware operations. | `jwt/distributed_provider.go`; `TestDistributedProviderDoesNotExposeContextFreeProviderMethods`; `TestDistributedProviderComposeAndParseAcrossInstances`. | PASS |
| Redis supports current, find, rotate, forced rotate, trim, expiry, delete. | `jwt/redis_repository.go`; `TestRepositoryFindUsesKIDHashLookup`; `TestRepositoryRotateCASReturnsConcurrentWinner`; `TestRepositoryCapacityTrimPreservesNewestKeys`; `TestRepositoryConfiguredKeyTTLRetainsNonExpiredKeys`; `TestRepositoryDeleteAllRemovesNamespacedState`. | PASS |
| Two instances verify tokens across rotation by `kid`. | `TestRedisDistributedProviderParsesAfterForcedRotationByKID`; `TestRedisDistributedProvidersShareHMACKeysAcrossInstances`; `TestRedisDistributedProvidersShareRSAKeysAcrossInstances`. | PASS |
| Stress tests use `GoroutineStressTester`. | `jwt/distributed_provider_test.go`; `jwt/redis_repository_test.go`; `jwt/redis_integration_test.go`. | PASS |
| Redis cancellation uses `AsyncJobTester`. | `TestRepositoryRotateCanceledAfterCreateDoesNotPersistCandidate`; `TestRedisDistributedProviderContextCancellationStress`; `TestRedisDistributedProviderDeadlineStress`. | PASS |
| Redis command budget is enforced. | `TestRepositoryFindUsesKIDHashLookup`; `TestRepositoryCurrentHitCommandBudget`; `TestRepositoryRotateCurrentHitCommandBudget`; forbidden command assertions reject `SCAN`, `KEYS`, `LRANGE`, `ZRANGE`, and `HGETALL`. | PASS |
| Benchmark results have chart asset when published. | Raw output, SVG, PNG, Vega-Lite JSON, generator, and visual inspection evidence above. | PASS |
| Public API does not expose raw-key repository constructors. | Redis DTO encode/decode stays package-private in `jwt/redis_dto.go`; package `jwt/redis` exposes only `Options`, `Repository`, and `New`. | PASS |
| MongoDB remains deferred. | `jwt/README.md` and `jwt/README.ko.md` link MongoDB distributed storage to #198. | PASS |
| Redis runbook covers operator boundaries. | README pair documents TLS, ACL, persistence/backups, `noeviction`, namespace diagnostics, rollback/reset, token invalidation, and secret-safe logging. | PASS |
| Inbound `kid` cannot amplify Redis lookup or leak control characters in errors. | `validateKID`, `validateLookupKID`, `TestDistributedProviderParseRejectsInvalidKIDBeforeRepositoryLookup`, Redis `Find` invalid KID tests, and quoted `KeyError` formatting. | PASS |
| Redis trim and TTL validation handle skew/latency edge cases. | `TestRepositoryCapacityTrimSkipsCandidateWithoutExceedingCapacity`; `TestRepositoryConfiguredKeyTTLUsesKeyValidityNotStoreClock`; Lua trim removes non-candidate keys until capacity is satisfied. | PASS |
| Examples avoid reusing bootstrap timeout contexts as operation contexts. | README pair and `jwt/redis/example_test.go` use `setupCtx` for constructors and `opCtx` for distributed operations. | PASS |

## Notes

`make ci` initially failed on lint issues in the new JWT examples and tests, plus
stale golangci cache references to a removed sibling worktree. The JWT issues
were fixed, `golangci-lint cache clean` was run, `golangci-lint run ./jwt/...`
returned `0 issues`, and a later `make ci` run passed.

After Step 6-R fixes, `make ci` had one unrelated transient failure in
`ratelimit/redis` (`TestLimiterRefillsFromRedisServerTime`) after `go test ./...`
had already passed. The failing test passed when rerun directly, and the final
`make ci` retry passed.
