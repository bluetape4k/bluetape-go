# Step 6-R Code Review: Issue #173 Redis Distributed JWT KeyChain Repositories

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

Gate verdict: PASS

- P0=0
- P1=0
- P2=0
- P3=1

This review ran as 6 independent subagent lanes plus main integration:

1. Tier 1 Performance
2. Tier 2 Stability
3. Tier 3 Security
4. Tier 4 Operator
5. Tier 5 Developer
6. Tier 6 User

## 검증 증거

Latest validation after Step 6-R fixes:

```bash
go test -p 1 -count=1 ./jwt ./jwt/redis
go test -race -p 1 -count=1 ./jwt ./jwt/redis
go test -p 1 -run '^$' -bench 'BenchmarkRedis(Repository|Distributed)' -benchtime=100ms -benchmem ./jwt
go test -p 1 -count=1 ./...
golangci-lint config verify
golangci-lint run ./jwt/...
go test -p 1 -count=1 ./ratelimit/redis -run TestLimiterRefillsFromRedisServerTime
make ci
git diff --check
```

결과: PASS. One earlier `make ci` attempt hit an unrelated transient
`ratelimit/redis` timing failure after `go test ./...` had passed; the failing
test passed directly, and the final `make ci` retry passed.

## Tier 1 Performance

Final status: PASS

Findings:

| Severity | Status | Evidence | Resolution |
| --- | --- | --- | --- |
| P3 | Open follow-up | `jwt/redis_benchmark_test.go` uses serial benchmark loops for repository/provider paths. | Current benchmark evidence covers serial latency/allocation. Parallel `b.RunParallel` benchmarks for shared Redis client/provider contention are a non-blocking follow-up. |
| P3 | Resolved | Task 8 review benchmark table was stale after raw benchmark regeneration. | `docs/superpowers/reviews/2026-06-12-issue-173-distributed-jwt-keychain-repositories-task-8-review.md` now matches `docs/research/outputs/issue-173/distributed-jwt-redis-bench.txt`. |

Counts: P0=0 P1=0 P2=0 P3=1

Gate verdict: PASS

## Tier 2 Stability

Final status: PASS

Initial findings and closure:

| Severity | Status | Evidence | Resolution |
| --- | --- | --- | --- |
| P2 | Resolved | `jwt/redis_scripts.go` trim skipped `candidate_kid` after fetching exactly `remove_count` stale members, allowing `capacity+1` under skew. | Lua trim now iterates oldest members and removes non-candidate keys until `ZCARD <= capacity`; `TestRepositoryCapacityTrimSkipsCandidateWithoutExceedingCapacity` covers the skew case. |
| P2 | Resolved | Redis TTL validation used a pre-create/store `now`, risking false rejection when key generation latency occurs. | TTL validation now uses `key.ExpiresAt().Sub(key.CreatedAt()) + RetentionLeeway`; `TestRepositoryConfiguredKeyTTLUsesKeyValidityNotStoreClock` covers the edge. |

Re-review result: 발견 사항 없음.

Counts: P0=0 P1=0 P2=0 P3=0

Gate verdict: PASS

## Tier 3 Security

Final status: PASS

Initial findings and closure:

| Severity | Status | Evidence | Resolution |
| --- | --- | --- | --- |
| P2 | Resolved | `DistributedProvider.ParseContext` accepted arbitrary string `kid` and sent it to repository lookup; errors formatted raw `kid`. | Added `maxKIDBytes`, `validateKID`, `validateLookupKID`; distributed parse rejects invalid inbound `kid` before repository lookup; Redis and in-memory lookup validate `kid`; `KeyError` quotes `kid` with `%q`. |

Re-review result: 발견 사항 없음.

Counts: P0=0 P1=0 P2=0 P3=0

Gate verdict: PASS

## Tier 4 Operator

Final status: PASS

Initial findings and closure:

| Severity | Status | Evidence | Resolution |
| --- | --- | --- | --- |
| P1 | Resolved | README quickstart configured `KeyTTL: 24 * time.Hour` while provider default key TTL is 365 days, so copy-paste bootstrap would fail. | README snippets removed unsafe `KeyTTL` and document that Redis `KeyTTL` must cover provider key lifetime plus `RetentionLeeway`. |
| P2 | Resolved | Runbook documented `meta version` but not `algorithm`, making algorithm-family namespace reuse harder to diagnose. | README pair now includes `redis-cli --tls HMGET ... version algorithm` and explains algorithm-family mismatch before reset. |

Re-review result: 발견 사항 없음.

Counts: P0=0 P1=0 P2=0 P3=0

Gate verdict: PASS

## Tier 5 Developer

Final status: PASS

발견 사항: 발견 사항 없음.

Evidence reviewed:

- `jwt/distributed_provider.go` exposes context-aware methods without
  context-free distributed operation wrappers.
- `jwt/distributed_provider_test.go` asserts `DistributedProvider` does not
  anonymously embed `*Provider`.
- `jwt/distributed_repository.go` validates nil context and typed-nil
  repositories.
- `jwt/redis_options.go` validates client, namespace, capacity, TTL, and payload
  size bounds.
- `jwt/redis/redis.go` remains a minimal facade and exposes no DTO/raw-key API.
- `jwt/redis/example_test.go` compiles.

Counts: P0=0 P1=0 P2=0 P3=0

Gate verdict: PASS

## Tier 6 User

Final status: PASS

Initial findings and closure:

| Severity | Status | Evidence | Resolution |
| --- | --- | --- | --- |
| P2 | Resolved | English README still said repository state was process-local/future #173 despite Redis distributed support being implemented. | English README now says Redis-backed context-aware distributed key storage is available and MongoDB remains deferred to #198. |
| P3 | Resolved | Examples reused one timeout context for constructor and later operations. | README pair and `jwt/redis/example_test.go` now use `setupCtx` for bootstrap and `opCtx` for distributed operations. |

Re-review result: 발견 사항 없음.

Counts: P0=0 P1=0 P2=0 P3=0

Gate verdict: PASS

## 메인 통합 검토

Integrated result:

| Severity | Count | Release gate impact |
| --- | ---: | --- |
| P0 | 0 | None |
| P1 | 0 | None |
| P2 | 0 | None |
| P3 | 1 | Follow-up only |

Main integration reviewed the lane outputs, deduplicated findings, fixed all
P1/P2 findings, reran affected lanes, and reran validation. The only remaining
finding is the non-blocking performance follow-up to add parallel benchmark
coverage for shared Redis client/provider contention.

Gate verdict: PASS
