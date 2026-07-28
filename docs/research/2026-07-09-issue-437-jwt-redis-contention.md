# Issue #437 JWT Redis Contention Benchmark Evidence

Issue #437는 #173 lesson에서 남은 distributed JWT Redis/provider contention benchmark follow-up을 닫는다. 변경은 benchmark row만
추가하며 signing semantics, key retention, Redis storage format, provider cache contract는 바꾸지 않는다.

## Artifacts

- local default benchmark: `docs/research/outputs/issue-437/jwt-local-bench.txt`
- Redis/Testcontainers opt-in benchmark: `docs/research/outputs/issue-437/jwt-redis-bench.txt`
- environment 및 Docker metadata: `docs/research/outputs/issue-437/environment.md`

## Commands

- local/default: `go test -run '^$' -bench . -benchmem ./jwt`
- Redis/Testcontainers, serial and opt-in:
  `BLUETAPE_JWT_REDIS_BENCH=1 go test -p 1 -run '^$' -bench '^BenchmarkRedis' -benchtime=100x -benchmem ./jwt`

Redis benchmark는 Docker와 `redis:7.4-alpine`이 필요하다. normal benchmark run은 `BLUETAPE_JWT_REDIS_BENCH=1`이 없으면
Redis/Testcontainers row를 skip한다.

## Local Provider Contention Rows

| Case | Result |
|---|---:|
| `BenchmarkProviderFindKeyChainParallel` | `98.95 ns/op`, `0 B/op`, `0 allocs/op` |
| `BenchmarkProviderFindRetainedKeyParallel` | `98.44 ns/op`, `0 B/op`, `0 allocs/op` |
| `BenchmarkProviderComposeParseHMACParallel` | `2513 ns/op`, `7619 B/op`, `117 allocs/op` |
| `BenchmarkProviderForcedRotateParallel` | `1618 ns/op`, `8482 B/op`, `9 allocs/op` |
| `BenchmarkCachedProviderParseHMACWarmHitParallel` | `353.5 ns/op`, `1176 B/op`, `15 allocs/op` |
| `BenchmarkCachedDistributedProviderParseHMACWarmHitParallel` | `367.7 ns/op`, `1324 B/op`, `17 allocs/op` |

해석: local concurrent key lookup과 retained-key lookup은 allocation-free다. cached warm-hit provider read는
compose-plus-parse 및 forced-rotation path보다 훨씬 저렴하다.

## Redis Provider Contention Rows

| Case | Result |
|---|---:|
| `BenchmarkRedisRepositoryFindParallel` | `68205 ns/op`, `8101 B/op`, `39 allocs/op` |
| `BenchmarkRedisRepositoryFindRetainedParallel` | `64362 ns/op`, `8020 B/op`, `38 allocs/op` |
| `BenchmarkRedisRepositoryForcedRotateParallel` | `110602 ns/op`, `10546 B/op`, `75 allocs/op` |
| `BenchmarkRedisDistributedProviderComposeContextParallel` | `92150 ns/op`, `12514 B/op`, `103 allocs/op` |
| `BenchmarkRedisDistributedProviderParseContextParallel` | `55988 ns/op`, `11698 B/op`, `105 allocs/op` |
| `BenchmarkRedisCachedDistributedProviderParseContextWarmHitParallel` | `74058 ns/op`, `9274 B/op`, `53 allocs/op` |

해석: Redis contention row는 direct key lookup, retained key lookup, forced rotation, compose, parse, cached provider
warm-hit path를 덮는다. `-benchtime=100x` run은 comparable local snapshot이지 production capacity guidance가 아니다.

## 결정

이 run에서 optimization issue를 열지 않는다. measured row는 contention benchmark evidence gap을 닫지만 JWT security, key
retention, cache trust scope, Redis storage contract를 바꿀 correctness 또는 API 문제를 보이지 않는다.
