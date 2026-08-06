# Issue #107 Cache Benchmark Suite Research

Issue: #107
Milestone: 0.3.0
Date: 2026-06-04
Work type: Type A - Benchmark Suite

## Research Question

normal CI를 느리게 만들지 않으면서 future adapter decision을 repeatable evidence에 기반하게 하려면 `bluetape-go`가
0.3.0 local cache와 Redis Pub/Sub NearCache path를 어떻게 benchmark해야 하는가?

## 현재 결정

`cache`와 `cache/redisnear`에 package-local Go benchmark를 둔다. 이 benchmark는 opt-in `make bench-cache` target으로
노출하고, result는 production ranking이 아니라 비교 가능한 local snapshot으로 유지한다.

## Evidence

| Evidence | Observation | Decision impact |
|---|---|---|
| `cache.Memory` tests | 기존 test가 TTL, miss, same-key collapse, different-key flight behavior, cancellation을 이미 덮는다. | benchmark는 현재 package internal을 재사용하고 production hook을 피할 수 있다. |
| `cache/redisnear` tests | Testcontainers가 Pub/Sub peer invalidation, outage handling, stress behavior를 이미 증명한다. | Redis benchmark도 같은 strategy를 쓰고 serial/opt-in으로 남겨야 한다. |
| `compression` benchmark pattern | 기존 benchmark는 package test 옆에 있으며 Makefile opt-in target을 가진다. | cache benchmark도 같은 Go-native structure를 사용한다. |
| #110 RESP3 research | RESP3 `CLIENT TRACKING`은 future strategy boundary로 남는다. | #107에서는 RESP3를 benchmark하지 않는다. |

## Benchmark Commands

```bash
go test -run '^$' -bench '^BenchmarkMemory' -benchmem ./cache
go test -run '^$' -bench '^BenchmarkNearCache' -benchmem ./cache/redisnear
make bench-cache
```

`make ci`는 `bench-cache`를 호출하면 안 된다.

## Environment Notes

- local memory benchmark에는 외부 service가 필요 없다.
- Redis NearCache benchmark는 Testcontainers로 Redis 7.4를 시작하므로 Docker가 필요하다.
- Testcontainers-backed benchmark command는 worktree 간 serial로 실행해야 한다.
- 짧은 local `-benchtime` run은 PR evidence에는 유용하지만 production capacity number로 취급하지 않는다.

## Planned Metrics

- standard Go `ns/op`, `B/op`, `allocs/op`.
- `GetOrLoad` concurrency scenario에서 loader collapse 또는 invalidation pressure를 드러내기 위한 `loads/op`.
- publish-to-peer-evict latency를 위한 peer invalidation benchmark `ns/op`.

## Sample Results

local smoke run:

- Date: 2026-06-04
- Host: macOS arm64, Apple M4 Pro
- Benchtime: `100ms`
- Purpose: PR validation snapshot, not production ranking

### `cache.Memory`

Command:

```bash
go test -run '^$' -bench '^BenchmarkMemory' -benchtime=100ms -benchmem ./cache
```

Selected output:

| Benchmark | ns/op | B/op | allocs/op | Extra |
|---|---:|---:|---:|---:|
| `BenchmarkMemoryGetHit-12` | 40.67 | 0 | 0 |  |
| `BenchmarkMemoryGetMiss-12` | 10.21 | 0 | 0 |  |
| `BenchmarkMemorySet-12` | 45.63 | 0 | 0 |  |
| `BenchmarkMemoryDeleteExisting-12` | 73.93 | 0 | 0 |  |
| `BenchmarkMemoryTTLExpiredGet-12` | 47.21 | 0 | 0 |  |
| `BenchmarkMemoryGetOrLoadHot-12` | 50.23 | 16 | 1 |  |
| `BenchmarkMemoryGetOrLoadCold-12` | 1006 | 780 | 10 | `1.000 loads/op` |
| `BenchmarkMemoryGetOrLoadSameKeyConcurrent-12` | 9331 | 4200 | 57 | `1.000 loads/op` |
| `BenchmarkMemoryGetOrLoadDifferentKeysConcurrent-12` | 27573 | 15043 | 183 | `16.00 loads/op` |

### Redis Pub/Sub NearCache

Command:

```bash
go test -run '^$' -bench '^BenchmarkNearCache' -benchtime=100ms -benchmem ./cache/redisnear
```

Selected output:

| Benchmark | ns/op | B/op | allocs/op | Extra |
|---|---:|---:|---:|---:|
| `BenchmarkNearCacheGetLocalHit-12` | 63.97 | 16 | 1 |  |
| `BenchmarkNearCacheGetLocalMiss-12` | 32.45 | 16 | 1 |  |
| `BenchmarkNearCacheSetPublish-12` | 242127 | 1208 | 29 |  |
| `BenchmarkNearCacheDeletePublish-12` | 244146 | 1224 | 29 |  |
| `BenchmarkNearCacheClearPublish-12` | 298150 | 1161 | 28 |  |
| `BenchmarkNearCachePeerInvalidation-12` | 464896 | 2617 | 58 |  |
| `BenchmarkNearCacheGetOrLoadUnderInvalidation-12` | 326.5 | 44 | 2 | `0.005997 loads/op` |

## 해석 경계

이 benchmark suite는 이 repository 안에서 behavior가 측정 가능하고 비교 가능한지 답한다. 외부 cache dependency를 ranking하지
않고, production Redis latency를 증명하지 않으며, application deployment의 load test를 대체하지 않는다.
