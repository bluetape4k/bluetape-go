# Issue 26 State Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: 새 state package, public API, deterministic tests, examples, docs를 포함한다.
- 범위: workflow runner가 사용할 수 있는 first-party state transition primitive를 만든다.

## 목표

상태 값, transition guard, error contract를 작은 Go package로 제공한다. 구현은 workflow runner와 저장소 backend에 결합하지 않고, 호출자가 명시적으로 transition을 제어할 수 있게 한다.

## 순서

1. state research와 #26 issue contract를 확인한다.
2. state identifier, transition rule, invalid transition error를 spec에 고정한다.
3. zero value, unknown state, duplicate transition, terminal state tests를 먼저 작성한다.
4. state machine과 transition validation을 구현한다.
5. examples와 package docs를 작성한다.
6. #27 workflow runner에서 재사용 가능한 boundary인지 검토한다.

## 리뷰 게이트

- package가 workflow-specific terminology에 오염되지 않았는지 확인한다.
- invalid transition error가 caller에게 충분한 진단 정보를 주는지 확인한다.
- transition table이 deterministic하고 race-free한지 확인한다.
- examples가 실제 사용 흐름을 보여주는지 확인한다.

## 검증 게이트

- `go test -count=1 ./state`
- `go test -race -count=1 ./state`
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
