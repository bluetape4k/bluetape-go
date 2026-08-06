# Issue 107 Cache Benchmark Suite Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: cache benchmark harness, reproducible evidence, documentation, CI-safe test boundaries를 추가한다.
- 범위: cache interface와 Redis-backed cache 구현의 성능 특성을 비교 가능한 형태로 고정한다.

## 목표

cache 패키지에 재현 가능한 benchmark suite를 추가하고, raw benchmark output과 요약 문서를 분리한다. 벤치마크는 기능 검증을 대체하지 않으며, allocation/latency 추세를 추적하는 evidence로 사용한다.

## 순서

1. #22 cache interfaces와 #23 Redis near-cache 구현 상태를 확인한다.
2. benchmark 대상 API와 데이터셋 크기를 고정한다.
3. in-memory, Redis, near-cache 경로를 같은 workload shape로 비교한다.
4. `testing.B` benchmark와 필요 시 opt-in integration benchmark를 분리한다.
5. raw output은 `docs/research/outputs/issue-107/` 아래에 보존하고, 문서 본문에는 한국어 요약과 해석만 둔다.
6. benchmark 결과가 README나 PR evidence에 들어가면 source command, machine/context, date를 함께 기록한다.
7. regression threshold가 필요한 경우 CI-safe 범위로만 적용한다.

## 리뷰 게이트

- benchmark가 cache semantics 테스트를 대체하지 않는지 확인한다.
- Redis/Testcontainers 의존 benchmark가 기본 unit test를 불안정하게 만들지 않는지 확인한다.
- 데이터셋과 workload가 package behavior와 연결되는지 확인한다.
- raw output과 해석 문서가 분리되어 있는지 확인한다.
- allocation과 latency 수치가 과장 없이 설명되는지 확인한다.

## 검증 게이트

- `go test -count=1 ./cache/...`
- `go test -run '^$' -bench . ./cache/...`
- 필요 시 opt-in Redis benchmark command를 별도 기록한다.
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
