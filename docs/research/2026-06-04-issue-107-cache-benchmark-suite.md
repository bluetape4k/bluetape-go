# Issue #107 Cache Benchmark Suite Research

Issue: #107
Milestone: 0.3.0
Date: 2026-06-04
Work type: Type A - Benchmark Suite

## Research Question

How should `bluetape-go` benchmark the 0.3.0 local cache and Redis Pub/Sub
NearCache paths so future adapter decisions are based on repeatable evidence
without slowing normal CI?

## Current Decision

Use package-local Go benchmarks in `cache` and `cache/redisnear`, expose them
through an opt-in `make bench-cache` target, and keep results as comparable
local snapshots rather than production rankings.

## Evidence

| Evidence | Observation | Decision impact |
|---|---|---|
| `cache.Memory` tests | Existing tests already cover TTL, miss, same-key collapse, different-key flight behavior, and cancellation. | Benchmarks can reuse current package internals and avoid production hooks. |
| `cache/redisnear` tests | Testcontainers already proves Pub/Sub peer invalidation, outage handling, and stress behavior. | Redis benchmarks should use the same strategy and remain serial/opt-in. |
| `compression` benchmark pattern | Existing benchmark lives beside package tests and has a Makefile opt-in target. | Use the same Go-native structure for cache benchmarks. |
| #110 RESP3 research | RESP3 `CLIENT TRACKING` remains a future strategy boundary. | Do not benchmark RESP3 in #107. |

## Benchmark Commands

```bash
go test -run '^$' -bench '^BenchmarkMemory' -benchmem ./cache
go test -run '^$' -bench '^BenchmarkNearCache' -benchmem ./cache/redisnear
make bench-cache
```

`make ci` must not call `bench-cache`.

## Environment Notes

- Local memory benchmarks need no external service.
- Redis NearCache benchmarks start Redis 7.4 with Testcontainers, so Docker
  must be available.
- Testcontainers-backed benchmark commands should be run serially across
  worktrees.
- Short local `-benchtime` runs are useful for PR evidence but should not be
  treated as production capacity numbers.

## Planned Metrics

- Standard Go `ns/op`, `B/op`, and `allocs/op`.
- `loads/op` for `GetOrLoad` concurrency scenarios to expose loader collapse or
  invalidation pressure.
- Peer invalidation benchmark `ns/op` for publish-to-peer-evict latency.

## Sample Results

Local smoke run:

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

## Interpretation Boundary

The benchmark suite answers whether behavior is measurable and comparable in
this repository. It does not rank external cache dependencies, does not prove
production Redis latency, and does not replace load testing in an application
deployment.
