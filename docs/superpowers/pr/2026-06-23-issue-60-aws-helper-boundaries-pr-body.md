Part of #60

Stacked on #266 (`issue-61-floci-service-smoke`).

## Summary

- Recorded the #60 AWS SDK for Go and Floci helper boundary decision.
- Classified the broader `bluetape4k-aws` parity surface across S3, SQS, SNS,
  DynamoDB, KMS, Secrets Manager, Parameter Store, STS/RDS IAM,
  CloudWatch/Logs, Kinesis, IMDS, SES, SigV4, AWS-backed config, and emulator
  alternatives.
- Kept Floci adopted as the local AWS test fixture while preserving LocalStack,
  DynamoDB Local, and ElasticMQ as fallback-only or deferred options.
- Rejected Kotlin/JVM-shaped wrapper ports and generic AWS SDK method mirrors
  for Go.

## Scope Decisions

- #61 remains the Floci fixture track and is represented by PR #266.
- #62 should add S3 examples without service-client wrappers.
- #63 should add SQS/SNS producer-consumer and fanout examples without
  service-client wrappers.
- #64 should decide whether DynamoDB conditional-write, optimistic-locking,
  batch, expression, or item-mapping pain justifies narrow helpers.
- No new issue is created for KMS, Secrets Manager, Parameter Store, STS/RDS
  IAM, CloudWatch/Logs, Kinesis, IMDS, SES, SigV4, or AWS-backed config until a
  concrete Go consumer exists.

## Validation

- `git diff --check`
- `make fmt-check`
- `make tidy-check`
- `go test -p 1 -count=1 ./testcontainers/floci`

## Review Evidence

- Step 2-R spec review:
  `docs/superpowers/reviews/2026-06-23-issue-60-aws-helper-boundaries-step-2r-spec-review.md`
  with `P0=0 P1=0`.
- Step 3-R plan review:
  `docs/superpowers/reviews/2026-06-23-issue-60-aws-helper-boundaries-step-3r-plan-review.md`
  with `P0=0 P1=0`.
- Step 6-R review:
  `docs/superpowers/reviews/2026-06-23-issue-60-aws-helper-boundaries-step-6r-code-review.md`
  with `P0=0 P1=0`.
- Main integration fallback was used for the 7-Tier review lanes per current
  session policy; the six perspectives are recorded in the tracked review
  artifacts.

## DoD Status

| Step | Status | Evidence |
|---|---|---|
| Step 1-R issue classification | PASS | #60 classified as Type E research/decision work for AWS helper boundaries. |
| Step 2 spec | PASS | `docs/superpowers/specs/2026-06-23-issue-60-aws-helper-boundaries-spec.md`. |
| Step 2-R spec review | PASS | `docs/superpowers/reviews/2026-06-23-issue-60-aws-helper-boundaries-step-2r-spec-review.md`, `P0=0 P1=0`. |
| Step 3 plan | PASS | `docs/superpowers/plans/2026-06-23-issue-60-aws-helper-boundaries-plan.md`. |
| Step 3-R plan review | PASS | `docs/superpowers/reviews/2026-06-23-issue-60-aws-helper-boundaries-step-3r-plan-review.md`, `P0=0 P1=0`. |
| Step 4 TDD | N/A | Docs-only research decision; no implementation code or behavior changed. |
| Step 5 implementation | PASS | Added #60 research matrix, spec, plan, review artifacts, and PR body. |
| Step 6 validation | PASS | `git diff --check`, `make fmt-check`, `make tidy-check`, and `go test -p 1 -count=1 ./testcontainers/floci`. |
| Step 6-R review | PASS | `docs/superpowers/reviews/2026-06-23-issue-60-aws-helper-boundaries-step-6r-code-review.md`, `P0=0 P1=0`. |
| Step 7 PR readiness | PASS | PR is stacked on #266; assignee, milestone, and labels mirror #60; this PR body ends with `## DoD Status`. |
