# Issue #543 Gin 어댑터 benchmark evidence

## 실행 정보

- Capture 시각: `2026-08-16T02:25:06Z` (KST `2026-08-16 11:25:06`)
- Capture SHA: `3e14c7e0ad027c7d0187e7a0524f0a5db976e126`
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
변환했다. 결과 ledger는 14개 기대 행과 CPU 1·2·4의 5개 sample씩 총
210개 행을 포함한다.

## CPU 1 serial 요약

모든 수치는 5개 sample의 median이며, latency와 allocation은 낮을수록
좋다.

| 경계 | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| NoOp / Serial | 844.1 | 5,360 | 13 |
| DirectCore / Serial | 1,158.0 | 5,840 | 18 |
| Bridge / Serial | 1,206.0 | 5,840 | 18 |
| FullAdapter / Serial | 5,160.0 | 12,009 | 96 |
| FullAdapterRetry / Serial | 5,197.0 | 12,041 | 97 |
| ColdConstruction | 631.8 | 1,672 | 25 |
| ColdFirstRequest | 10,345.0 | 12,503 | 101 |
| WarmRequest / Serial | 5,172.0 | 12,009 | 96 |
| WarmRequest / Parallel | 5,159.0 | 12,009 | 96 |

CPU 확장 시 대표 행은 다음과 같다. Parallel 측정은 worker start gate,
worker join, 요청·recorder 격리와 완료 수 `b.N` 일치 검사를 포함한다.

| 경계 | CPU 1 | CPU 2 | CPU 4 |
| --- | ---: | ---: | ---: |
| NoOp / Parallel ns/op | 856.1 | 828.4 | 749.4 |
| DirectCore / Parallel ns/op | 1,154.0 | 1,016.0 | 904.0 |
| Bridge / Parallel ns/op | 1,200.0 | 1,032.0 | 927.6 |
| FullAdapter / Parallel ns/op | 5,139.0 | 3,515.0 | 2,979.0 |
| FullAdapterRetry / Parallel ns/op | 5,194.0 | 3,530.0 | 2,976.0 |

## 산식과 해석

chart와 이 문서가 사용하는 산식은 다음과 같다.

```text
bridge overhead = (bridge ns/op - direct-core ns/op) / direct-core ns/op
full overhead   = (full ns/op - no-op ns/op) / no-op ns/op
```

CPU 1 serial median 기준 bridge overhead는 약 `4.1%`, full adapter overhead는
약 `511.3%`이다. 이는 이 fixture에서 request-context bridge 자체의 추가 비용과
request context + rate-limit + parser-only JWT + resilience를 모두 조합한 경로의
비용을 분리해 보여주는 수치다. 특정 framework의 보편적인 우열이나 production
throughput 보증으로 해석하지 않는다.

- `NoOp`: 동일한 `httptest` request/writer 계약의 기준선
- `DirectCore`: Gin 없이 framework-neutral request-context 경계만 실행
- `Bridge`: Gin router와 request-context middleware를 실행
- `FullAdapter`: request context, local limiter, parser-only JWT, 빈 resilience policy 조합
- `FullAdapterRetry`: `FullAdapter`에 no-backoff `MaxAttempts=2` retry policy를 추가한 성공 경로
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
- baseline: `docs/research/outputs/issue-543/bench-baseline.json`
- regression report: `docs/research/outputs/issue-543/bench-regression.json`

차트 self-test와 재생성 명령은 다음과 같다.

```bash
node docs/images/readme-charts/generate-gin-adapter-benchmark-summary.mjs --self-test
node docs/images/readme-charts/generate-gin-adapter-benchmark-summary.mjs \
  docs/research/outputs/issue-543/bench-results.json
```

동일 fixture와 clean 실행 환경의 baseline 회귀 비교는 다음 명령으로 수행한다.

```bash
make bench-web-gin-regression \
  BENCH_BASELINE=docs/research/outputs/issue-543/bench-baseline.json
```

비교기는 CPU 1 serial의 `FullAdapter`와 `FullAdapterRetry`에 대해 `ns/op`
15%, `B/op`와 `allocs/op` 10% 초과 증가를 실패로 판정한다. fixture identity,
Go/Gin 버전, OS/arch, CPU matrix, clean provenance가 다르면 회귀 없음으로
오인하지 않고 `inconclusive`/`no_regression: N/A`를 반환한다.

raw output은 secret/token/endpoint/path 패턴을 redaction한 뒤 parser에 전달하며,
parser·chart generator는 누락, 미지 행, 단일 capture의 중복, 비유한 값, 실패 행을
거부한다. `-count>1`에서 Go가 반올림된 동일 sample을 반복할 수 있으므로 capture의
`benchmark_count` metadata가 있을 때만 같은 metric sample을 허용한다.

## 결론과 제한

이번 변경은 동일 command와 fixture identity로 clean baseline을 저장하고, 현재
candidate를 비교해 `bench-regression.json`의 `status: passed`와
`no_regression: passed`를 확인했다. 이후 다른 OS/arch, Go/Gin 버전, fixture가
섞이면 비교기는 `inconclusive`로 중단하며 `no_regression: N/A`를 유지한다.

이 benchmark는 기능 테스트를 대체하지 않는다. request cancellation, trusted peer,
JWT redaction, rate-limit headers, committed-response retry stop, recovery 조합과
framework-neutral conformance의 정합성은 `go test`, `go test -race`, conformance와
script-contract gate로 별도 검증한다. JWKS network/provider 경로는 Issue #545 범위로
이번 local parser-only fixture에 포함하지 않는다.
