# Issue 166 KSUID Generator Family Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: ID generator family extension, benchmarks, tests, README updates를 포함한다.
- 범위: existing id generator package에 KSUID 계열 generator를 추가한다.

## 목표

시간 정렬 가능한 KSUID generator를 first-party API로 제공하고, UUID/ULID 등 기존 generator와 같은 contract 아래에서 비교 가능하게 만든다.

## 설계 원칙

- public API는 기존 generator family naming과 option pattern을 따른다.
- randomness source와 clock source는 테스트에서 deterministic하게 주입할 수 있어야 한다.
- string/binary representation과 ordering semantics를 문서화한다.
- benchmark 수치는 raw output과 해석을 분리한다.

## 순서

1. 기존 `id` package와 #166 issue contract를 확인한다.
2. KSUID format, timestamp precision, entropy size, error contract를 spec에 고정한다.
3. deterministic clock/random tests를 먼저 작성한다.
4. generator implementation과 parser/formatter helpers를 추가한다.
5. collision smoke, ordering, zero-value, invalid input tests를 추가한다.
6. benchmark와 README locale pair를 갱신한다.
7. PR evidence에 benchmark를 쓰면 command와 raw output path를 함께 기록한다.

## 리뷰 게이트

- 기존 ID generator API와 일관적인지 확인한다.
- entropy source가 concurrency-safe한지 확인한다.
- ordering guarantee가 과장되지 않았는지 확인한다.
- parser error가 caller에게 충분히 진단 가능한지 확인한다.
- benchmark가 실제 hot path를 반영하는지 확인한다.

## 검증 게이트

- `go test -count=1 ./id/...`
- `go test -race -count=1 ./id/...`
- `go test -run '^$' -bench . ./id/...`
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
