# Issue 42 AWS Helper And Examples Research Scope

Issue #42 is the 0.7.0 research gate for reconciling the full
`bluetape4k-aws` source surface with the already-completed 0.8.0 Go AWS work.
The outcome is conservative: keep AWS SDK for Go v2 clients caller-owned,
provide Floci-backed tests/examples, and add package code only where repeated
Go-specific service mechanics are proven.

## Source Inventory

Source repository: `/Users/debop/work/bluetape4k/bluetape4k-aws`

- `aws-java` wraps AWS Java SDK v2 sync/async/coroutine access for DynamoDB,
  S3, S3 Vectors, SES/SESv2, SNS, SQS, KMS, CloudWatch, CloudWatch Logs,
  Kinesis, and STS.
- `aws-kotlin` wraps AWS Kotlin SDK native suspend clients and DSL builders for
  DynamoDB, S3, SES/SESv2, SNS, SQS, KMS, CloudWatch, CloudWatch Logs,
  Kinesis, and STS.
- `aws-exposed` provides AWS-backed database configuration foundations,
  Secrets Manager/Parameter Store source descriptors, RDS IAM authentication
  token support, Hikari-backed Exposed database creation, and redacted secret
  diagnostics.
- `aws-ktor` provides a Ktor SigV4 client plugin, S3 REST client, SQS consumer
  runtime, DynamoDB server repository plugin, EC2 IMDS helpers, CloudWatch
  helpers, and AWS-backed Exposed configuration.
- `aws-spring-boot` provides Spring Boot 4 auto-configuration, S3/SQS/SNS/SES
  operations, DynamoDB repositories, KMS helpers, remote config sources,
  CloudWatch/Logs, IMDS, and AWS-backed Exposed wiring.
- Examples cover Ktor and Spring shapes for DynamoDB, S3, SQS/SNS, Exposed, and
  local emulator-backed testing.

## Current bluetape-go Evidence

- #47, #60, #61, #62, #63, #64, and #270 are already closed.
- `testcontainers/floci` is active and exposes endpoint, region, credentials,
  service config, and opt-in smoke coverage for local AWS-style tests.
- `examples/s3` and `examples/sqs-sns` are example-only and use AWS SDK for Go
  v2 directly.
- `dynamodb/batchwrite` is the single accepted narrow AWS helper so far. It
  handles DynamoDB `BatchWriteItem` 25-item chunking and `UnprocessedItems`
  retry while preserving caller-owned AWS SDK clients and request types.

## Ranking

| Area | Go fit | Risk | Decision |
|---|---:|---:|---|
| Floci fixture | High | Medium | Already implemented; keep as default local AWS test path. |
| S3 | High as examples | Medium | Already example-only; do not wrap the S3 client. |
| SQS/SNS | High as examples | Medium/high | Already example-only; keep listener/runtime wrappers out until repeated lifecycle pain is proven. |
| DynamoDB batch write | High | Medium | Already implemented as narrow helper in #270. |
| DynamoDB repository/mapper | Medium | High | Keep example/workshop-owned; direct SDK `attributevalue`, expressions, and app repositories remain clearer. |
| KMS envelope encryption | Medium/high | High | Route to #71; evaluate only as cloud-KMS envelope compatibility for the encryption facade. |
| Secrets Manager / Parameter Store | Medium | High | Defer; config precedence, refresh, caching, redaction, and rotation need an application config owner. |
| STS / RDS IAM | Medium | High | Defer to a future SQL/database or deployment credential issue. Do not add AWS track wrappers first. |
| CloudWatch / CloudWatch Logs | Medium | High | Defer until observability/logging requirements define metrics/log contracts. |
| Kinesis | Low/medium | High | Defer; stream consumption needs iterator, checkpoint, retry, and backpressure design. |
| EC2 IMDS | Low/medium | High | Defer; runtime-sensitive and should be bounded only when an EC2 deployment consumer exists. |
| SES/SESv2 | Low/medium | High | Defer; mail identity, MIME, size, template, and validation policies are app-specific. |
| SigV4 generic HTTP signing | Low/medium | Medium/high | Defer to #43 protocol research; avoid Ktor-shaped port. |
| S3 Vectors | Low/medium | High | Defer until a concrete vector package or example needs it. |

## Implemented / Keep

- Keep `testcontainers/floci` as the AWS-style local integration fixture.
- Keep `examples/s3` and `examples/sqs-sns` as compile-checked examples with
  opt-in Floci smoke tests.
- Keep `dynamodb/batchwrite` as the only current AWS helper package.
- Keep AWS SDK clients, request types, retries, auth, metrics, and service
  behavior caller-owned unless a future issue proves repeated Go-specific
  mechanics.

## Route To Existing Research

- #71 should evaluate KMS only as envelope encryption compatibility for the
  encryption facade. A standalone KMS wrapper is still rejected.
- #43 should evaluate SigV4 only as a protocol helper candidate. A Ktor-style
  plugin port is rejected.
- Future SQL/database work may evaluate RDS IAM if IAM database authentication
  becomes part of the repository/database story.

## Defer

- Secrets Manager and Parameter Store config loading until a Go config package
  or application example defines precedence, refresh, redaction, and caching.
- CloudWatch/Logs until observability/logging package requirements exist.
- Kinesis until a stream-processing package needs iterator, checkpoint, retry,
  and backpressure semantics.
- IMDS until an EC2 runtime/deployment package needs bounded metadata access.
- SES/SESv2 until a mail package or application example defines identity,
  template, MIME, attachment, and validation contracts.
- S3 Vectors until a vector package or example needs it.

## Rejected Wrappers

- Generic AWS SDK for Go client wrappers.
- Coroutine, Spring Boot, Ktor, or awspring-shaped ports.
- Repository abstractions for DynamoDB before app-shaped examples prove a
  repeated contract.
- Wrapper-owned retries, auth, metrics, or serialization where AWS SDK options
  already expose the behavior.
- LocalStack as the default test fixture. It remains only a compatibility
  fallback for a proven Floci gap.

## Issue Updates Required

- #47 and #60 should record that the full AWS source surface was reviewed and
  no broad wrappers are selected.
- #61 should remain the Floci fixture track.
- #62 and #63 should remain example-only.
- #64 and #270 should remain the DynamoDB batchwrite-only helper decision.
- #71 should record that KMS envelope compatibility belongs there if the
  encryption facade selects cloud KMS support.

## Validation Plan

- Documentation-only PR: `git diff --check` and targeted `rg`.
- Verify #47, #60-#64, #270, and #71 issue bodies contain the #42 research
  update where relevant.
- No Go tests are required for this PR because no Go code changes.

## Follow-up Recommendation

Do not create new AWS package tracks from #42. Work the already-open #43 and
#71 research next, and only create new AWS service issues when those tracks
prove a concrete Go service contract.
