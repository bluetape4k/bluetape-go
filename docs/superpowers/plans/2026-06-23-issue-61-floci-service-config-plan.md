# Issue #61 Floci Service Config Plan

> 한국어 운영 요약: 이 계획 문서는 사용자 협업용 실행 계획이다. 아래 원문에 포함된 명령, 경로, API 이름, issue/PR 번호, branch 이름, code block, test output은 추적성과 재현성을 위해 그대로 보존한다. 작업 순서, 위험, 검증, 롤백 판단은 한국어 독자가 바로 실행 경계를 이해할 수 있도록 이 메모를 우선 적용한다.
> 추가 한국어 요약: 이 문서의 실행 판단은 기존 순서를 따르며, 변경자는 작업 표와 검증 목록을 먼저 확인한 뒤 관련 테스트를 실행한다. 영어로 남은 항목은 코드 식별자 또는 재현 증거다.\n

Issue: [#61](https://github.com/bluetape4k/bluetape-go/issues/61)  
Spec: `docs/superpowers/specs/2026-06-23-issue-61-floci-service-config-design.md`  
Date: 2026-06-23

## Baseline

Stack worktree: `.worktrees/issue-61-floci-service-smoke` from PR #265 branch
`issue-220-aws-graph-infra-fixtures` at `c62bcdb`.

Baseline command:

```bash
go test -p 1 -count=1 ./testcontainers/floci
```

passed before the #61 diff.

## 작업

1. Extend `testcontainers/floci/floci.go` with service config aliases, default
   helpers, and `ContainerOption` adapters for S3, SQS, SNS, and DynamoDB.
2. Extend the opt-in smoke test to exercise S3, SQS, SNS fanout through SQS, and
   DynamoDB using AWS SDK for Go v2 clients.
3. Add AWS SDK service module requirements for SQS, SNS, and DynamoDB through
   `go mod tidy`.
4. Update English and Korean README files with service config and smoke coverage
   guidance.
5. Validate serially:

```bash
go test -p 1 -count=1 ./testcontainers/floci
BLUETAPE_FLOCI_SMOKE=1 go test -p 1 -count=1 ./testcontainers/floci
go test -race -p 1 -count=1 ./testcontainers/floci
make fmt-check
make tidy-check
make vet
make lint
git diff --check
```

Run broader `make test && make race` before opening the stacked PR if the
targeted suite is stable.

## 위험

- Floci upstream default services are broad and use `floci/floci:latest`.
- SNS fanout delivery may need long polling; keep receive wait bounded.
- This PR depends on PR #265 and must be opened with base
  `issue-220-aws-graph-infra-fixtures`.

