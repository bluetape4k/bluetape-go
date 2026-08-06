# Issue #60 AWS Helper Boundary Spec

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

Issue: [#60](https://github.com/bluetape4k/bluetape-go/issues/60)  
Date: 2026-06-23

## 범위

Record a repository decision for the 0.9.0 AWS helper track:

- classify service candidates as adopt, example-only, research candidate,
  defer, fallback-only, or reject;
- prefer Floci for local integration testing unless a follow-up issue proves a
  blocker;
- reject Kotlin/JVM-shaped wrapper ports for Go;
- route follow-up work to #61, #62, #63, and #64 without creating premature
  packages.

## Non-Goals

- Do not add Go source code.
- Do not add AWS SDK dependencies.
- Do not close #62, #63, or #64.
- Do not merge stacked PRs.
- Do not create KMS, Secrets Manager, Parameter Store, CloudWatch, Kinesis,
  IMDS, SES, SigV4, or AWS-backed config issues without a concrete consumer.

## Required Evidence

- Current `bluetape-go` 0.9.0 AWS research document.
- Current Floci fixture and service-smoke decisions from #220 and #61.
- `bluetape4k-aws` service coverage and Floci-first emulator policy.
- Current open 0.9.0 GitHub issue routing.

## Acceptance Mapping

| Acceptance criterion | Spec response |
|---|---|
| Review service candidates beyond S3/SQS/DynamoDB. | Candidate matrix covers KMS, Secrets Manager, Parameter Store, STS/RDS IAM, CloudWatch/Logs, Kinesis, IMDS, SES/SNS, SigV4, AWS-backed config, and emulator fallbacks. |
| Mark each candidate implement/adopt/example-only/defer. | Candidate matrix uses adopt, example-only, research candidate, defer, fallback-only, and reject. |
| Prefer Floci unless blocker. | Floci fixture is adopted; LocalStack, DynamoDB Local, ElasticMQ, and MiniStack are fallback/deferred. |
| Document rejected wrappers and direct SDK rationale. | Go boundary rule rejects service-client wrappers and JVM/Spring/Ktor ports. |
| Update #61-#64 or create follow-ups. | Follow-up routing assigns #61-#64 and records no new issue needed for deferred surfaces. |

## Review Expectations

Run 7-tier review as read-only main integration fallback:

- Performance: no hot-path code or heavy test path added.
- Stability: examples and follow-ups remain isolated; no fixture default changes.
- Security: no new credential loading, secrets policy, or production AWS access.
- Operator/Ops: Floci remains default, fallback emulators are explicitly gated.
- Developer/API: direct AWS SDK remains caller-owned.
- User/Caller: follow-up issues have clear boundaries.
- Main integration: PR remains stacked on #266 and unmerged.
