# messaging/sqsextended

[English](README.md) | [한국어](README.ko.md)

`messaging/sqsextended`는 SQS에 직접 넣기 어렵거나 큰 payload를 caller가
선택한 S3 object에 저장하고, versioned JSON pointer envelope만 SQS로 보내는
작은 AWS SDK for Go v2 adapter입니다.

## 사용법

```go
provider, err := sqsextended.New(sqsextended.Options{
	SQSClient: sqs.NewFromConfig(cfg),
	S3Client:  s3.NewFromConfig(cfg),
})
if err != nil {
	return err
}

sent, err := provider.Send(ctx, sqsextended.SendRequest{
	QueueURL:    queueURL,
	Bucket:      "order-payloads",
	Key:         "orders/42/payload.json",
	Payload:     payload,
	ContentType: "application/json",
})
if err != nil {
	return err
}

messages, err := provider.Receive(ctx, sqsextended.ReceiveRequest{
	QueueURL:          queueURL,
	VisibilityTimeout: 120,
})
if err != nil {
	return err
}
for _, message := range messages {
	// 처리 성공 후에만 message를 acknowledge합니다.
	if err := process(message.Payload); err != nil {
		return err
	}
	if err := provider.Delete(ctx, sqsextended.DeleteRequest{
		QueueURL:      message.QueueURL,
		ReceiptHandle: message.ReceiptHandle,
		Envelope:      message.Envelope,
	}); err != nil {
		return err
	}
}
_ = sent
```

`SQSClient`와 `S3Client`는 provider가 실제로 호출하는 최소 method subset입니다.
client 생성, 설정, retry, 관측과 close는 caller가 소유합니다. provider는
credential을 로드하거나 queue/bucket을 provision하거나 logger를 설치하거나
visibility를 연장하거나 background cleanup worker를 실행하지 않습니다.

## Envelope

`Send`는 payload의 SHA-256 digest를 계산하고 bounded canonical envelope를
생성합니다.

```json
{
  "version": 1,
  "bucket": "order-payloads",
  "key": "orders/42/payload.json",
  "content_size": 1048577,
  "checksum": "lowercase-sha256-hex",
  "content_type": "application/json",
  "encryption_metadata": {"algorithm":"aws:kms","key_id":"alias/orders"}
}
```

`Bucket`, `Key`, content size, checksum, content type와 선택적 descriptive
encryption metadata를 wire contract에 보존합니다. `EncodeEnvelope`와
`DecodeEnvelope`는 unsupported version, unknown/duplicate field, non-canonical
JSON, invalid UTF-8, malformed checksum과 oversized input을 거부합니다.
기본 payload bound는 `DefaultMaxPayloadSize` (256 MiB)이며, application이 더
작은 budget을 가지면 `Options.MaxPayloadSize`로 줄일 수 있습니다.

## Failure와 cleanup 의미

- `Send`는 항상 `S3 PutObject`를 먼저 호출한 뒤 SQS `SendMessage`를 정확히
  한 번 호출합니다. put이 실패하면 queue message를 보내지 않습니다.
- S3가 성공한 뒤 SQS가 실패하거나 사용할 수 없는 response를 반환하면 object를
  자동 삭제하지 않습니다. 반환된 `*sqsextended.Error`의
  `OrphanedObject() == true`를 확인하고 `DeleteObject`를 명시적으로 호출하거나
  caller-owned S3 lifecycle policy에 맡깁니다.
- `Receive`는 envelope를 검증하고 declared size + 1 byte까지만 읽고 S3
  response body를 close한 뒤 exact size와 SHA-256을 확인합니다. missing object,
  malformed response, size mismatch와 checksum failure에서는 SQS message를
  acknowledge하지 않습니다.
- `Delete`는 SQS `DeleteMessage`를 먼저 실행하고 성공한 경우에만 S3
  `DeleteObject`를 실행합니다. SQS 삭제가 실패하면 S3를 호출하지 않습니다.
  S3 cleanup이 실패하면 반환 error의 `QueueDeleted() == true`로 queue ack
  완료를 확인할 수 있으므로 별도 repair/lifecycle process로 cleanup할 수
  있습니다.
- SDK 호출 전과 각 response 직후에 context를 확인합니다. 늦게 도착한 성공
  response보다 cancellation이 우선하며, 취소된 작업을 retry하거나 visibility를
  암묵적으로 연장하지 않습니다.

S3-backed message read와 processing이 queue visibility timeout보다 오래 걸릴
수 있습니다. Caller는 충분한 `VisibilityTimeout`을 전달하거나 명시적인
visibility-extension policy를 직접 소유해야 합니다. 이 package는 해당 정책을
변경하지 않고, message를 자동 삭제하지 않으며, exactly-once delivery를
보장하지 않습니다.

## 운영 ownership

queue/bucket provisioning, IAM, encryption/KMS policy, retention/lifecycle,
DLQ, replay, deduplication, client credential, timeout, retry/backoff, metric,
logging과 production rollout은 caller/operator 책임입니다. 기본 CI는
LocalStack, Floci 또는 live AWS compatibility를 주장하지 않습니다.

## Tests

```bash
go test -count=1 ./messaging/sqsextended
go test -race -count=1 ./messaging/sqsextended
go vet ./messaging/sqsextended
```

테스트는 credential이나 Docker 없이 mutex-safe fake client만 사용합니다.
Canonical envelope encoding, failure order, orphan/cleanup 상태, bounded read,
checksum validation, cancellation, redacted error와 concurrent request
isolation을 검증합니다.
