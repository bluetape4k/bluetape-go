# Issue #60 AWS SDK and Floci Helper Boundaries

Issue: [#60](https://github.com/bluetape4k/bluetape-go/issues/60)  
Stack base: PR #266 / branch `issue-61-floci-service-smoke`  
Date: 2026-06-23

## 목표

어떤 AWS service surface가 bluetape-go helper가 되어야 하는지, 어떤 것은
example-only로 남아야 하는지, 어떤 것은 연기해야 하는지 결정한다. 결정은
Kotlin/JVM wrapper 형태를 이식하지 않고 bluetape-go가 Go idiom을 유지하게
해야 한다.

## 증거

- `docs/research/2026-06-01-milestone-0.9.0-aws-research.md`는 이미 Go
  방향을 정했다. 반복 service 이점 없이 AWS SDK for Go v2를 감싸지 말고,
  Floci Testcontainers로 시작하며, S3/SQS는 example-first로 두고,
  DynamoDB helper pain point는 별도로 평가한다.
- PR #265는 반복 가능한 Floci 시작과 AWS SDK config loading을 위한
  `testcontainers/floci`를 추가한다.
- PR #266은 그 fixture에 S3, SQS, SNS, DynamoDB service config alias와
  opt-in service smoke test를 더한다. AWS service client는 caller-owned로
  남긴다.
- `bluetape4k-aws/README.md`는 훨씬 넓은 Kotlin/JVM surface를 다룬다.
  DynamoDB, S3, SES/SESv2, SNS, SQS, KMS, CloudWatch/Logs, IMDS, Kinesis,
  STS, RDS IAM, Secrets Manager, Parameter Store, Spring Boot operations,
  Ktor SigV4, Floci-first emulator policy가 포함된다.

## Go 경계 규칙

기본값은 direct AWS SDK for Go v2 client를 사용한다. Helper가 SDK로 이미
깔끔하게 표현되지 않는 반복적인 Go-specific integration 작업을 제거할 때만
bluetape-go package를 추가한다.

허용되는 helper 형태:

- Local AWS endpoint, credential, cleanup을 반복 가능하게 만드는 test
  fixture helper.
- Floci 대상에서 idiomatic `context.Context`, SDK client construction,
  request option, cleanup을 보여 주는 example package.
- 반복 boilerplate 또는 오류가 잦은 SDK 사용이 후속 issue에서 증명된 뒤의
  좁은 utility.

거절되는 helper 형태:

- Coroutine, Spring Boot, Ktor API port.
- AWS SDK method를 그대로 반영하는 generic service client.
- Concrete package가 SDK option으로 표현할 수 없는 local contract를
  증명하기 전의 wrapper-owned retry, auth, metrics, serialization.

## Candidate Matrix

| Candidate | Decision | Owner | Rationale |
|---|---|---|---|
| Floci fixture | Adopt | #61 / PR #266 | 재사용 가능한 local endpoint, static test credential, service config, cleanup은 repo-specific testing concern이다. |
| S3 | Example-only | #62 | AWS SDK for Go v2가 이미 S3 client와 request type을 소유한다. Example은 local endpoint config, path-style access, object IO, 필요 시 presign flow를 다루면 된다. |
| SQS | Example-only | #63 | AWS SDK call은 직접 사용한다. Example은 bounded receive loop, delete/visibility handling, `context.Context` cancellation을 다루면 된다. |
| SNS | Example-only | #63 | Direct SDK client를 사용한다. Example은 Floci를 통한 SNS to SQS fanout과 queue policy caveat를 필요 시 다룬다. |
| DynamoDB | Research candidate | #64 | Conditional write, expression construction, optimistic locking, batch limit, item mapping은 좁은 helper를 정당화할 수 있지만 #64가 구현 전에 반복 pain을 증명해야 한다. |
| KMS | Defer | Future issue only | KMS encryption/decryption은 service-specific이고 security-sensitive하다. Consumer가 envelope encryption policy를 요구하기 전에는 direct SDK와 example이면 충분하다. |
| Secrets Manager | Defer | Future issue only | Secret loading policy, caching, rotation, redaction은 helper contract가 안전해지기 전에 application consumer가 필요하다. |
| Parameter Store | Defer | Future issue only | Secrets Manager와 같은 경계다. 반복성이 증명될 때까지 direct SDK 또는 app config code가 naming/caching을 소유한다. |
| STS | Defer | Future issue only | Direct SDK가 role/session call을 덮는다. Package가 assumed-role 또는 caller-identity setup을 필요로 할 때만 example을 추가한다. |
| RDS IAM | Defer | Future issue only | Token generation은 IAM auth가 필요한 미래 SQL/database package에 속한다. AWS track package를 먼저 만들지 않는다. |
| CloudWatch | Defer | Future issue only | Metrics publishing은 observability package 요구에서 출발해야 한다. Generic CloudWatch wrapper를 피한다. |
| CloudWatch Logs | Defer | Future issue only | Log stream token behavior는 service-specific이다. 구체 logging/ops consumer가 있을 때만 추가한다. |
| Kinesis | Defer | Future issue only | Stream consumer는 iterator, retry, backpressure semantics를 신중히 다뤄야 한다. 현재 Go consumer가 없다. |
| IMDS | Defer | Future issue only | Metadata access는 runtime/environment-sensitive하다. App/runtime package가 필요로 할 때만 추가한다. |
| SES/SESv2 | Defer | Future issue only | Email sending에는 identity, MIME, size, validation policy가 필요하다. 현재 Go mail package가 의존하지 않는다. |
| SigV4 HTTP signing | Defer | Future issue only | Concrete generic HTTP signing package가 요청되기 전에는 AWS SDK와 smithy signer를 직접 둔다. |
| AWS-backed config | Defer | Future issue only | Config loading은 secrets, refresh, precedence policy를 건드린다. Go config package가 필요로 하기 전에는 app-owned로 둔다. |
| LocalStack | Fallback only | #60-#64 | Floci gap이 증명될 때의 compatibility fallback으로 유지하고 default fixture로 두지 않는다. |
| DynamoDB Local | Defer | #64 | #64가 DynamoDB repository helper를 선택하고 Floci가 필요한 동작을 덮지 못할 때만 고려한다. |
| ElasticMQ | Defer | #63 | Floci SQS/SNS가 #63 example을 막을 때만 고려한다. |
| MiniStack | Reject for now | None | 정확한 SDK smoke matrix를 통과하고 Floci가 해결하지 못하는 blocker를 풀기 전까지 evaluation-only로 취급한다. |

## 후속 라우팅

- #61은 Floci fixture track으로 남고 PR #266이 이를 대표한다.
- #62는 client wrapper 없이 S3 example을 구현해야 한다.
- #63은 service wrapper 없이 SQS/SNS producer-consumer와 fanout example을
  구현해야 한다.
- #64는 direct SDK를 기본값으로 두고 DynamoDB가 좁은 helper를 필요로
  하는지 결정해야 한다. DynamoDB Local은 fallback evidence로만 둔다.
- KMS, Secrets Manager, Parameter Store, STS/RDS IAM, CloudWatch/Logs,
  Kinesis, IMDS, SES, SigV4, AWS-backed config는 실제 consumer가 나타날
  때까지 새 issue가 필요 없다.

## 결정

0.9.0에서 bluetape-go는 Floci-backed example을 제공하고 AWS SDK for Go v2
service client는 caller-owned로 유지한다. Floci fixture를 넘어서는 helper
구현은 미래 issue evidence로 제한하며, DynamoDB만 현재 research candidate다.
