# Issue #599: Go-native Fory와 JSON Redis cache benchmark

## 목적과 범위

Issue #599의 목적은 현재 Go cache surface에서 JSON, Fory
`NativeFast`, Fory `NativeCompatible`를 같은 fixture와 같은 실행 조건으로
비교하는 것이다. 비교 범위는 다음 네 경로다.

- in-process codec: `Small`, `Medium`, `Repeated`, `Nil`, `Empty`의 encode,
  decode, round-trip
- `cache/redisfory` direct Redis value cache: `Small`과 `Medium`의 `Set`,
  `Get`, `RoundTrip`
- `cache/rediscoord` complete coordination path: local `Hot`과 Redis lock/result
  를 거치는 `ColdWinner`
- Fory native runtime의 shared mutex와 benchmark-only `sync.Pool` contention

Production default, public API, schema generation, dependency, Redis cloud
설정은 변경하지 않았다. `NativeFast`와 `NativeCompatible`는 서로 다른
schema/metadata profile이므로 동일한 wire format으로 해석하지 않는다.

## 재현 조건

| 항목 | 값 |
|---|---|
| command | `go test -run '^$' -bench '^BenchmarkIssue599' -benchmem -count=3 ./cache/redisfory` |
| parse | `python3 scripts/parse-issue-599-benchmark.py --input docs/research/outputs/issue-599/benchmark.txt --output docs/research/outputs/issue-599/summary.json` |
| timestamp (UTC) | `2026-08-07T04:49:44Z` |
| source commit | `84ff458a257a9da737856f370c39360300b635b7` + approved Issue #599 worktree diff |
| worktree | benchmark harness/parser를 포함한 의도된 dirty worktree |
| host | macOS `darwin/arm64`, Apple M5 |
| Go | `go1.26.5` |
| Apache Fory | `github.com/apache/fory/go/fory v1.3.0` |
| Redis | `redis:7.4-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99` |
| samples | row마다 3회 (`-count=3`) |
| parsed rows | 71 |

Redis/Testcontainers 경로는 top-level benchmark별로 직렬 실행했다. 전체 raw
출력은 [benchmark.txt](../research/outputs/issue-599/benchmark.txt), parser가
검증한 summary는 [summary.json](../research/outputs/issue-599/summary.json)이고,
capture manifest는 [environment.md](../research/outputs/issue-599/environment.md)다.

## 결과

`ns/op`, `B/op`, `allocs/op`는 summary의 3-sample median이다. 낮은 `ns/op`가
좋다. direct Redis의 `wire-bytes`는 Redis에 저장한 실제 value/envelope 길이이고,
in-process와 coordination의 `wire-bytes`는 해당 codec payload 길이이므로 서로
다른 계층의 수치다.

| 경로 / fixture | JSON | NativeFast | NativeCompatible |
|---|---:|---:|---:|
| Codec Small RoundTrip (`ns/op`) | 1,897 | 807 | 679 |
| Codec Small RoundTrip (`B/op`) | 325 | 263 | 327 |
| Codec Small RoundTrip (`allocs/op`) | 20 | 26 | 16 |
| Codec Medium RoundTrip (`ns/op`) | 19,473 | 2,336 | 2,198 |
| Codec Medium RoundTrip (`B/op`) | 5,831 | 4,339 | 4,391 |
| Codec Medium RoundTrip (`allocs/op`) | 36 | 38 | 28 |
| Direct Redis Small RoundTrip (`ns/op`) | 362,419 | 368,433 | 343,563 |
| Direct Redis Medium RoundTrip (`ns/op`) | 416,319 | 397,376 | 389,708 |
| Coordination ColdWinner (`ns/op`) | 710,174 | 744,297 | 712,784 |

Direct Redis의 상세 wire/alloc 결과:

| profile / fixture | `wire-bytes` | `B/op` | `allocs/op` |
|---|---:|---:|---:|
| JSON / Small | 325 | 2,539 | 45 |
| NativeFast / Small | 267 | 2,768 | 51 |
| NativeCompatible / Small | 331 | 2,800 | 41 |
| JSON / Medium | 5,831 | 19,485 | 61 |
| NativeFast / Medium | 4,343 | 20,784 | 63 |
| NativeCompatible / Medium | 4,395 | 20,624 | 53 |

Nil/empty와 반복 fixture도 같은 command에 포함됐다. Codec round-trip median은
JSON `Nil 688 ns/op`, `Empty 781 ns/op`, `Repeated 25,334 ns/op`,
NativeFast `Nil 387`, `Empty 467`, `Repeated 3,355`,
NativeCompatible `Nil 311`, `Empty 393`, `Repeated 3,150`이다.

![Issue #599 Fory and Redis benchmark](../../docs/images/readme-charts/issue599-fory-redis-benchmark.png)

차트 source는 [generator](../../docs/images/readme-charts/generate-issue-599-fory-redis-benchmark.mjs)와
[source manifest](../../docs/images/readme-charts/issue599-fory-redis-benchmark.source.json)이며,
SVG 원본은 [issue599-fory-redis-benchmark.svg](../../docs/images/readme-charts/issue599-fory-redis-benchmark.svg)다.

## 해석과 제한

1. In-process에서는 NativeCompatible의 Small round-trip이 JSON보다 약 64% 낮은
   `ns/op`였고, Medium에서도 두 native profile이 JSON보다 낮았다. NativeFast는
   Small에서 NativeCompatible보다 빠르지만 allocation 수는 더 높았다.
2. Direct Redis는 Redis command/host latency가 지배한다. Small/Medium에서
   NativeCompatible median이 가장 낮았지만, 단일 macOS/Testcontainers 실행의
   short-window 결과이며 production capacity ranking으로 일반화하지 않는다.
3. Coordination `Hot`은 wrapped in-memory cache hit(`약 81 ns/op`)이고,
   `ColdWinner`는 Redis lock/result envelope와 polling을 포함한다. ColdWinner
   결과는 JSON 710k, NativeCompatible 713k, NativeFast 744k ns/op로 profile
   차이보다 coordination 경로 비용이 크다.
4. Mutex 대 pool은 NativeFast contention에서 mutex `1,122 ns/op`, pool
   `398 ns/op`였다. Pool 비교는 각 worker가 독립 codec을 재사용하도록 한
   benchmark-only 가설이며, codec lifecycle, pool retention/GC, 실제 요청
   분포를 대표하지 않는다. 이 결과만으로 production shared-mutex default를
   변경하지 않았다.
5. NativeCompatible schema evolution reader가 V1 payload의 새 field를 zero
   value로 읽었고, truncated Fory envelope와 malformed JSON은 거부됐다.
   이는 correctness gate이며 latency 비교의 승자를 의미하지 않는다.

## DoD 상태

- [x] exact command와 Go/Fory/Redis/host metadata 보존
- [x] 세 profile과 다섯 in-process fixture matrix 비교
- [x] direct Redis와 complete coordination path 별도 측정
- [x] serialized bytes, latency, allocation, malformed rejection 기록
- [x] mutex 대 pool contention 분석
- [x] English/Korean README benchmark section에 snapshot 연결
- [x] raw output, parsed summary, SVG/PNG chart 보존
- [x] production defaults/API/dependencies 변경 없음

결론: Issue #599의 증거 수집과 문서화 범위는 완료됐다. 이 snapshot은
benchmark harness의 회귀 기준이지, 특정 profile을 전역 기본값으로 채택하는
결정은 아니다.
