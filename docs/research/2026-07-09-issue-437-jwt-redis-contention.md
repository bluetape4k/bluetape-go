# Issue #437 JWT Redis Contention Benchmark Evidence

Issue #437 closes the remaining distributed JWT Redis/provider contention
benchmark follow-up from the #173 lesson. The change adds benchmark rows only;
signing semantics, key retention, Redis storage format, and provider cache
contracts are unchanged.

## Artifacts

- Local default benchmark:
  `docs/research/outputs/issue-437/jwt-local-bench.txt`
- Redis/Testcontainers opt-in benchmark:
  `docs/research/outputs/issue-437/jwt-redis-bench.txt`
- Environment and Docker metadata:
  `docs/research/outputs/issue-437/environment.md`

## Commands

- Local/default:
  `go test -run '^$' -bench . -benchmem ./jwt`
- Redis/Testcontainers, serial and opt-in:
  `BLUETAPE_JWT_REDIS_BENCH=1 go test -p 1 -run '^$' -bench '^BenchmarkRedis' -benchtime=100x -benchmem ./jwt`

Redis benchmarks require Docker and `redis:7.4-alpine`. Normal benchmark runs
skip Redis/Testcontainers rows unless `BLUETAPE_JWT_REDIS_BENCH=1` is set.

## Local Provider Contention Rows

| Case | Result |
|---|---:|
| `BenchmarkProviderFindKeyChainParallel` | `98.95 ns/op`, `0 B/op`, `0 allocs/op` |
| `BenchmarkProviderFindRetainedKeyParallel` | `98.44 ns/op`, `0 B/op`, `0 allocs/op` |
| `BenchmarkProviderComposeParseHMACParallel` | `2513 ns/op`, `7619 B/op`, `117 allocs/op` |
| `BenchmarkProviderForcedRotateParallel` | `1618 ns/op`, `8482 B/op`, `9 allocs/op` |
| `BenchmarkCachedProviderParseHMACWarmHitParallel` | `353.5 ns/op`, `1176 B/op`, `15 allocs/op` |
| `BenchmarkCachedDistributedProviderParseHMACWarmHitParallel` | `367.7 ns/op`, `1324 B/op`, `17 allocs/op` |

Interpretation: local concurrent key lookup and retained-key lookup are
allocation-free. Cached warm-hit provider reads stay materially cheaper than
compose-plus-parse and forced-rotation paths.

## Redis Provider Contention Rows

| Case | Result |
|---|---:|
| `BenchmarkRedisRepositoryFindParallel` | `68205 ns/op`, `8101 B/op`, `39 allocs/op` |
| `BenchmarkRedisRepositoryFindRetainedParallel` | `64362 ns/op`, `8020 B/op`, `38 allocs/op` |
| `BenchmarkRedisRepositoryForcedRotateParallel` | `110602 ns/op`, `10546 B/op`, `75 allocs/op` |
| `BenchmarkRedisDistributedProviderComposeContextParallel` | `92150 ns/op`, `12514 B/op`, `103 allocs/op` |
| `BenchmarkRedisDistributedProviderParseContextParallel` | `55988 ns/op`, `11698 B/op`, `105 allocs/op` |
| `BenchmarkRedisCachedDistributedProviderParseContextWarmHitParallel` | `74058 ns/op`, `9274 B/op`, `53 allocs/op` |

Interpretation: Redis contention rows now cover direct key lookup, retained key
lookup, forced rotation, compose, parse, and cached provider warm-hit paths.
The `-benchtime=100x` run is a comparable local snapshot, not production
capacity guidance.

## Decision

No optimization issue is opened from this run. The measured rows close the
contention benchmark evidence gap, but they do not show a correctness or API
problem that warrants changing JWT security, key retention, cache trust scope,
or Redis storage contracts.

