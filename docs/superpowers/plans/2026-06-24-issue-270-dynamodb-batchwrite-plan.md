# Issue #270 Plan

> 한국어 운영 요약: 이 계획 문서는 사용자 협업용 실행 계획이다. 아래 원문에 포함된 명령, 경로, API 이름, issue/PR 번호, branch 이름, code block, test output은 추적성과 재현성을 위해 그대로 보존한다. 작업 순서, 위험, 검증, 롤백 판단은 한국어 독자가 바로 실행 경계를 이해할 수 있도록 이 메모를 우선 적용한다.
> 추가 한국어 요약: 이 문서의 실행 판단은 기존 순서를 따르며, 변경자는 작업 표와 검증 목록을 먼저 확인한 뒤 관련 테스트를 실행한다. 영어로 남은 항목은 코드 식별자 또는 재현 증거다.\n

## Steps

1. Add `dynamodb/batchwrite` with SDK-native request and client contracts.
2. Implement deterministic table ordering, 25-item chunking, unprocessed-item
   retry, context-aware backoff sleep, typed retry exhaustion, and result
   aggregation.
3. Add unit tests for request splitting, retry behavior, failure paths,
   cancellation, defensive copy, and option handling.
4. Add opt-in Floci smoke coverage for real DynamoDB-compatible execution.
5. Update package README files and root package tables in English and Korean.
6. Run local verification: targeted tests, race tests, smoke tests, format,
   tidy, vet, lint, and diff checks.
7. Create PR with issue metadata parity, 7-tier review evidence, and `## DoD
   Status` as the final PR body heading.

## Risk Controls

- Keep AWS SDK types visible in the public API to avoid a second modeling layer.
- Do not own credentials, endpoints, table creation, or repository concerns.
- Keep retries bounded by default and caller-configurable.
- Use context cancellation before each call and before delayed retry.
