# Issue 32 ID Generator Foundation Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: 새 ID generator foundation package, public API, tests, examples, README updates를 포함한다.
- 범위: UUID/ULID/KSUID 등 후속 generator family가 공유할 Go-first foundation을 만든다.

## 목표

분산 시스템에서 사용할 ID generator abstraction과 기본 구현을 제공한다. Kotlin/JVM helper를 그대로 옮기지 않고 Go의 `io.Reader`, `time.Time`, error contract를 중심으로 설계한다.

## 순서

1. #32 research와 existing package layout을 확인한다.
2. generator interface, options, clock/random injection, parse/format boundary를 spec에 고정한다.
3. deterministic tests를 먼저 작성한다.
4. foundation types와 base generator를 구현한다.
5. examples와 README/README.ko.md package table을 갱신한다.
6. 후속 #166 KSUID 작업과 충돌하지 않는 extension point를 확인한다.

## 리뷰 게이트

- API가 특정 ID format에 과도하게 결합하지 않는지 확인한다.
- randomness/clock injection이 테스트 가능성을 충분히 보장하는지 확인한다.
- errors가 invalid input과 entropy failure를 구분하는지 확인한다.
- examples가 caller-facing 사용법을 명확히 보여주는지 확인한다.

## 검증 게이트

- `go test -count=1 ./id/...`
- `go test -race -count=1 ./id/...`
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
