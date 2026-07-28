# Issue #59 Plan: Audit Example Service

> 한국어 운영 요약: 이 계획 문서는 사용자 협업용 실행 계획이다. 아래 원문에 포함된 명령, 경로, API 이름, issue/PR 번호, branch 이름, code block, test output은 추적성과 재현성을 위해 그대로 보존한다. 작업 순서, 위험, 검증, 롤백 판단은 한국어 독자가 바로 실행 경계를 이해할 수 있도록 이 메모를 우선 적용한다.
> 추가 한국어 요약: 이 문서의 실행 판단은 기존 순서를 따르며, 변경자는 작업 표와 검증 목록을 먼저 확인한 뒤 관련 테스트를 실행한다. 영어로 남은 항목은 코드 식별자 또는 재현 증거다.\n

## Steps

1. Add failing tests for a runnable order-service audit example.
2. Implement `examples/audit` service and in-memory outbox fixture.
3. Document usage and boundaries in English/Korean package READMEs.
4. Link the example from root READMEs and changelog.
5. Run targeted tests, race tests, vet/lint, and local CI.
6. Record review, lesson, and PR body artifacts.

## Design Decisions

- Keep code under `examples/audit`; do not promote a new production package API.
- Inject `audit.Repository` so the example proves the repository boundary.
- Use `audit.MemoryRepository` and `MemoryOutbox` fixtures to stay service-free.
- Mention `audit/sqloutbox.Store.Enqueue` as the production durable outbox path,
  but do not require PostgreSQL for this example.

## Risk Controls

- Use TDD RED before implementation.
- Use `GoroutineStressTester` and `AsyncJobTester` as required by #59.
- Run the race detector for the example package.
