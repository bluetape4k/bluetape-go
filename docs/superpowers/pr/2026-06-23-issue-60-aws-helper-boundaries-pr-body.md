Part of #60

Stacked on #266 (`issue-61-floci-service-smoke`).

## 요약

- #60 AWS SDK for Go 및 Floci helper 경계 결정을 기록했다.
- 지원 범위: S3, SQS, SNS, DynamoDB, KMS, Secrets Manager, Parameter Store, STS/RDS IAM,
  CloudWatch/Logs, Kinesis, IMDS, SES, SigV4, AWS-backed config, emulator
  대안을 아우르는 광범위한 `bluetape4k-aws` parity 범위를 분류했다.
- Floci를 로컬 AWS test fixture로 채택하되 LocalStack, DynamoDB Local,
  ElasticMQ는 fallback 전용 또는 보류 option으로 유지했다.
- Go에 Kotlin/JVM 형태의 wrapper port와 일반적인 AWS SDK method mirror를
  도입하는 방안을 거부했다.

## 범위 결정

- #61은 Floci fixture track으로 유지하며 PR #266이 이를 대표한다.
- #62는 service-client wrapper 없이 S3 example을 추가해야 한다.
- #63은 service-client wrapper 없이 SQS/SNS producer-consumer 및 fanout
  example을 추가해야 한다.
- #64는 DynamoDB conditional-write, optimistic-locking, batch, expression,
  item-mapping의 불편을 좁은 helper로 해결할 근거가 있는지 결정해야 한다.
- 구체적인 Go consumer가 생길 때까지 KMS, Secrets Manager, Parameter Store,
  STS/RDS IAM, CloudWatch/Logs, Kinesis, IMDS, SES, SigV4, AWS-backed
  config에 대한 새 이슈는 만들지 않는다.

## 검증

- `git diff --check`
- `make fmt-check`
- `make tidy-check`
- `go test -p 1 -count=1 ./testcontainers/floci`

## 검토 증거

- Step 2-R spec review:
  `docs/superpowers/reviews/2026-06-23-issue-60-aws-helper-boundaries-step-2r-spec-review.md`
  `P0=0 P1=0`으로 기록했다.
- Step 3-R plan review:
  `docs/superpowers/reviews/2026-06-23-issue-60-aws-helper-boundaries-step-3r-plan-review.md`
  `P0=0 P1=0`으로 기록했다.
- Step 6-R review:
  `docs/superpowers/reviews/2026-06-23-issue-60-aws-helper-boundaries-step-6r-code-review.md`
  `P0=0 P1=0`으로 기록했다.
- 현재 session policy에 따라 7-Tier review lane에는 main integration
  fallback을 사용했다. 여섯 관점은 추적 대상 review 산출물에 기록되어
  있다.

## DoD Status

| 단계 | 상태 | 증거 |
|---|---|---|
| Step 1-R 이슈 분류 | PASS | AWS helper 경계에 관한 research/decision 작업인 #60을 Type E로 분류. |
| Step 2 spec | PASS | `docs/superpowers/specs/2026-06-23-issue-60-aws-helper-boundaries-spec.md`. |
| Step 2-R spec review | PASS | `docs/superpowers/reviews/2026-06-23-issue-60-aws-helper-boundaries-step-2r-spec-review.md`, `P0=0 P1=0`. |
| Step 3 plan | PASS | `docs/superpowers/plans/2026-06-23-issue-60-aws-helper-boundaries-plan.md`. |
| Step 3-R plan review | PASS | `docs/superpowers/reviews/2026-06-23-issue-60-aws-helper-boundaries-step-3r-plan-review.md`, `P0=0 P1=0`. |
| Step 4 TDD | N/A | docs-only research decision이며 implementation code나 behavior를 변경하지 않음. |
| Step 5 implementation | PASS | #60 research matrix, spec, plan, review 산출물, PR body 추가. |
| Step 6 validation | PASS | `git diff --check`, `make fmt-check`, `make tidy-check`, `go test -p 1 -count=1 ./testcontainers/floci`. |
| Step 6-R review | PASS | `docs/superpowers/reviews/2026-06-23-issue-60-aws-helper-boundaries-step-6r-code-review.md`, `P0=0 P1=0`. |
| Step 7 PR readiness | PASS | PR은 #266에 stacked되어 있고 assignee, milestone, label이 #60과 일치하며 body는 `## DoD Status`로 끝남. |
