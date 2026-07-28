# Issue 34 Measured Values Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: measured value helpers, parsing/formatting API, tests, examples, docs를 포함한다.
- 범위: duration, size, rate 같은 값 객체를 Go-first utility로 제공한다.

## 목표

운영 설정과 문서에서 반복되는 measured values를 안전하게 parse/format할 수 있는 작은 package를 만든다. API는 standard library 타입과 잘 맞아야 하며, 단위 변환 오류를 테스트로 고정한다.

## 순서

1. #34 research와 expected value categories를 확인한다.
2. supported units, parse grammar, formatting policy, error contract를 spec에 고정한다.
3. valid/invalid parse, round-trip, boundary tests를 먼저 작성한다.
4. measured value types와 helpers를 구현한다.
5. examples와 README locale pair를 갱신한다.

## 리뷰 게이트

- API가 과도하게 많은 단위를 약속하지 않는지 확인한다.
- parse error가 caller에게 수정 가능한 정보를 제공하는지 확인한다.
- formatting output이 deterministic한지 확인한다.
- standard library type과 변환이 명확한지 확인한다.

## 검증 게이트

- `go test -count=1 ./measure/...`
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
