# Issue #63 SQS/SNS Examples Plan

> 한국어 운영 요약: 이 계획 문서는 사용자 협업용 실행 계획이다. 아래 원문에 포함된 명령, 경로, API 이름, issue/PR 번호, branch 이름, code block, test output은 추적성과 재현성을 위해 그대로 보존한다. 작업 순서, 위험, 검증, 롤백 판단은 한국어 독자가 바로 실행 경계를 이해할 수 있도록 이 메모를 우선 적용한다.
> 추가 한국어 요약: 이 문서의 실행 판단은 기존 순서를 따르며, 변경자는 작업 표와 검증 목록을 먼저 확인한 뒤 관련 테스트를 실행한다. 영어로 남은 항목은 코드 식별자 또는 재현 증거다.\n

Issue: [#63](https://github.com/bluetape4k/bluetape-go/issues/63)  
Type: B Fast Track  
Date: 2026-06-24

## Task List

1. Add `examples/sqs-sns` package docs and compile-checked examples.
2. Add example-local JSON codec, receive, ack/delete, visibility, queue ARN, and
   redrive policy helpers.
3. Add opt-in Floci smoke for SQS send/receive/delete/visibility and SNS to SQS
   fanout.
4. Update root README and README.ko package indexes.
5. Run targeted tests, smoke, race, repo gates, 7-tier review, PR, and CI.

## 검증

- `go test -count=1 ./examples/sqs-sns`
- `go test -race -count=1 ./examples/sqs-sns`
- `BLUETAPE_SQS_SNS_EXAMPLE_SMOKE=1 go test -p 1 -count=1 ./examples/sqs-sns`
- `BLUETAPE_SQS_SNS_EXAMPLE_SMOKE=1 go test -race -p 1 -count=1 ./examples/sqs-sns`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make test`
- `make race`
- `git diff --check`
