# Issue #60 AWS SDK and Floci Helper Boundaries

Issue: [#60](https://github.com/bluetape4k/bluetape-go/issues/60)  
Stack base: PR #266 / branch `issue-61-floci-service-smoke`  
Date: 2026-06-23

## Goal

Decide which AWS service surfaces should become bluetape-go helpers, which
should stay example-only, and which should remain deferred. The decision must
keep bluetape-go idiomatic for Go instead of porting Kotlin/JVM wrapper shapes.

## Evidence

- `docs/research/2026-06-01-milestone-0.9.0-aws-research.md` already sets the
  Go direction: avoid wrapping AWS SDK for Go v2 without repeated-service
  benefit, start with Floci Testcontainers, prefer example-first S3/SQS, and
  evaluate DynamoDB helper pain points separately.
- PR #265 adds `testcontainers/floci` for repeatable Floci startup and AWS SDK
  config loading.
- PR #266 extends that fixture with S3, SQS, SNS, and DynamoDB service config
  aliases and an opt-in service smoke test, while leaving AWS service clients
  caller-owned.
- `bluetape4k-aws/README.md` covers a much wider Kotlin/JVM surface:
  DynamoDB, S3, SES/SESv2, SNS, SQS, KMS, CloudWatch/Logs, IMDS, Kinesis, STS,
  RDS IAM, Secrets Manager, Parameter Store, Spring Boot operations, Ktor
  SigV4, and Floci-first emulator policy.

## Go Boundary Rule

Use direct AWS SDK for Go v2 clients by default. Add a bluetape-go package only
when the helper removes repeated, Go-specific integration work that the SDK does
not already express cleanly.

Accepted helper shapes:

- Test fixture helpers that make local AWS endpoints, credentials, and cleanup
  repeatable.
- Example packages that show idiomatic `context.Context`, SDK client
  construction, request options, and cleanup against Floci.
- Narrow utilities after a follow-up issue proves recurring boilerplate or
  error-prone SDK usage.

Rejected helper shapes:

- Coroutine, Spring Boot, or Ktor API ports.
- Generic service clients that mirror AWS SDK methods.
- Wrapper-owned retries, auth, metrics, or serialization unless a concrete
  package proves a local contract that cannot be represented by SDK options.

## Candidate Matrix

| Candidate | Decision | Owner | Rationale |
|---|---|---|---|
| Floci fixture | Adopt | #61 / PR #266 | Reusable local endpoint, static test credentials, service config, and cleanup are repo-specific testing concerns. |
| S3 | Example-only | #62 | AWS SDK for Go v2 already owns S3 clients and request types. Examples should cover local endpoint config, path-style access, object IO, and presign flow if needed. |
| SQS | Example-only | #63 | AWS SDK calls are direct. Examples should cover bounded receive loops, delete/visibility handling, and `context.Context` cancellation. |
| SNS | Example-only | #63 | Use direct SDK clients. Examples should cover SNS to SQS fanout through Floci and queue policy caveats if needed. |
| DynamoDB | Research candidate | #64 | Conditional writes, expression construction, optimistic locking, batch limits, and item mapping may justify narrow helpers, but #64 must prove the repeated pain before implementation. |
| KMS | Defer | Future issue only | KMS encryption/decryption is service-specific and security-sensitive; direct SDK plus example is enough until a consumer needs envelope encryption policy. |
| Secrets Manager | Defer | Future issue only | Secret loading policy, caching, rotation, and redaction need an application consumer before a helper contract is safe. |
| Parameter Store | Defer | Future issue only | Same boundary as Secrets Manager; direct SDK or app config code should own naming and caching until proven repeated. |
| STS | Defer | Future issue only | Direct SDK covers role/session calls. Add examples only when a package needs assumed-role or caller-identity setup. |
| RDS IAM | Defer | Future issue only | Token generation belongs with a future SQL/database package if it needs IAM auth. Do not add an AWS track package first. |
| CloudWatch | Defer | Future issue only | Metrics publishing should be driven by observability package requirements. Avoid a generic CloudWatch wrapper. |
| CloudWatch Logs | Defer | Future issue only | Log stream token behavior is service-specific; add only with a concrete logging/ops consumer. |
| Kinesis | Defer | Future issue only | Stream consumers need careful iterator, retry, and backpressure semantics. No current Go consumer exists. |
| IMDS | Defer | Future issue only | Metadata access is runtime/environment-sensitive. Add only when an app/runtime package needs it. |
| SES/SESv2 | Defer | Future issue only | Email sending needs identity, MIME, size, and validation policy. No current Go mail package depends on it. |
| SigV4 HTTP signing | Defer | Future issue only | The AWS SDK and smithy signer should remain direct unless a concrete generic HTTP signing package is requested. |
| AWS-backed config | Defer | Future issue only | Config loading touches secrets, refresh, and precedence policy. Keep it app-owned until a Go config package needs it. |
| LocalStack | Fallback only | #60-#64 | Keep as compatibility fallback for proven Floci gaps, not the default fixture. |
| DynamoDB Local | Defer | #64 | Consider only if #64 selects DynamoDB repository helpers and Floci cannot cover required behavior. |
| ElasticMQ | Defer | #63 | Consider only if Floci SQS/SNS blocks #63 examples. |
| MiniStack | Reject for now | None | Treat as evaluation-only until the exact SDK smoke matrix passes and it solves a blocker Floci cannot. |

## Follow-Up Routing

- #61 stays the Floci fixture track and is represented by PR #266.
- #62 should implement S3 examples without client wrappers.
- #63 should implement SQS/SNS producer-consumer and fanout examples without
  service wrappers.
- #64 should decide whether DynamoDB needs narrow helpers, with direct SDK as
  the default and DynamoDB Local only as fallback evidence.
- No new issue is needed for KMS, Secrets Manager, Parameter Store, STS/RDS IAM,
  CloudWatch/Logs, Kinesis, IMDS, SES, SigV4, or AWS-backed config until a real
  consumer appears.

## Decision

For 0.9.0, bluetape-go should ship Floci-backed examples and keep AWS SDK for
Go v2 service clients caller-owned. Helper implementation beyond the Floci
fixture is limited to future issue evidence, with DynamoDB as the only current
research candidate.
