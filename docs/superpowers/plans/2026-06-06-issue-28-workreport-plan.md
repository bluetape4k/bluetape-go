# Issue 28 WorkReport Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: workflow execution report model, formatting, tests, examples, docs를 포함한다.
- 범위: #27 workflow runner 결과를 사람이 읽을 수 있고 테스트 가능한 report로 정리한다.

## 목표

workflow 실행 결과를 structured report로 표현하고, CLI/log/Markdown 출력에 재사용할 수 있는 최소 formatting boundary를 제공한다.

## 순서

1. #27 runner result shape와 필요한 report fields를 확인한다.
2. report model, status enum, duration/error formatting을 spec에 고정한다.
3. success/failure/skipped/cancelled step report tests를 먼저 작성한다.
4. stable Markdown/text rendering을 구현한다.
5. examples와 README에 report 사용 흐름을 추가한다.
6. locale docs에서 report output literal과 설명을 분리한다.

## 리뷰 게이트

- report model이 runner internals에 과도하게 결합하지 않는지 확인한다.
- rendering output이 deterministic한지 확인한다.
- error message가 sensitive value를 그대로 노출하지 않는지 확인한다.
- tests가 ordering과 formatting을 고정하는지 확인한다.

## 검증 게이트

- `go test -count=1 ./workflow/...`
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
