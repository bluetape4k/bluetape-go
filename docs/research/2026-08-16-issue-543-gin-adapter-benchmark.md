# Issue #543 Gin 어댑터 benchmark evidence

## 실행 정보

- Capture 시각: `2026-08-16T00:00:44Z` (KST `2026-08-16 09:00:44`)
- Capture SHA: `91a41d9632f9f60fda4d28c9a40780d88d28cc4e`
- `dirty_tree`: `false`
- Go: `go1.26.6`
- Gin: `v1.12.0`
- OS/arch: `darwin/arm64`
- 호스트 logical CPU: `10` (Apple M5)
- 측정 CPU matrix: `1,2,4`
- fixture: `gin-v1.12.0-parser-only-local`
- 반복: `-count=5`, `-benchmem`, `ReportAllocs`

실행한 canonical 명령은 다음과 같다.

```bash
make bench-web-gin
# 내부 명령:
go test -timeout=10m -run '^$' -bench '^BenchmarkGinAdapter' \
  -benchmem -count=5 -cpu=1,2,4 ./web/gin
```

재현 시에는 다음처럼 matrix와 반복 수를 명시할 수 있다.

```bash
BENCH_COUNT=5 BENCH_CPU=1,2,4 make bench-web-gin
```

raw 출력은 parser가 benchmark name, CPU, `ns/op`, `B/op`, `allocs/op`으로
변환했다. 결과 ledger는 12개 기대 행과 CPU 1·2·4의 5개 sample씩 총
180개 행을 포함한다.

## CPU 1 serial 요약

모든 수치는 5개 sample의 median이며, latency와 allocation은 낮을수록
좋다.

| 경계 | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| NoOp / Serial | 825.6 | 5,360 | 13 |
| DirectCore / Serial | 1,136.0 | 5,840 | 18 |
| Bridge / Serial | 1,188.0 | 5,840 | 18 |
| FullAdapter / Serial | 5,073.0 | 12,009 | 96 |
| ColdConstruction | 634.2 | 1,672 | 25 |
| ColdFirstRequest | 8,976.0 | 12,503 | 101 |
| WarmRequest / Serial | 5,072.0 | 12,009 | 96 |
| WarmRequest / Parallel | 5,083.0 | 12,009 | 96 |

CPU 확장 시 대표 행은 다음과 같다. Parallel 측정은 worker start gate,
worker join, 요청·recorder 격리와 완료 수 `b.N` 일치 검사를 포함한다.

| 경계 | CPU 1 | CPU 2 | CPU 4 |
| --- | ---: | ---: | ---: |
| NoOp / Parallel ns/op | 837.7 | 823.1 | 753.2 |
| DirectCore / Parallel ns/op | 1,154.0 | 1,016.0 | 904.0 |
| Bridge / Parallel ns/op | 1,186.0 | 1,031.0 | 930.7 |
| FullAdapter / Parallel ns/op | 5,118.0 | 3,398.0 | 3,242.0 |

## 산식과 해석

chart와 이 문서가 사용하는 산식은 다음과 같다.

```text
bridge overhead = (bridge ns/op - direct-core ns/op) / direct-core ns/op
full overhead   = (full ns/op - no-op ns/op) / no-op ns/op
```

CPU 1 serial median 기준 bridge overhead는 약 `4.6%`, full adapter overhead는
약 `514.5%`이다. 이는 이 fixture에서 request-context bridge 자체의 추가 비용과
request context + rate-limit + parser-only JWT + resilience를 모두 조합한 경로의
비용을 분리해 보여주는 수치다. 특정 framework의 보편적인 우열이나 production
throughput 보증으로 해석하지 않는다.

- `NoOp`: 동일한 `httptest` request/writer 계약의 기준선
- `DirectCore`: Gin 없이 framework-neutral request-context 경계만 실행
- `Bridge`: Gin router와 request-context middleware를 실행
- `FullAdapter`: request context, local limiter, parser-only JWT, route resilience 조합
- `ColdConstruction`: middleware/router fixture construction만 측정
- `ColdFirstRequest`: construction 뒤 첫 request까지 측정
- `WarmRequest`: 동일 fixture를 10회 warm-up한 뒤 request만 측정

## 산출물과 재생성

동일 capture SHA를 가리키는 산출물은 다음과 같다.

- raw output: `docs/research/outputs/issue-543/bench-output.txt`
- environment ledger: `docs/research/outputs/issue-543/bench-environment.txt`
- parsed rows: `docs/research/outputs/issue-543/bench-results.json`
- chart source: `docs/images/readme-charts/gin-adapter-benchmark-summary.vl.json`
- SVG: `docs/images/readme-charts/gin-adapter-benchmark-summary.svg`
- PNG: `docs/images/readme-charts/gin-adapter-benchmark-summary.png`

차트 self-test와 재생성 명령은 다음과 같다.

```bash
node docs/images/readme-charts/generate-gin-adapter-benchmark-summary.mjs --self-test
node docs/images/readme-charts/generate-gin-adapter-benchmark-summary.mjs \
  docs/research/outputs/issue-543/bench-results.json
```

raw output은 secret/token/endpoint/path 패턴을 redaction한 뒤 parser에 전달하며,
parser·chart generator는 누락, 미지 행, 단일 capture의 중복, 비유한 값, 실패 행을
거부한다. `-count>1`에서 Go가 반올림된 동일 sample을 반복할 수 있으므로 capture의
`benchmark_count` metadata가 있을 때만 같은 metric sample을 허용한다.

## 결론과 제한

이번 capture는 clean tree이고 `capture_eligibility: eligible` provenance를 갖는다.
다만 비교 대상 baseline SHA가 별도로 제공되지 않았으므로 회귀 여부를 숫자로
추정하지 않고 `no_regression: N/A`로 남긴다. 다음 비교는 동일 command, fixture identity,
Gin/Go 버전과 clean baseline SHA를 함께 기록해야 한다.

이 benchmark는 기능 테스트를 대체하지 않는다. request cancellation, trusted peer,
JWT redaction, rate-limit headers, committed-response retry stop, recovery 조합과
framework-neutral conformance의 정합성은 `go test`, `go test -race`, conformance와
script-contract gate로 별도 검증한다. JWKS network/provider 경로는 Issue #545 범위로
이번 local parser-only fixture에 포함하지 않는다.
