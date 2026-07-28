# Issue #173 Task 8 Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #173
Task: Benchmarks and Chart Asset Gate
날짜: 2026-06-12

## 범위

- `jwt/redis_benchmark_test.go`
- `docs/research/outputs/issue-173/distributed-jwt-redis-bench.txt`
- `docs/images/readme-charts/generate-distributed-jwt-redis-benchmark.mjs`
- `docs/images/readme-charts/distributed-jwt-redis-benchmark.vl.json`
- `docs/images/readme-charts/distributed-jwt-redis-benchmark.svg`
- `docs/images/readme-charts/distributed-jwt-redis-benchmark.png`

## Benchmark Evidence

| Command | Evidence | Status |
| --- | --- | --- |
| `go test -p 1 -run '^$' -bench 'BenchmarkRedis(Repository\|Distributed)' -benchtime=100ms -benchmem ./jwt` | Smoke benchmark produced `ns/op`, `B/op`, and `allocs/op` for six Redis repository/provider rows. | PASS |
| `mkdir -p docs/research/outputs/issue-173 && go test -p 1 -run '^$' -bench 'BenchmarkRedis(Repository\|Distributed)' -benchtime=100ms -benchmem ./jwt \| tee docs/research/outputs/issue-173/distributed-jwt-redis-bench.txt` | Raw output stored for PR/docs evidence. | PASS |

Stored benchmark snapshot:

| Benchmark | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| `BenchmarkRedisRepositoryFind` | 239,841 | 1,224 | 24 |
| `BenchmarkRedisRepositoryRotateCurrentHit` | 243,367 | 1,840 | 35 |
| `BenchmarkRedisRepositoryRotateExpired` | 479,239 | 5,507 | 96 |
| `BenchmarkRedisRepositoryForcedRotate` | 311,732 | 3,621 | 60 |
| `BenchmarkRedisDistributedProviderComposeContext` | 248,355 | 5,580 | 88 |
| `BenchmarkRedisDistributedProviderParseContext` | 242,631 | 4,880 | 91 |

## Chart Evidence

| Requirement | Evidence | Status |
| --- | --- | --- |
| Chart is a real bar chart, not a table or heatmap. | `distributed-jwt-redis-benchmark.png` has three horizontal bar panels for `ns/op`, `B/op`, and `allocs/op`. | PASS |
| Benchmark names appear on category axis. | Six operation labels appear on the left of each panel. | PASS |
| Latency and allocation metrics use separate scales. | Separate panels for latency, heap bytes, and allocation count. | PASS |
| Unit and direction are visible. | Each panel says `<unit>; lower is better`. | PASS |
| Raw output remains numeric source of truth. | Footer says to use raw benchmark output as the numeric source. | PASS |
| Vega-Lite chart source is valid JSON. | `node -e 'JSON.parse(...)'` passed for `distributed-jwt-redis-benchmark.vl.json`. | PASS |
| SVG parses. | `xmllint --noout docs/images/readme-charts/distributed-jwt-redis-benchmark.svg` passed. | PASS |
| PNG renders from SVG. | `rsvg-convert ... -o distributed-jwt-redis-benchmark.png` passed; `file` reports `PNG image data, 1280 x 1240`. | PASS |
| Rendered PNG was visually inspected. | Panel overlap found and fixed twice; final PNG has no panel, row, axis, value-label, or footer overlap. | PASS |
| Whitespace check passes. | `git diff --check` passed. | PASS |

## Command Budget Notes

- `Find`: one `HGET` against `keys`, plus local DTO decode.
- `Current`: one current-pointer read plus one `HGET`.
- `Rotate` current-hit: one Lua read phase, no `create` call.
- `Rotate` empty/expired: one Lua read phase, one provider `create`, one Lua/CAS store phase.
- `ForcedRotate`: one provider `create`, one Lua/store phase.

## 판정

P0=0 P1=0

Task 8 verdict: PASS
