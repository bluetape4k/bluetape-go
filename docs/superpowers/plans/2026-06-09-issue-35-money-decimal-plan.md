# Issue 35 Money Decimal Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: money/decimal package, arithmetic contract, parsing/formatting tests, docs/examples를 포함한다.
- 범위: 금융 계산에서 float 사용을 피하기 위한 decimal money value를 제공한다.

## 목표

currency-aware money type과 decimal arithmetic helper를 제공한다. 구현은 정확성, overflow/bounds, serialization, formatting을 명시적으로 테스트하고, 외부 money framework를 감싸는 형태를 피한다.

## 설계 원칙

- public API는 immutable value semantics를 제공한다.
- decimal scale과 rounding policy를 명확히 드러낸다.
- currency code validation은 ISO 목록 전체 내장을 강제하지 않고 최소 contract로 유지한다.
- JSON/text marshal behavior를 테스트로 고정한다.
- benchmark 결과는 raw output과 해석을 분리한다.

## 순서

1. #35 research와 decimal library 선택/비선택 근거를 확인한다.
2. amount representation, scale, rounding, currency validation, parse/format grammar를 spec에 고정한다.
3. arithmetic, comparison, rounding, overflow, marshal tests를 먼저 작성한다.
4. money/decimal implementation을 추가한다.
5. examples와 README/README.ko.md를 갱신한다.
6. benchmark가 필요한 경우 `go test -run '^$' -bench . ./money/...` 결과를 raw output으로 보존한다.

## 리뷰 게이트

- float 기반 계산이 public arithmetic path에 들어가지 않는지 확인한다.
- rounding mode가 호출자에게 명확한지 확인한다.
- overflow와 invalid scale이 테스트되는지 확인한다.
- JSON/text representation이 안정적인지 확인한다.
- examples가 실제 결제/정산 misuse를 줄이는지 확인한다.

## 검증 게이트

- `go test -count=1 ./money/...`
- `go test -race -count=1 ./money/...`
- `go test -run '^$' -bench . ./money/...`
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
