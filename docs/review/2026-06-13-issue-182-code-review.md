# Issue #182 Step 6-R Code Review

Scope: `origin/develop...HEAD`

Head reviewed after repair commit: `c57056e`

Subject: Redis-backed probabilistic Bloom filters.

## Gate Model

7-Tier gate was executed as six independent subagent lanes plus this main
integration review. No seventh integration subagent was used.

| Lane | Initial Result | Final Result | Notes |
|---|---:|---:|---|
| Tier 1 Performance | P0=0 P1=1 P2=2 FAIL | P0=0 P1=0 P2=0 PASS | Added low/medium/high FPP benchmark matrix, removed `GETBIT` from `Put` Lua, and reduced offset allocations with `AppendIndexes`. |
| Tier 2 Stability | P0=0 P1=0 P2=1 PASS | P0=0 P1=0 P2=0 PASS | Replaced nil example placeholders; `gopls check` is clean. |
| Tier 3 Security | P0=0 P1=0 P2=0 PASS | P0=0 P1=0 P2=0 PASS | Namespace validation, static Lua, redacted errors, and ACL/TLS docs reviewed. |
| Tier 4 Operator/Ops | P0=0 P1=2 P2=2 FAIL | P0=0 P1=0 P2=0 PASS | Added `allkeys-*` eviction/data-loss caveat, `evicted_keys`, `EXISTS`/`PTTL`, Docker/Testcontainers test docs, and fixed `v0.6.0` README drift. |
| Tier 5 Developer/API | P0=0 P1=0 P2=3 PASS | P0=0 P1=0 P2=0 PASS | Addressed README test commands, changelog wording, and generator font discovery. |
| Tier 6 User/Caller | P0=0 P1=1 P2=1 FAIL | P0=0 P1=0 P2=0 PASS | Replaced nil examples and clarified Redis package tests plus Docker requirement. |

## Repaired Findings

- Performance P1: benchmark coverage now spans `fpp_0.100`, `fpp_0.010`, and
  `fpp_0.001` for `Put`, `MightContain`, and `Offsets`.
- Performance P2: `Put` Lua uses `SETBIT` return values instead of
  `GETBIT` plus `SETBIT`.
- Performance P2: offset argument preparation now uses
  `bloomhash.AppendIndexes` and integer Redis script arguments.
- Stability/User/Ops P1: examples no longer call methods on nil interfaces.
- Ops P1: README warns that `allkeys-*` eviction or external bitmap deletion
  voids the no-false-negative guarantee until rebuild.
- Ops P2: root README keeps published `v0.6.0` in-memory scope separate from
  Unreleased `0.6.1` Redis Bloom scope.
- User/Developer P2: package README test commands include
  `./probabilistic/redis` and Docker/Testcontainers requirements.
- Developer P2: changelog says compile-checked examples, not runnable examples.
- Developer P2: diagram generator discovers fonts through env overrides and
  common paths instead of a single hardcoded local path.

## Commands Run

```bash
gopls check probabilistic/redis/example_test.go probabilistic/redis/filter.go probabilistic/internal/bloomhash/indexes.go probabilistic/redis/filter_benchmark_test.go
go test -count=1 ./probabilistic ./probabilistic/internal/bloomhash
go test -p 1 -count=1 ./probabilistic/redis
go test -race -count=1 ./probabilistic ./probabilistic/internal/bloomhash
go test -p 1 -race -count=1 ./probabilistic/redis
go test -p 1 -run '^$' -bench 'BenchmarkRedisBloom(Put|MightContain|Offsets)' -benchmem ./probabilistic/redis
node scripts/generate-redis-bloom-diagram.mjs
file docs/images/readme-diagrams/redis-bloom-key-layout-01.png docs/images/readme-diagrams/redis-bloom-key-layout-01-graphviz.png
xmllint --noout docs/images/readme-diagrams/redis-bloom-key-layout-01.svg docs/images/readme-diagrams/redis-bloom-key-layout-01-graphviz.svg
rg -n "probabilistic/redis|Redis Bloom|redis-bloom-key-layout-01.png|Put\(ctx, value\).*false|Clear\(ctx\)|approval|authorization|rebuild|dual-write|retire old keys|ErrConfigMismatch|ErrConfigCorrupt|errors\.Is|errors\.As|PTTL|EVALSHA|TLS|AUTH|ACL|GETBIT|SETBIT|BITCOUNT|STRLEN|HGET|HGETALL|HSET|DEL|EVAL|EXISTS|evicted_keys|noeviction|allkeys" README.md README.ko.md probabilistic/README.md probabilistic/README.ko.md CHANGELOG.md
git diff --check
make ci
```

Benchmark evidence from the final run:

```text
BenchmarkRedisBloomPut/fpp_0.100-12              217789 ns/op    519 B/op   15 allocs/op
BenchmarkRedisBloomPut/fpp_0.010-12              208645 ns/op    647 B/op   15 allocs/op
BenchmarkRedisBloomPut/fpp_0.001-12              213386 ns/op    743 B/op   15 allocs/op
BenchmarkRedisBloomMightContain/fpp_0.100-12     217466 ns/op    518 B/op   15 allocs/op
BenchmarkRedisBloomMightContain/fpp_0.010-12     220764 ns/op    646 B/op   15 allocs/op
BenchmarkRedisBloomMightContain/fpp_0.001-12     218801 ns/op    743 B/op   15 allocs/op
BenchmarkRedisBloomOffsets/fpp_0.100-12             119.9 ns/op  119 B/op    4 allocs/op
BenchmarkRedisBloomOffsets/fpp_0.010-12             139.5 ns/op  183 B/op    4 allocs/op
BenchmarkRedisBloomOffsets/fpp_0.001-12             161.8 ns/op  231 B/op    4 allocs/op
```

## Main Integration Verdict

All P1 findings were repaired and rerun by the owning lanes. Remaining residual
risk is limited to GitHub CI/PR review, which has not run yet.

P0=0 P1=0
