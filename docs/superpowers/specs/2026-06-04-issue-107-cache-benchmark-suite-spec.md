# Issue 107 Cache Benchmark Suite Spec

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

Issue: #107
Milestone: 0.3.0
Date: 2026-06-04
Work type: Type A - Full Feature

## Problem

`cache.Memory` and `cache/redisnear.NewPubSub` now provide the 0.3.0 cache
baseline, but adapter decisions still lack repeatable benchmark evidence. The
benchmark suite must measure local cache behavior and Redis near-cache
invalidation paths without making normal CI slower or turning benchmarks into
production code.

## 목표s

- Add opt-in Go benchmarks for `cache.Memory`.
- Add opt-in Go benchmarks for Redis Pub/Sub `NearCache`.
- Cover cache hit, miss, set, delete, TTL expiry, and `GetOrLoad` hot/cold
  paths.
- Cover same-key concurrent `GetOrLoad` collapse and different-key independent
  loading.
- Cover Redis NearCache local hit/miss, `Set`/`Delete`/`Clear` publish paths,
  peer invalidation latency, and concurrent `GetOrLoad` under invalidation
  pressure.
- Add an explicit command target so benchmark execution stays outside
  `make ci`.
- Record benchmark commands, Redis/Testcontainers notes, and sample local
  results under `docs/research` for GNO search.

## Non-Goals

- Do not add Ristretto, BigCache, or another local-cache dependency.
- Do not change production cache APIs.
- Do not add benchmark execution to `make ci`.
- Do not claim production rankings from a short local benchmark run.
- Do not benchmark RESP3 `CLIENT TRACKING`; #110 keeps that strategy deferred.

## Current Evidence

- `cache.Memory` already supports `Get`, `Set`, `Delete`, `Clear`, TTL expiry,
  and same-key `GetOrLoad` through `singleflight`.
- `cache/redisnear.NearCache` delegates local reads/loads to a local
  `cache.LoadingCache` and publishes Redis Pub/Sub invalidations on `Set`,
  `Delete`, and `Clear`.
- Existing Redis tests use Redis 7.4 Testcontainers and must not run in parallel
  across worktrees.
- Existing compression benchmarks use standard Go `Benchmark*` functions and a
  Makefile opt-in target, which is the closest repo-local benchmark pattern.

## Approach Options

| Option | Shape | Pros | Cons | Decision |
|---|---|---|---|---|
| A. PR body only | Add benchmarks and report commands only in the PR body. | Smallest docs diff. | Not durable or GNO-searchable after merge. | Rejected. |
| B. Dedicated benchmark package | Add a separate benchmark module/package. | Clear separation. | Go repo has no benchmark module convention; would add module boundaries without need. | Rejected. |
| C. Package-local `*_benchmark_test.go` | Add benchmarks beside current tests and keep execution behind `-bench`/Makefile target. | Go-native, compiles with package tests, no production API, matches compression benchmark style. | Testcontainers benchmark code lives in the package test harness. | Selected. |

## Benchmark Design

### `cache.Memory`

Benchmarks should run in `cache` package scope so the TTL expiry path can use a
deterministic test clock without exporting new hooks.

Required scenarios:

- `BenchmarkMemoryGetHit`
- `BenchmarkMemoryGetMiss`
- `BenchmarkMemorySet`
- `BenchmarkMemoryDeleteExisting`
- `BenchmarkMemoryTTLExpiredGet`
- `BenchmarkMemoryGetOrLoadHot`
- `BenchmarkMemoryGetOrLoadCold`
- `BenchmarkMemoryGetOrLoadSameKeyConcurrent`
- `BenchmarkMemoryGetOrLoadDifferentKeysConcurrent`

Concurrency benchmarks should report `loads/op` so the benchmark result shows
whether loader collapse or independent loading happened. Same-key collapse
should target one loader per round; different-key loading should target one
loader per worker/key per round.

### Redis Pub/Sub NearCache

Redis benchmarks should run in `cache/redisnear` package scope and start Redis
with Testcontainers only when the benchmark is invoked.

Required scenarios:

- `BenchmarkNearCacheGetLocalHit`
- `BenchmarkNearCacheGetLocalMiss`
- `BenchmarkNearCacheSetPublish`
- `BenchmarkNearCacheDeletePublish`
- `BenchmarkNearCacheClearPublish`
- `BenchmarkNearCachePeerInvalidation`
- `BenchmarkNearCacheGetOrLoadUnderInvalidation`

Peer invalidation must measure the whole publish-to-peer-evict path by priming
the peer cache, publishing through the other cache, and waiting until the peer
returns `cache.ErrCacheMiss`. The benchmark should use bounded waiting and fail
instead of silently reporting invalid data.

`GetOrLoad` under invalidation pressure should run an invalidator loop against
shared keys while parallel readers call `GetOrLoad`; it should report loader
calls per operation as a stability signal.

## Command Surface

Add `make bench-cache` as an opt-in target:

```bash
go test -run '^$' -bench '^BenchmarkMemory' -benchmem ./cache
go test -run '^$' -bench '^BenchmarkNearCache' -benchmem ./cache/redisnear
```

The target must not be part of `make ci`.

## Documentation

Create `docs/research/2026-06-04-issue-107-cache-benchmark-suite.md` with:

- benchmark purpose and scope;
- commands;
- environment notes, including Redis/Testcontainers dependency;
- sample local results;
- interpretation boundaries and follow-up notes.

Update the research index and the 0.3.0 milestone research note so GNO can find
the benchmark work from milestone and issue terms.

## 위험

- Redis/Testcontainers benchmarks can be noisy and slow; report them as local
  snapshots only.
- Peer invalidation benchmarks can hide broken invalidation if the benchmark
  does not assert the final miss state.
- Same-key concurrency benchmarks can accidentally become hot-cache benchmarks
  if they do not clear per round and report loader calls.
- Benchmarks compile during normal `go test`; helper code must avoid starting
  containers at package load time.
- Benchmark helper code can duplicate test fixture code; keep it local unless a
  broader `testing.TB` fixture change is explicitly needed.

## Acceptance Criteria Mapping

| Requirement | Spec coverage |
|---|---|
| cache hit/miss/set/delete/TTL/GetOrLoad hot/cold | `BenchmarkMemory*` scenarios |
| same-key concurrent `GetOrLoad` collapse | `BenchmarkMemoryGetOrLoadSameKeyConcurrent` with `loads/op` |
| different-key concurrent scaling | `BenchmarkMemoryGetOrLoadDifferentKeysConcurrent` with `loads/op` |
| Redis NearCache benchmark coverage | `BenchmarkNearCache*` scenarios |
| commands, environment notes, sample results | `docs/research/2026-06-04-issue-107-cache-benchmark-suite.md` |
| opt-in and outside CI | `make bench-cache`; no `ci` dependency |

## Step 1 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Target repository confirmed | Done | `bluetape4k/bluetape-go`, branch `bench/issue-107-cache-suite`. |
| Worktree created | Done | `.worktrees/bench-issue-107-cache-suite` from `origin/develop`. |
| Issue inspected | Done | #107 benchmark suite requirements and #23 dependency checked. |
| User intent clear | Done | Work one issue at a time through workflow, create PR, request merge approval. |
| Review-only boundary | N/A | User requested implementation work. |

## Step 1-R Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Current repo checked | Done | `cache.Memory`, `cache/redisnear`, Redis Testcontainers tests, Makefile, compression benchmark pattern. |
| GNO context checked | Done | `bluetape4k-github` #107/#3/#106 and `bluetape4k-docs` milestone context queried. |
| External API checked | Done | Standard Go `testing` benchmark behavior verified through local Go tool usage; no new external dependency. |
| Adopt/borrow/skip decisions recorded | Done | Borrow package-local benchmark style; skip new dependency and production API hooks. |
| Technical constraints identified | Done | Testcontainers benchmarks serial; benchmarks opt-in; docs under `docs/research` for GNO. |
