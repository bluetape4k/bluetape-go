# SQS S3 대용량 payload envelope 설계

## 상태와 범위

- 상태: #517 승인 실행 계획에 따라 구현한다.
- 부모 이슈: [#517](https://github.com/bluetape4k/bluetape-go/issues/517)
- 작업 이슈: [#523](https://github.com/bluetape4k/bluetape-go/issues/523)
- 기준 head: `906a68fdb41551ccaa6ce1394a2370e654ade10e`
- 대상 package: `messaging/sqsextended`
- 구현 경계: caller-owned AWS SDK for Go v2 `SQS`/`S3` client를 조합하는
  작은 Go-native adapter다. queue/bucket/lifecycle/IAM/DLQ/replay/live AWS는
  이 package가 소유하지 않는다.

## 목표

1. S3 object와 SQS body를 연결하는 versioned JSON envelope를 제공한다.
2. payload 업로드 후 envelope만 SQS에 전송하고, 수신 시 envelope 검증·payload
   read·SHA-256 검증을 순서대로 수행한다.
3. SQS delete를 먼저 실행한 다음 S3 object를 삭제하는 명시적 cleanup 계약을
   제공한다.
4. object put 이후 SQS send가 실패하면 object를 자동 삭제하지 않고 caller가
   orphan cleanup을 결정할 수 있도록 오류 상태와 `DeleteObject` capability를
   제공한다.
5. fake-first 테스트로 cancellation, output anomaly, missing object, failed
   delete, request deep-copy와 redaction을 검증한다.

## 비목표와 source-parity 결정

| 후보 | 결정 | 이유 |
|---|---|---|
| AWS Java/Python Extended Client 전체 parity | replace/split | Go에는 SQS 전체 wrapper가 필요하지 않으며 caller-owned lifecycle을 보존한다. |
| SQS queue provisioning/IAM/DLQ/replay | non-goal | 운영 topology와 재처리는 caller/operator 책임이다. |
| AWS client 생성·credential·retry·logger | non-goal | adapter는 narrow SDK surface만 호출하고 정책은 caller가 소유한다. |
| S3 object 자동 orphan sweeper | defer | retention/lifecycle와 충돌할 수 있으므로 이번 package는 명시적 수동 cleanup만 제공한다. |
| LocalStack/Floci live test | defer | 현재 S3-backed large-payload의 동작 계약을 검증했다는 근거가 없으므로 기본 CI에는 넣지 않는다. |

## Envelope 계약

`EnvelopeVersion`은 `1`이며 JSON field 순서와 map key 순서는 deterministic하게
encode한다.

```json
{
  "version": 1,
  "bucket": "orders-payloads",
  "key": "orders/order-42/payload.bin",
  "content_size": 1048577,
  "checksum": "sha256-hex-64",
  "content_type": "application/octet-stream",
  "encryption_metadata": {"algorithm":"aws:kms","key_id":"alias/orders"}
}
```

- `bucket`과 `key`는 caller가 제공하고 그대로 보존한다. blank, invalid UTF-8,
  S3 documented byte bound 초과는 거부한다.
- `content_size`는 payload byte 수와 exact match해야 하며, `checksum`은
  lowercase SHA-256 hex 64자리다.
- `content_type`은 선택 값이며 S3 `PutObject`의 `ContentType`에도 그대로
  전달한다.
- `encryption_metadata`는 선택적인 caller metadata map이다. provider는 값을
  해석하거나 KMS 호출을 하지 않고 envelope에만 복사한다.
- envelope는 bounded JSON이고 unknown/duplicate field, trailing bytes,
  non-canonical JSON, unsupported version을 거부한다.

## Public API와 ownership

`Provider`는 다음 SDK method subset만 요구한다.

```go
type SQSClient interface {
    SendMessage(context.Context, *sqs.SendMessageInput, ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
    ReceiveMessage(context.Context, *sqs.ReceiveMessageInput, ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
    DeleteMessage(context.Context, *sqs.DeleteMessageInput, ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

type S3Client interface {
    PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
    GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
    DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}
```

`Options`에는 두 client와 개별 `MaxPayloadSize`, Receive 누적
`MaxReceivePayloadSize`를 둔다. 기본 누적 bound는 512 MiB이며 개별 payload
bound 이상 512 MiB 이하의 bounded 값만 허용한다. provider는 client를 닫거나
  goroutine/retry/logger를 설치하지 않는다. client가 동시 호출과 cooperative
  context cancellation을 지원하면 provider도 concurrent `Send`/`Receive`에
  안전하다.

`SendRequest`는 `QueueURL`, `Bucket`, `Key`, `Payload`와 optional
`ContentType`/`EncryptionMetadata`를 받는다. `Send`는 S3 `PutObject` 성공 뒤
SQS `SendMessage`를 정확히 한 번 호출하고 `SendResult{MessageID,Envelope}`를
반환한다. S3 성공 후 SQS 실패 또는 cancellation으로 send 결과가 확정되지
않으면 `OrphanedObject()==true`를 반환하며 자동 delete하지 않는다.

`ReceiveRequest`는 `QueueURL`, optional `MaxNumberOfMessages`,
`VisibilityTimeout`, `WaitTimeSeconds`를 받는다. 수신된 각 SQS body를
decode하고 모든 envelope의 declared size 합을 누적 bound와 비교한 뒤 S3 object를
bounded read한다. 누적 bound를 초과하면 어떤 S3 object도 dispatch하지 않는다.
malformed envelope, missing object,
size/checksum mismatch는 SQS message를 삭제하지 않고 오류를 반환한다.
여러 message 중 일부가 성공한 경우 already-read values를 반환하지 않고
오류를 우선한다. caller는 visibility timeout과 processing budget을 맞춰야
하며 provider는 `ChangeMessageVisibility`를 암묵적으로 호출하지 않는다.
`ReceiveMessage` response 뒤 object read 전 cancellation에서는 receipt handle을
반환하지 않는다. visibility가 이미 시작될 수 있으므로 해당 batch의 retry,
visibility extension과 reconciliation은 caller가 소유한다.

`DeleteRequest`는 `QueueURL`, `ReceiptHandle`, `Envelope`를 받는다. SQS
`DeleteMessage`가 성공한 뒤 cancellation checkpoint를 확인하고 S3
`DeleteObject`를 호출한다. SQS delete 실패 시 S3 delete를 호출하지 않는다.
S3 delete 실패는 `ErrObjectDeleteFailed`이며 이미 queue message가 ack됐다는
사실을 `QueueDeleted()==true`로 관찰할 수 있다. cancellation이 각 side
effect 직후 발생하면 `ErrCanceled`와 원인 context error를 함께 반환하고,
`QueueDeleted()==true` 또는 `OrphanedObject()==true`로 확인된 cleanup 상태를
caller가 reconciliation해야 한다.

## 오류·cancellation 계약

오류 문자열은 고정된 sentinel과 allowlisted operation만 포함하고 queue URL,
bucket/key, payload, checksum, AWS provider message를 포함하지 않는다.
`errors.Is`로 package sentinel과 injected cause를 관찰할 수 있고
`errors.As`로 `*Error`를 얻는다. `fmt.Sprintf("%+v", err)`와 `%#v`도 redacted다.

모든 public IO method는 nil context를 `context.Background()`로 정규화하고,
dispatch 전·각 provider response 직후·최종 결과 publication 직전에
`ctx.Err()`를 확인한다. caller cancellation은 SDK 오류나 성공 response보다
우선한다. side effect가 이미 완료된 뒤 취소되면 `ErrCanceled`를 사용해
`errors.Is(err, context.Canceled)`를 유지하면서 cleanup 상태를 보존한다.
provider는 cancellation을 retry하거나 late result를 publish하지 않는다.

## Test 계약

- envelope encode/decode round-trip, field validation, unknown/duplicate/
  trailing/non-canonical/version mismatch, size/checksum mismatch
- constructor nil/typed-nil client와 bounded option
- `Send` request deep-copy, S3→SQS 순서, S3 failure, SQS failure/orphan flag,
  nil/malformed outputs, side-effect cancellation과 orphan flag
- `Receive` envelope parsing, aggregate payload preflight, bounded body read/close, missing object,
  checksum/size mismatch, partial batch error와 no delete
- `Delete` SQS-first order, SQS failure no object delete, object delete failure,
  side-effect cancellation과 queue-deleted flag
- error redaction including `%+v`, `errors.Is`/`errors.As`, concurrent fake
  isolation under `go test -race`

기본 CI는 fake client와 compile-checked example만 사용한다. real AWS credentials,
unverified emulator, queue/bucket provisioning은 DoD에 포함하지 않는다.

## SPW trace

- SPW-01 requirements: live issue #523, parent #517, AWS research gate.
- SPW-02 design: 이 문서의 envelope, ownership, failure/cancellation 계약.
- SPW-03 plan: `docs/superpowers/plans/2026-09-04-issue-523-sqs-s3-envelope-plan.md`.
- SPW-04 implementation: RED fake/tests → GREEN package implementation.
- SPW-05 verification: package tests, race, vet, fmt/tidy, repository checks와
  `docs/review/` evidence.
