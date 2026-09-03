# messaging/sqsextended

[English](README.md) | [한국어](README.ko.md)

`messaging/sqsextended` is a small AWS SDK for Go v2 adapter for payloads that
are too large or expensive to place directly in SQS. It stores the bytes in a
caller-selected S3 object and sends a versioned JSON pointer envelope through
SQS.

## Usage

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
	// Process message.Payload before acknowledging it.
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

`SQSClient` and `S3Client` are the smallest method subsets required by the
provider. The caller creates, configures, retries, observes, and closes those
clients. The provider does not load credentials, provision queues or buckets,
install a logger, extend visibility, or run a background cleanup worker.

## Envelope

`Send` computes a SHA-256 digest and emits a canonical, bounded envelope:

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

`Bucket`, `Key`, content size, checksum, content type, and optional descriptive
encryption metadata are preserved in the wire contract. `EncodeEnvelope` and
`DecodeEnvelope` reject unsupported versions, unknown/duplicate fields,
non-canonical JSON, invalid UTF-8, malformed checksums, and oversized input.
The default per-object payload bound is `DefaultMaxPayloadSize` (256 MiB); configure a
smaller bound with `Options.MaxPayloadSize` when the application has a tighter
budget. `Receive` also enforces a 512 MiB aggregate payload budget by default
(`DefaultMaxReceivePayloadSize`). Configure a lower bounded value with
`Options.MaxReceivePayloadSize`; envelopes are preflighted before any S3 object
is dispatched so an over-budget batch performs no partial reads.

## Failure and cleanup semantics

- `Send` always performs `S3 PutObject` before one `SQS SendMessage`. A failed
  put never sends a queue message.
- If S3 succeeds but SQS fails or returns an unusable response, the object is
  intentionally not deleted. The returned `*sqsextended.Error` reports
  `OrphanedObject() == true`; call `DeleteObject` explicitly or rely on the
  caller-owned S3 lifecycle policy.
- If cancellation is observed immediately after the S3 put or SQS send,
  `ErrCanceled` is returned while `errors.Is(err, context.Canceled)` remains
  true. The object may be orphaned, so inspect `OrphanedObject()` and reconcile
  it explicitly.
- `Receive` validates the envelope, reads at most the declared payload size plus
  one byte, closes the S3 response body, and verifies exact size and SHA-256.
  Missing objects, malformed responses, size mismatches, and checksum failures
  leave the SQS message unacknowledged.
- `Delete` calls SQS `DeleteMessage` first and only then calls S3
  `DeleteObject`. If SQS deletion fails, S3 is not touched. If S3 cleanup fails,
  the returned error reports `QueueDeleted() == true`, so the caller can route
  cleanup to a separate repair/lifecycle process.
- If cancellation is observed immediately after either delete side effect,
  `ErrCanceled` is returned with `QueueDeleted() == true`; the caller can use
  that state to reconcile object cleanup without retrying the queue ack.
- Context cancellation is checked before dispatch and after every SDK response.
  Cancellation wins over a late success response. The provider never retries a
  canceled operation or extends visibility implicitly.

An S3-backed message can take longer to read and process than the queue's
visibility timeout. The caller must pass a suitable `VisibilityTimeout` or own
an explicit visibility-extension policy. This package does not change that
policy, delete messages automatically, or guarantee exactly-once delivery.

## Operational ownership

The caller/operator owns queue and bucket provisioning, IAM, encryption/KMS
policy, retention/lifecycle, DLQ, replay, deduplication, client credentials,
timeouts, retry/backoff, metrics, logging, and production rollout. The package
does not claim LocalStack, Floci, or live AWS compatibility in default CI.

## Tests

```bash
go test -count=1 ./messaging/sqsextended
go test -race -count=1 ./messaging/sqsextended
go vet ./messaging/sqsextended
```

The tests use mutex-safe fake clients only. They cover canonical envelope
encoding, failure ordering, orphan and cleanup status, bounded reads,
checksum validation, cancellation, redacted errors, and concurrent request
isolation without credentials or Docker.
