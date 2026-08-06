# Issue #456 Review: JSON Repeated Collection Profile

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

## 범위

- `serialization.JSONSerializer.Unmarshal` default decode path.
- Repeated-collection JSON decode and round-trip benchmark/profile artifacts.
- `serialization` README pair benchmark notes.
- Research, lesson, and review ledger notes.

## 다이어그램 결정

No diagram change is required. The existing serialization README diagram
describes envelope flow; #456 changes the default JSON decode implementation
path without adding a new public topology, sequence, class relationship, or
reader question that a diagram would answer.

## 관점별 발견 사항

### 성능

P0: 0
P1: 0

- Baseline profile identified `encoding/json.(*Decoder).refill` as the largest
  avoidable allocation source for default decode.
- After using `json.Unmarshal` on the default path, repeated decode allocation
  dropped from about `2,062,640 B/op` to about `1,015,804 B/op`.
- RoundTrip allocation dropped from about `3.46-3.51 MB/op` to about
  `2.14-2.17 MB/op`.

### 안정성

P0: 0
P1: 0

- `WithDisallowUnknownFields` still uses `json.Decoder`, preserving strict
  unknown-field rejection.
- Existing tests continue to cover corrupt input, empty input, trailing JSON,
  and unknown fields.

### 보안

P0: 0
P1: 0

- No unsafe reuse, pooling, global state, or relaxed decode trust boundary was
  introduced.
- The serializer still rejects trailing JSON values.

### 운영/Ops

P0: 0
P1: 0

- Retained raw benchmark and pprof artifacts record command, host, revision,
  metric direction, and interpretation boundary.

### 개발자/API

P0: 0
P1: 0

- Public API is unchanged.
- Error wrapping remains `unmarshal json: %w` for `encoding/json` failures.
- Empty input keeps the package-owned validation error.

### 사용자/호출자

P0: 0
P1: 0

- README notes explain the default and strict decode paths without exposing a
  new API.
- No caller-visible aliasing or data ownership contract changed.

## 통합 판정

P0: 0
P1: 0

The change is narrow and evidence-backed. It removes avoidable decoder buffer
allocation from the common JSON decode path while preserving strict decode
behavior and leaving remaining fixture materialization costs to `encoding/json`.

## 증거

- `gh issue view 456`
- `ctx_batch_execute` over #456, #455, #404, and SerDe benchmark docs.
- `go test -run '^$' -bench '^BenchmarkSerialization(Decode|RoundTrip)/JSON/serde-repeated-collection-v1$' -benchmem -count=5 ./serialization`
- `go test -run '^$' -bench '^BenchmarkSerializationDecode/JSON/serde-repeated-collection-v1$' -benchmem -memprofile docs/research/outputs/issue-456/json-repeated-decode.mem.pprof ./serialization`
- `go tool pprof -top -alloc_space docs/research/outputs/issue-456/json-repeated-decode.mem.pprof`
- `go test -run '^$' -bench '^BenchmarkSerializationRoundTrip/JSON/serde-repeated-collection-v1$' -benchmem -memprofile docs/research/outputs/issue-456/json-repeated-roundtrip.mem.pprof ./serialization`
- `go tool pprof -top -alloc_space docs/research/outputs/issue-456/json-repeated-roundtrip.mem.pprof`
- `go test -count=1 ./serialization`
- `go test -run '^$' -bench '^BenchmarkSerialization(Decode|RoundTrip)/JSON/serde-repeated-collection-v1$' -benchmem -count=5 ./serialization`
- `go test -run '^$' -bench '^BenchmarkSerializationDecode/JSON/serde-repeated-collection-v1$' -benchmem -memprofile docs/research/outputs/issue-456/json-repeated-decode-after.mem.pprof ./serialization`
- `go tool pprof -top -alloc_space docs/research/outputs/issue-456/json-repeated-decode-after.mem.pprof`
- `go test -run '^$' -bench '^BenchmarkSerializationRoundTrip/JSON/serde-repeated-collection-v1$' -benchmem -memprofile docs/research/outputs/issue-456/json-repeated-roundtrip-after.mem.pprof ./serialization`
- `go tool pprof -top -alloc_space docs/research/outputs/issue-456/json-repeated-roundtrip-after.mem.pprof`
- `gofmt -w serialization/json.go`
- `go test -count=1 ./serialization`
- `go test -race -count=1 ./serialization`
- `go test -run '^$' -bench '^BenchmarkSerialization(Decode|RoundTrip)/JSON/serde-repeated-collection-v1$' -benchmem -benchtime=1x ./serialization`
- `git diff --check`
- `golangci-lint cache clean`
- `make ci`
