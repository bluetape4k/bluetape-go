# Issue 27 Workflow Runners Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: workflow runner package, context-aware execution, tests, examples, docs를 포함한다.
- 범위: #26 state primitive를 사용하는 first-party workflow runner를 만든다.

## 목표

작업 단계를 순서대로 실행하고, 실패와 cancellation을 명확히 보고하는 작은 workflow runner를 제공한다. runner는 orchestration engine이 아니라 library-level primitive로 유지한다.

## 순서

1. #26 state contract와 #28 workreport 요구사항을 확인한다.
2. step, runner, result, error contract를 spec에 고정한다.
3. success, failure, cancellation, panic recovery, skipped step tests를 먼저 작성한다.
4. context-aware runner implementation을 추가한다.
5. report hook 또는 result export boundary를 #28과 맞춘다.
6. examples와 README locale pair를 갱신한다.

## 리뷰 게이트

- runner가 storage/backend detail을 소유하지 않는지 확인한다.
- context cancellation과 deadline을 각 step에 전달하는지 확인한다.
- 실패 후 state/result가 caller에게 충분히 설명되는지 확인한다.
- panic handling 정책이 테스트와 문서에 명확한지 확인한다.

## 검증 게이트

- `go test -count=1 ./workflow/...`
- `go test -race -count=1 ./workflow/...`
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
