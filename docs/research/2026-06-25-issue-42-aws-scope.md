# Issue 42 AWS Helper 및 Example 연구 범위

Issue #42는 전체 `bluetape4k-aws` source surface와 이미 완료된 0.8.0 Go AWS work를
대조하는 0.7.0 research gate다. 결론은 보수적이다. AWS SDK for Go v2 client는
caller-owned로 유지하고, Floci-backed test/example을 제공하며, 반복되는 Go-specific
service mechanic이 증명되는 곳에만 package code를 추가한다.

## 소스 인벤토리

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

## 현재 bluetape-go 증거

- #47, #60, #61, #62, #63, #64, #270은 이미 closed다.
- `testcontainers/floci`는 active이며 local AWS-style test를 위해 endpoint, region,
  credential, service config, opt-in smoke coverage를 노출한다.
- `examples/s3`와 `examples/sqs-sns`는 example-only이고 AWS SDK for Go v2를 직접 사용한다.
- `dynamodb/batchwrite`는 현재 유일하게 승인된 좁은 AWS helper다. Caller-owned AWS SDK
  client와 request type을 보존하면서 DynamoDB `BatchWriteItem` 25-item chunking과
  `UnprocessedItems` retry를 처리한다.

## 순위

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

## 구현됨 / 유지

- `testcontainers/floci`는 AWS-style local integration fixture로 유지한다.
- `examples/s3`와 `examples/sqs-sns`는 opt-in Floci smoke test를 가진 compile-checked
  example로 유지한다.
- `dynamodb/batchwrite`는 현재 유일한 AWS helper package로 유지한다.
- 향후 issue가 반복되는 Go-specific mechanic을 증명하기 전까지 AWS SDK client,
  request type, retry, auth, metric, service behavior는 caller-owned로 유지한다.

## 기존 Research로 라우팅

- #71은 KMS를 encryption facade의 envelope encryption compatibility로만 평가해야 한다.
  standalone KMS wrapper는 여전히 rejected다.
- #43은 SigV4를 protocol helper candidate로만 평가해야 한다. Ktor-style plugin port는
  rejected다.
- Future SQL/database work는 IAM database authentication이 repository/database story의
  일부가 될 때 RDS IAM을 평가할 수 있다.

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

## Rejected Wrapper

- Generic AWS SDK for Go client wrappers.
- Coroutine, Spring Boot, Ktor, or awspring-shaped ports.
- Repository abstractions for DynamoDB before app-shaped examples prove a
  repeated contract.
- Wrapper-owned retries, auth, metrics, or serialization where AWS SDK options
  already expose the behavior.
- LocalStack as the default test fixture. It remains only a compatibility
  fallback for a proven Floci gap.

## 필요한 Issue 업데이트

- #47 and #60 should record that the full AWS source surface was reviewed and
  no broad wrappers are selected.
- #61 should remain the Floci fixture track.
- #62 and #63 should remain example-only.
- #64 and #270 should remain the DynamoDB batchwrite-only helper decision.
- #71 should record that KMS envelope compatibility belongs there if the
  encryption facade selects cloud KMS support.

## 검증 계획

- Documentation-only PR에서는 `git diff --check`와 targeted `rg`를 실행한다.
- #47, #60-#64, #270, #71 issue body가 관련 위치에 #42 research update를 포함하는지
  확인한다.
- Go code change가 없으므로 이 PR에는 Go test가 필요하지 않다.

## 후속 권고

#42에서 새 AWS package track을 만들지 않는다. 이미 열린 #43과 #71 research를 다음으로
진행하고, 그 track들이 구체적인 Go service contract를 증명할 때만 새 AWS service issue를
만든다.
