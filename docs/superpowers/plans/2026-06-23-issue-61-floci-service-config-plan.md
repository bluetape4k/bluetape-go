# Issue #61 Floci Service Config Plan

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

## Tasks

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

## Risks

- Floci upstream default services are broad and use `floci/floci:latest`.
- SNS fanout delivery may need long polling; keep receive wait bounded.
- This PR depends on PR #265 and must be opened with base
  `issue-220-aws-graph-infra-fixtures`.

