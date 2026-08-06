# Issue 86 Strategic Leader Elector Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: leader election strategy layer, public options, tests, docs를 포함한다.
- 범위: #85 group elector 위에 caller가 election strategy를 선택할 수 있는 계층을 추가한다.

## 목표

기본 leader election primitive를 운영 환경에서 쓰기 쉽게 조합한다. strategy는 renew cadence, leadership callback, loss handling, startup behavior를 명확히 드러내되 Redis implementation detail을 API에 새지 않게 한다.

## 순서

1. #85 group elector contract를 확인한다.
2. strategy options와 callback lifecycle을 spec에 고정한다.
3. leadership acquired/lost, callback error, context cancellation tests를 작성한다.
4. strategy runner와 lifecycle control을 구현한다.
5. examples와 README에 usage caveats를 기록한다.

## 리뷰 게이트

- callback이 election state를 교착시키지 않는지 확인한다.
- loss handling이 caller에게 명확히 전달되는지 확인한다.
- context cancellation으로 goroutine이 누수 없이 종료되는지 확인한다.
- API가 Redis-specific type에 결합하지 않는지 확인한다.

## 검증 게이트

- `go test -count=1 ./leader/...`
- `go test -race -count=1 ./leader/...`
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
