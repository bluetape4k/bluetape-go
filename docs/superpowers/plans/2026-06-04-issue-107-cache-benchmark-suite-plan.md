# Issue 107 Cache Benchmark Suite Implementation Plan

Issue: #107
Milestone: 0.3.0
Date: 2026-06-04
Spec: `docs/superpowers/specs/2026-06-04-issue-107-cache-benchmark-suite-spec.md`

## Objective

Add repeatable, opt-in benchmark coverage for `cache.Memory` and Redis Pub/Sub
NearCache while keeping production code and normal CI unchanged.

## Task Plan

| Task | Scope | Details | Validation |
|---|---|---|---|
| T1 Memory benchmark harness | `cache/memory_benchmark_test.go` | Add hit, miss, set, delete, TTL expiry, hot/cold `GetOrLoad`, same-key concurrent collapse, and different-key concurrent load benchmarks. | `go test -run '^$' -bench '^BenchmarkMemory' -benchtime=100ms -benchmem ./cache` |
| T2 Redis benchmark harness | `cache/redisnear/near_cache_benchmark_test.go` | Add Testcontainers-backed benchmarks for local hit/miss, `Set`/`Delete`/`Clear` publish paths, peer invalidation latency, and `GetOrLoad` under invalidation pressure. | `go test -run '^$' -bench '^BenchmarkNearCache' -benchtime=100ms -benchmem ./cache/redisnear` |
| T3 Opt-in command | `Makefile` | Add `bench-cache` help text and target; do not wire it into `ci`. | `make -n bench-cache`; `rg "bench-cache|ci:" Makefile` |
| T4 Research/report update | `docs/research` | Fill #107 research note with commands, environment, sample local results, and interpretation boundary; link it from index and 0.3.0 note. | `gno update`; `gno search "issue-107-cache-benchmark-suite" -c bluetape4k-docs` after merge/local sync |
| T5 Verification | repo root | Run targeted tests, benchmark smoke runs, formatting, diff check, and full package tests for touched packages. | See validation commands below. |
| T6 Review and lessons | `docs/superpowers/reviews`, `docs/lessons` | Record implemented diff 7-Tier review and lessons before commit/PR. | Review artifact P0=0/P1=0; lessons committed. |

## Benchmark Checks

- Use `b.ReportAllocs()` in all benchmark scenarios.
- Use `b.ResetTimer()` after setup.
- Use bounded waits for peer invalidation so a broken invalidation path fails.
- Use `loads/op` custom metrics for `GetOrLoad` concurrency scenarios.
- Avoid package-level Redis/Testcontainers startup.
- Keep helper code in benchmark/test files unless production reuse is required.

## Validation Commands

Run Testcontainers-backed commands serially.

```bash
go test -count=1 ./cache
go test -count=1 ./cache/redisnear
go test -run '^$' -bench '^BenchmarkMemory' -benchtime=100ms -benchmem ./cache
go test -run '^$' -bench '^BenchmarkNearCache' -benchtime=100ms -benchmem ./cache/redisnear
make -n bench-cache
gofmt -w cache/memory_benchmark_test.go cache/redisnear/near_cache_benchmark_test.go
git diff --check
```

Optional if time and Docker stability allow:

```bash
go test -race -count=1 ./cache ./cache/redisnear
```

## Documentation and Evidence

- `docs/research/2026-06-04-issue-107-cache-benchmark-suite.md` must include
  sample results from the local benchmark smoke runs.
- PR body must state that results are local snapshots and not production
  rankings.
- README pair is not required because public runtime behavior and user-facing
  API do not change.
- CHANGELOG is not required because benchmarks do not change library behavior.

## Step 3 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Every spec requirement mapped | Done | T1-T4 map all #107 acceptance criteria. |
| Task order implementable | Done | Benchmarks first, command surface second, docs and review after measured results. |
| Testcontainers handled serially | Done | Redis benchmark/test commands are listed as serial. |
| Verification commands concrete | Done | Targeted tests, benchmark smoke runs, Makefile dry run, gofmt, diff check. |
| Public docs impact assessed | Done | README/CHANGELOG N/A because no public runtime behavior changes. |
