# Issue 22 Cache Interfaces Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: 새 공개 cache package, generic interfaces, docs/spec/plan, tests, examples를 포함한다.
- 범위: Redis 구현 전에 first-party cache abstraction과 in-memory 기준 구현을 만든다.

## 목표

Go idiom에 맞는 작은 cache interface를 정의한다. Kotlin/JVM cache abstraction을 기계적으로 옮기지 않고, `context.Context`, generic key/value, explicit expiration, error handling을 중심으로 설계한다.

## 순서

1. cache research와 issue #22 계약을 확인한다.
2. `cache` package의 public interface, option, error contract를 spec에 고정한다.
3. in-memory implementation으로 interface semantics를 잠근다.
4. expiration, missing value, loader error, context cancellation 테스트를 추가한다.
5. examples와 package docs를 작성한다.
6. Redis near-cache(#23)와 stampede protection(#117)이 확장할 수 있는 seam을 확인한다.
7. README package table과 locale pair를 갱신한다.

## 리뷰 게이트

- interface가 Redis 구현 세부사항에 오염되지 않았는지 확인한다.
- `context.Context`가 모든 blocking/loader path에 전달되는지 확인한다.
- zero value, nil loader, expiration edge case가 테스트되는지 확인한다.
- errors가 `errors.Is`/`errors.As`에 적합한지 확인한다.
- public API가 작고 first-party Go package답게 유지되는지 확인한다.

## 검증 게이트

- `go test -count=1 ./cache`
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `go mod tidy && git diff --exit-code -- go.mod go.sum`
- `git diff --check`
