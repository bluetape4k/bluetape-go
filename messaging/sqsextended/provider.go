package sqsextended

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// SQSClient - Provider가 사용하는 최소 SQS SDK 표면이다.
// client 생성, credential, retry, timeout과 lifecycle은 호출자가 소유한다.
type SQSClient interface {
	SendMessage(context.Context, *sqs.SendMessageInput, ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(context.Context, *sqs.ReceiveMessageInput, ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(context.Context, *sqs.DeleteMessageInput, ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

// S3Client - Provider가 사용하는 최소 S3 SDK 표면이다.
// client 생성, credential, retry, timeout과 lifecycle은 호출자가 소유한다.
type S3Client interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

var _ SQSClient = (*sqs.Client)(nil)
var _ S3Client = (*s3.Client)(nil)

// Options - 호출자가 소유한 SQS와 S3 client로 Provider를 구성한다.
type Options struct {
	// SQSClient는 envelope message를 수신하고 acknowledge한다.
	SQSClient SQSClient
	// S3Client는 payload object를 저장하고 읽고 삭제한다.
	S3Client S3Client
	// MaxPayloadSize는 허용하고 읽을 payload byte 수의 상한이다. 0이면
	// DefaultMaxPayloadSize를 사용한다.
	MaxPayloadSize int64
}

// Provider - SQS client 하나와 S3 client 하나를 결합하지만 어느 client의
// lifecycle이나 운영 정책도 소유하지 않는다.
type Provider struct {
	sqsClient      SQSClient
	s3Client       S3Client
	maxPayloadSize int64
}

// New - client와 payload 상한을 검증하고 변경할 수 없는 Provider를 반환한다.
func New(options Options) (*Provider, error) {
	if isNilClient(options.SQSClient) || isNilClient(options.S3Client) {
		return nil, newError(ErrNilClient, "validate options", nil, false, false)
	}
	maxPayloadSize := options.MaxPayloadSize
	if maxPayloadSize == 0 {
		maxPayloadSize = DefaultMaxPayloadSize
	}
	if maxPayloadSize < 1 || maxPayloadSize > DefaultMaxPayloadSize {
		return nil, newError(ErrInvalidOptions, "validate options", nil, false, false)
	}
	return &Provider{
		sqsClient:      options.SQSClient,
		s3Client:       options.S3Client,
		maxPayloadSize: maxPayloadSize,
	}, nil
}

// MaxPayloadSize - 구성된 payload 상한을 반환하며, Provider가 nil이면 0을
// 반환한다.
func (p *Provider) MaxPayloadSize() int64 {
	if p == nil {
		return 0
	}
	return p.maxPayloadSize
}

// SendRequest - 하나의 payload object와 대상 queue를 기술한다.
type SendRequest struct {
	// QueueURL은 호출자가 선택한 SQS queue URL이다.
	QueueURL string
	// Bucket은 호출자가 선택한 S3 bucket 또는 access-point identifier이다.
	Bucket string
	// Key는 호출자가 선택한 S3 object key이다. provider는 key를 생성하지 않는다.
	Key string
	// Payload는 SQS를 호출하기 전에 S3 request reader에 복사된다.
	Payload []byte
	// ContentType은 선택 사항이며 설정하면 변경하지 않고 S3에 전달한다.
	ContentType string
	// EncryptionMetadata는 설명용 envelope metadata이다. 이 package는 이를
	// 복사할 뿐 key나 credential로 해석하지 않는다.
	EncryptionMetadata map[string]string
}

// SendResult - SQS가 할당한 message ID와 실제로 전송한 envelope를 담는다.
type SendResult struct {
	MessageID string
	Envelope  Envelope
}

// Send - payload를 S3에 저장한 뒤 envelope body 하나를 SQS에 전송한다.
// S3가 성공했지만 SQS가 실패하면 object를 의도적으로 남기며, 반환하는
// *Error에서 OrphanedObject() == true를 확인할 수 있다.
func (p *Provider) Send(ctx context.Context, request SendRequest) (*SendResult, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	if err := p.validateSendRequest(request); err != nil {
		return nil, err
	}
	payload := append([]byte(nil), request.Payload...)
	envelope := Envelope{
		Version:            EnvelopeVersion,
		Bucket:             request.Bucket,
		Key:                request.Key,
		ContentSize:        int64(len(payload)),
		Checksum:           payloadChecksum(payload),
		ContentType:        request.ContentType,
		EncryptionMetadata: cloneMetadata(request.EncryptionMetadata),
	}
	envelopeBody, err := EncodeEnvelope(envelope)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	checksum := sha256.Sum256(payload)
	contentLength := int64(len(payload))
	s3Output, callErr := p.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:            aws.String(request.Bucket),
		Key:               aws.String(request.Key),
		Body:              bytes.NewReader(payload),
		ContentLength:     &contentLength,
		ContentType:       optionalString(request.ContentType),
		ChecksumAlgorithm: s3types.ChecksumAlgorithmSha256,
		ChecksumSHA256:    aws.String(base64.StdEncoding.EncodeToString(checksum[:])),
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if callErr != nil {
		return nil, newError(ErrObjectPutFailed, "put object", callErr, s3Output != nil, false)
	}
	if s3Output == nil {
		return nil, newError(ErrMalformedOutput, "put object", nil, true, false)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output, callErr := p.sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(request.QueueURL),
		MessageBody: aws.String(string(envelopeBody)),
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if callErr != nil {
		return nil, newError(ErrMessageSendFailed, "send message", callErr, true, false)
	}
	if output == nil || output.MessageId == nil || !validRequiredString(aws.ToString(output.MessageId), 128) {
		return nil, newError(ErrMalformedOutput, "send message", nil, true, false)
	}
	return &SendResult{MessageID: aws.ToString(output.MessageId), Envelope: envelope}, nil
}

// ReceiveRequest - 하나의 SQS ReceiveMessage 호출을 구성한다.
type ReceiveRequest struct {
	// QueueURL은 호출자가 선택한 SQS queue URL이다.
	QueueURL string
	// MaxNumberOfMessages는 기본값이 1이며 1에서 10 사이로 지정할 수 있다.
	MaxNumberOfMessages int32
	// VisibilityTimeout이 양수이면 그대로 전달한다. S3 read와 processing
	// 시간을 포함하도록 호출자가 값을 선택해야 하며, 암묵적으로 연장하지 않는다.
	VisibilityTimeout int32
	// WaitTimeSeconds가 양수이면 그대로 전달하며 20을 초과할 수 없다.
	WaitTimeSeconds int32
}

// ReceivedMessage - 검증된 SQS envelope, payload와 receipt handle이다.
// Receive는 acknowledge하지 않으므로 processing이 성공한 뒤 Delete를 호출한다.
type ReceivedMessage struct {
	QueueURL      string
	MessageID     string
	ReceiptHandle string
	Envelope      Envelope
	Payload       []byte
}

// Receive - SQS envelope를 수신하고 참조된 S3 object를 읽어 검증한다.
// message를 자동으로 acknowledge하지 않는다. 수신한 message 중 하나라도
// 검증할 수 없으면 nil slice를 반환하고 caller의 retry 또는 DLQ 정책에 맡긴다.
func (p *Provider) Receive(ctx context.Context, request ReceiveRequest) ([]ReceivedMessage, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	maxMessages := request.MaxNumberOfMessages
	if maxMessages == 0 {
		maxMessages = 1
	}
	if !validRequiredString(request.QueueURL, 2048) || maxMessages < 1 || maxMessages > 10 || request.VisibilityTimeout < 0 || request.VisibilityTimeout > 43200 || request.WaitTimeSeconds < 0 || request.WaitTimeSeconds > 20 {
		return nil, newError(ErrInvalidRequest, "validate request", nil, false, false)
	}
	input := &sqs.ReceiveMessageInput{QueueUrl: aws.String(request.QueueURL), MaxNumberOfMessages: maxMessages}
	if request.VisibilityTimeout > 0 {
		input.VisibilityTimeout = request.VisibilityTimeout
	}
	if request.WaitTimeSeconds > 0 {
		input.WaitTimeSeconds = request.WaitTimeSeconds
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output, callErr := p.sqsClient.ReceiveMessage(ctx, input)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if callErr != nil {
		return nil, newError(ErrReceiveFailed, "receive", callErr, false, false)
	}
	if output == nil {
		return nil, newError(ErrMalformedOutput, "receive", nil, false, false)
	}
	if len(output.Messages) > 10 {
		return nil, newError(ErrMalformedOutput, "receive", nil, false, false)
	}
	messages := make([]ReceivedMessage, 0, len(output.Messages))
	for _, message := range output.Messages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if message.Body == nil {
			return nil, newError(ErrMalformedOutput, "receive", nil, false, false)
		}
		body := aws.ToString(message.Body)
		if len(body) == 0 || len(body) > MaxEnvelopeSize || !validRequiredString(aws.ToString(message.MessageId), 128) || !validRequiredString(aws.ToString(message.ReceiptHandle), 1024) {
			return nil, newError(ErrMalformedOutput, "receive", nil, false, false)
		}
		envelope, err := DecodeEnvelope([]byte(body))
		if err != nil {
			return nil, err
		}
		if envelope.ContentSize > p.maxPayloadSize {
			return nil, newError(ErrPayloadTooLarge, "receive", nil, false, false)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		object, callErr := p.s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(envelope.Bucket),
			Key:    aws.String(envelope.Key),
		})
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if callErr != nil {
			closeObjectBody(object)
			return nil, newError(ErrObjectReadFailed, "get object", callErr, false, false)
		}
		payload, err := readPayload(ctx, object, envelope.ContentSize)
		if err != nil {
			return nil, err
		}
		if got := payloadChecksum(payload); got != envelope.Checksum {
			return nil, newError(ErrChecksumMismatch, "receive", nil, false, false)
		}
		messages = append(messages, ReceivedMessage{
			QueueURL:      request.QueueURL,
			MessageID:     aws.ToString(message.MessageId),
			ReceiptHandle: aws.ToString(message.ReceiptHandle),
			Envelope:      envelope,
			Payload:       payload,
		})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

// DeleteRequest - 수신한 message와 해당 S3 envelope를 식별한다.
type DeleteRequest struct {
	QueueURL      string
	ReceiptHandle string
	Envelope      Envelope
}

// Delete - 먼저 SQS를 acknowledge한 다음 S3 object를 삭제한다. object
// cleanup이 실패하면 반환하는 *Error에서 QueueDeleted() == true를 확인할 수 있다.
func (p *Provider) Delete(ctx context.Context, request DeleteRequest) error {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.validate(); err != nil {
		return err
	}
	if !validRequiredString(request.QueueURL, 2048) || !validRequiredString(request.ReceiptHandle, 1024) {
		return newError(ErrInvalidRequest, "validate request", nil, false, false)
	}
	if err := validateEnvelope(request.Envelope); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	sqsOutput, callErr := p.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(request.QueueURL),
		ReceiptHandle: aws.String(request.ReceiptHandle),
	})
	if err := ctx.Err(); err != nil {
		return err
	}
	if callErr != nil {
		return newError(ErrMessageDeleteFailed, "delete message", callErr, false, false)
	}
	if sqsOutput == nil {
		return newError(ErrMalformedOutput, "delete message", nil, false, false)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s3Output, callErr := p.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(request.Envelope.Bucket),
		Key:    aws.String(request.Envelope.Key),
	})
	if err := ctx.Err(); err != nil {
		return err
	}
	if callErr != nil {
		return newError(ErrObjectDeleteFailed, "delete object", callErr, false, true)
	}
	if s3Output == nil {
		return newError(ErrMalformedOutput, "delete object", nil, false, true)
	}
	return nil
}

// DeleteObject - 예를 들어 OrphanedObject()로 보고된 send error 뒤에 S3
// object를 명시적으로 삭제한다. SQS에는 접근하지 않는다.
func (p *Provider) DeleteObject(ctx context.Context, envelope Envelope) error {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.validate(); err != nil {
		return err
	}
	if err := validateEnvelope(envelope); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	output, callErr := p.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(envelope.Bucket),
		Key:    aws.String(envelope.Key),
	})
	if err := ctx.Err(); err != nil {
		return err
	}
	if callErr != nil {
		return newError(ErrObjectDeleteFailed, "delete object", callErr, false, false)
	}
	if output == nil {
		return newError(ErrMalformedOutput, "delete object", nil, false, false)
	}
	return nil
}

func (p *Provider) validate() error {
	if p == nil || isNilClient(p.sqsClient) || isNilClient(p.s3Client) || p.maxPayloadSize < 1 {
		return newError(ErrInvalidOptions, "validate options", nil, false, false)
	}
	return nil
}

func (p *Provider) validateSendRequest(request SendRequest) error {
	if !validRequiredString(request.QueueURL, 2048) || !validRequiredString(request.Bucket, maxBucketSize) || !validRequiredString(request.Key, maxObjectKeySize) {
		return newError(ErrInvalidRequest, "validate request", nil, false, false)
	}
	if int64(len(request.Payload)) > p.maxPayloadSize {
		return newError(ErrPayloadTooLarge, "validate request", nil, false, false)
	}
	if request.ContentType != "" && (!utf8.ValidString(request.ContentType) || strings.TrimSpace(request.ContentType) == "" || len(request.ContentType) > maxContentTypeSize) {
		return newError(ErrInvalidRequest, "validate request", nil, false, false)
	}
	if len(request.EncryptionMetadata) > maxMetadataEntries {
		return newError(ErrInvalidRequest, "validate request", nil, false, false)
	}
	for key, value := range request.EncryptionMetadata {
		if !validRequiredString(key, maxMetadataKeySize) || !validString(value, maxMetadataValSize) {
			return newError(ErrInvalidRequest, "validate request", nil, false, false)
		}
	}
	return nil
}

func readPayload(ctx context.Context, output *s3.GetObjectOutput, expected int64) ([]byte, error) {
	if output == nil || output.Body == nil {
		return nil, newError(ErrMalformedOutput, "get object", nil, false, false)
	}
	if expected < 0 || expected > DefaultMaxPayloadSize {
		_ = output.Body.Close()
		return nil, newError(ErrPayloadTooLarge, "read object", nil, false, false)
	}
	payload, readErr := io.ReadAll(io.LimitReader(output.Body, expected+1))
	closeErr := output.Body.Close()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if readErr != nil {
		return nil, newError(ErrObjectReadFailed, "read object", readErr, false, false)
	}
	if closeErr != nil {
		return nil, newError(ErrObjectReadFailed, "read object", closeErr, false, false)
	}
	if output.ContentLength != nil && *output.ContentLength != int64(len(payload)) {
		return nil, newError(ErrPayloadSizeMismatch, "read object", nil, false, false)
	}
	if int64(len(payload)) != expected {
		return nil, newError(ErrPayloadSizeMismatch, "read object", nil, false, false)
	}
	return payload, nil
}

func closeObjectBody(output *s3.GetObjectOutput) {
	if output != nil && output.Body != nil {
		_ = output.Body.Close()
	}
}

func payloadChecksum(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return aws.String(value)
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func isNilClient(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
