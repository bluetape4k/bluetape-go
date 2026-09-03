package sqsextended

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type fakeSQSClient struct {
	mu          sync.Mutex
	puts        int
	receives    int
	deletes     int
	lastSend    *sqs.SendMessageInput
	lastReceive *sqs.ReceiveMessageInput
	lastDelete  *sqs.DeleteMessageInput
	sendOutput  *sqs.SendMessageOutput
	receiveOut  *sqs.ReceiveMessageOutput
	deleteOut   *sqs.DeleteMessageOutput
	sendErr     error
	receiveErr  error
	deleteErr   error
	order       *[]string
	cancelAfter func()
	lastContext context.Context
}

func (f *fakeSQSClient) SendMessage(ctx context.Context, input *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	f.mu.Lock()
	f.puts++
	f.lastSend = cloneSendInput(input)
	f.lastContext = ctx
	output, err := cloneSendOutput(f.sendOutput), f.sendErr
	order, cancelAfter := f.order, f.cancelAfter
	f.mu.Unlock()
	if order != nil {
		*order = append(*order, "sqs.send")
	}
	if cancelAfter != nil {
		cancelAfter()
	}
	if err != nil {
		return output, err
	}
	if err := ctx.Err(); err != nil {
		return output, err
	}
	return output, nil
}

func (f *fakeSQSClient) ReceiveMessage(ctx context.Context, input *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	f.mu.Lock()
	f.receives++
	f.lastReceive = cloneReceiveInput(input)
	f.lastContext = ctx
	output, err := cloneReceiveOutput(f.receiveOut), f.receiveErr
	order := f.order
	f.mu.Unlock()
	if order != nil {
		*order = append(*order, "sqs.receive")
	}
	if err != nil {
		return output, err
	}
	if err := ctx.Err(); err != nil {
		return output, err
	}
	return output, nil
}

func (f *fakeSQSClient) DeleteMessage(ctx context.Context, input *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	f.mu.Lock()
	f.deletes++
	f.lastDelete = cloneDeleteInput(input)
	f.lastContext = ctx
	output, err := cloneDeleteOutput(f.deleteOut), f.deleteErr
	order, cancelAfter := f.order, f.cancelAfter
	f.mu.Unlock()
	if order != nil {
		*order = append(*order, "sqs.delete")
	}
	if cancelAfter != nil {
		cancelAfter()
	}
	if err != nil {
		return output, err
	}
	if err := ctx.Err(); err != nil {
		return output, err
	}
	return output, nil
}

type fakeS3Client struct {
	mu          sync.Mutex
	puts        int
	gets        int
	deletes     int
	lastPut     *s3.PutObjectInput
	lastGet     *s3.GetObjectInput
	lastDelete  *s3.DeleteObjectInput
	putOutput   *s3.PutObjectOutput
	getOutput   *s3.GetObjectOutput
	getBody     io.ReadCloser
	deleteOut   *s3.DeleteObjectOutput
	putErr      error
	getErr      error
	deleteErr   error
	order       *[]string
	cancelAfter func()
	lastContext context.Context
}

type countingReadCloser struct {
	io.Reader
	onClose func()
}

func (r *countingReadCloser) Close() error {
	if r.onClose != nil {
		r.onClose()
	}
	return nil
}

func (f *fakeS3Client) PutObject(ctx context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.mu.Lock()
	f.puts++
	f.lastPut = clonePutInput(input)
	f.lastContext = ctx
	output, err := f.putOutput, f.putErr
	order, cancelAfter := f.order, f.cancelAfter
	f.mu.Unlock()
	if order != nil {
		*order = append(*order, "s3.put")
	}
	if cancelAfter != nil {
		cancelAfter()
	}
	if err != nil {
		return output, err
	}
	if err := ctx.Err(); err != nil {
		return output, err
	}
	return output, nil
}

func (f *fakeS3Client) GetObject(ctx context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	f.mu.Lock()
	f.gets++
	f.lastGet = cloneGetInput(input)
	f.lastContext = ctx
	output, err := cloneGetOutput(f.getOutput), f.getErr
	if f.getBody != nil {
		output = &s3.GetObjectOutput{Body: f.getBody}
	}
	order, cancelAfter := f.order, f.cancelAfter
	f.mu.Unlock()
	if order != nil {
		*order = append(*order, "s3.get")
	}
	if cancelAfter != nil {
		cancelAfter()
	}
	if err != nil {
		return output, err
	}
	if err := ctx.Err(); err != nil {
		return output, err
	}
	return output, nil
}

func (f *fakeS3Client) DeleteObject(ctx context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	f.mu.Lock()
	f.deletes++
	f.lastDelete = cloneDeleteObjectInput(input)
	f.lastContext = ctx
	output, err := f.deleteOut, f.deleteErr
	order := f.order
	f.mu.Unlock()
	if order != nil {
		*order = append(*order, "s3.delete")
	}
	if err != nil {
		return output, err
	}
	if err := ctx.Err(); err != nil {
		return output, err
	}
	return output, nil
}

func TestNewRejectsNilAndInvalidOptions(t *testing.T) {
	fakeSQS := &fakeSQSClient{}
	fakeS3 := &fakeS3Client{}
	tests := []struct {
		name    string
		options Options
		want    error
	}{
		{name: "nil sqs", options: Options{S3Client: fakeS3}, want: ErrNilClient},
		{name: "nil s3", options: Options{SQSClient: fakeSQS}, want: ErrNilClient},
		{name: "typed nil sqs", options: Options{SQSClient: (*fakeSQSClient)(nil), S3Client: fakeS3}, want: ErrNilClient},
		{name: "typed nil s3", options: Options{SQSClient: fakeSQS, S3Client: (*fakeS3Client)(nil)}, want: ErrNilClient},
		{name: "zero max", options: Options{SQSClient: fakeSQS, S3Client: fakeS3, MaxPayloadSize: -1}, want: ErrInvalidOptions},
		{name: "receive budget below payload", options: Options{SQSClient: fakeSQS, S3Client: fakeS3, MaxReceivePayloadSize: DefaultMaxPayloadSize - 1}, want: ErrInvalidOptions},
		{name: "receive budget above hard bound", options: Options{SQSClient: fakeSQS, S3Client: fakeS3, MaxReceivePayloadSize: DefaultMaxReceivePayloadSize + 1}, want: ErrInvalidOptions},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.options); !errors.Is(err, tt.want) {
				t.Fatalf("New error = %v, want %v", err, tt.want)
			}
		})
	}
	provider, err := New(Options{SQSClient: fakeSQS, S3Client: fakeS3})
	if err != nil {
		t.Fatalf("New defaults: %v", err)
	}
	if provider.MaxPayloadSize() != DefaultMaxPayloadSize {
		t.Fatalf("MaxPayloadSize = %d, want %d", provider.MaxPayloadSize(), DefaultMaxPayloadSize)
	}
	if provider.MaxReceivePayloadSize() != DefaultMaxReceivePayloadSize {
		t.Fatalf("MaxReceivePayloadSize = %d, want %d", provider.MaxReceivePayloadSize(), DefaultMaxReceivePayloadSize)
	}
}

func TestSendStoresObjectBeforeSendingEnvelope(t *testing.T) {
	order := make([]string, 0, 2)
	s3Client := &fakeS3Client{putOutput: &s3.PutObjectOutput{}, order: &order}
	sqsClient := &fakeSQSClient{sendOutput: &sqs.SendMessageOutput{MessageId: aws.String("message-1")}, order: &order}
	provider := mustProvider(t, sqsClient, s3Client)
	payload := []byte("hello world")
	result, err := provider.Send(context.Background(), SendRequest{
		QueueURL:           "https://sqs.example/queue",
		Bucket:             "payloads",
		Key:                "orders/42/payload.txt",
		Payload:            payload,
		ContentType:        "text/plain",
		EncryptionMetadata: map[string]string{"algorithm": "aws:kms"},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if diff := fmt.Sprintf("%v", order); diff != "[s3.put sqs.send]" {
		t.Fatalf("call order = %s, want [s3.put sqs.send]", diff)
	}
	if result.MessageID != "message-1" {
		t.Fatalf("message ID = %q, want message-1", result.MessageID)
	}
	if result.Envelope.ContentSize != int64(len(payload)) || result.Envelope.Bucket != "payloads" || result.Envelope.Key != "orders/42/payload.txt" {
		t.Fatalf("envelope = %#v", result.Envelope)
	}
	if result.Envelope.Checksum != checksum(payload) {
		t.Fatalf("checksum = %q, want %q", result.Envelope.Checksum, checksum(payload))
	}
	if sqsClient.lastSend == nil || sqsClient.lastSend.MessageBody == nil {
		t.Fatal("missing SQS message body")
	}
	decoded, err := DecodeEnvelope([]byte(*sqsClient.lastSend.MessageBody))
	if err != nil {
		t.Fatalf("decode sent envelope: %v", err)
	}
	if !reflect.DeepEqual(decoded, result.Envelope) {
		t.Fatalf("sent envelope = %#v, want %#v", decoded, result.Envelope)
	}
	if s3Client.lastPut == nil || s3Client.lastPut.Bucket == nil || *s3Client.lastPut.Bucket != "payloads" || s3Client.lastPut.Key == nil || *s3Client.lastPut.Key != "orders/42/payload.txt" {
		t.Fatalf("S3 put input = %#v", s3Client.lastPut)
	}
	if got, err := io.ReadAll(s3Client.lastPut.Body); err != nil || string(got) != string(payload) {
		t.Fatalf("S3 payload = %q, err=%v", got, err)
	}
	if s3Client.lastPut.ContentLength == nil || *s3Client.lastPut.ContentLength != int64(len(payload)) {
		t.Fatalf("S3 content length = %#v", s3Client.lastPut.ContentLength)
	}
	if s3Client.lastPut.ContentType == nil || *s3Client.lastPut.ContentType != "text/plain" {
		t.Fatalf("S3 content type = %#v", s3Client.lastPut.ContentType)
	}
}

func TestSendFailureMatrixAndOrphanContract(t *testing.T) {
	putCause := errors.New("s3 provider secret object key")
	s3Client := &fakeS3Client{putErr: putCause}
	sqsClient := &fakeSQSClient{sendOutput: &sqs.SendMessageOutput{MessageId: aws.String("message")}}
	provider := mustProvider(t, sqsClient, s3Client)
	request := validSendRequest()
	_, err := provider.Send(context.Background(), request)
	if !errors.Is(err, ErrObjectPutFailed) || !errors.Is(err, putCause) {
		t.Fatalf("S3 failure = %v, want object-put and cause", err)
	}
	if sqsClient.puts != 0 {
		t.Fatalf("SQS calls after S3 failure = %d, want 0", sqsClient.puts)
	}

	s3Client = &fakeS3Client{putOutput: &s3.PutObjectOutput{}}
	sendCause := errors.New("sqs provider secret queue")
	sqsClient = &fakeSQSClient{sendErr: sendCause}
	provider = mustProvider(t, sqsClient, s3Client)
	_, err = provider.Send(context.Background(), request)
	if !errors.Is(err, ErrMessageSendFailed) || !errors.Is(err, sendCause) {
		t.Fatalf("SQS failure = %v, want message-send and cause", err)
	}
	var operationError *Error
	if !errors.As(err, &operationError) || !operationError.OrphanedObject() {
		t.Fatalf("SQS failure = %#v, want orphaned object error", err)
	}
	if s3Client.deletes != 0 {
		t.Fatalf("automatic S3 deletes = %d, want 0", s3Client.deletes)
	}
	if strings.Contains(fmt.Sprintf("%+v %#v", err, err), "secret") || strings.Contains(err.Error(), "queue") {
		t.Fatalf("error leaked provider details: %v / %+v", err, err)
	}
}

func TestSendRejectsInvalidRequestBeforeSDK(t *testing.T) {
	s3Client := &fakeS3Client{}
	sqsClient := &fakeSQSClient{}
	provider := mustProvider(t, sqsClient, s3Client)
	tests := []struct {
		name   string
		mutate func(*SendRequest)
		want   error
	}{
		{name: "blank queue", mutate: func(value *SendRequest) { value.QueueURL = " " }, want: ErrInvalidRequest},
		{name: "blank bucket", mutate: func(value *SendRequest) { value.Bucket = " " }, want: ErrInvalidRequest},
		{name: "blank key", mutate: func(value *SendRequest) { value.Key = "" }, want: ErrInvalidRequest},
		{name: "oversized payload", mutate: func(value *SendRequest) { value.Payload = bytes.Repeat([]byte("x"), 1<<20+1) }, want: ErrPayloadTooLarge},
		{name: "invalid content type", mutate: func(value *SendRequest) { value.ContentType = string([]byte{0xff}) }, want: ErrInvalidRequest},
		{name: "invalid metadata", mutate: func(value *SendRequest) { value.EncryptionMetadata = map[string]string{"": "bad"} }, want: ErrInvalidRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := validSendRequest()
			tt.mutate(&request)
			if _, err := provider.Send(context.Background(), request); !errors.Is(err, tt.want) {
				t.Fatalf("Send error = %v, want %v", err, tt.want)
			}
			if s3Client.puts != 0 || sqsClient.puts != 0 {
				t.Fatalf("calls = S3/%d SQS/%d, want 0/0", s3Client.puts, sqsClient.puts)
			}
		})
	}
}

func TestSendCancellationWinsAtBoundaries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s3Client := &fakeS3Client{putOutput: &s3.PutObjectOutput{}, cancelAfter: cancel}
	sqsClient := &fakeSQSClient{sendOutput: &sqs.SendMessageOutput{MessageId: aws.String("message")}}
	provider := mustProvider(t, sqsClient, s3Client)
	result, err := provider.Send(ctx, validSendRequest())
	var operationError *Error
	if result != nil || !errors.Is(err, context.Canceled) || !errors.Is(err, ErrCanceled) || !errors.As(err, &operationError) || !operationError.OrphanedObject() {
		t.Fatalf("Send cancellation after S3 = %v, want context.Canceled", err)
	}
	if sqsClient.puts != 0 {
		t.Fatalf("SQS calls after canceled S3 response = %d, want 0", sqsClient.puts)
	}

	ctx, cancel = context.WithCancel(context.Background())
	s3Client = &fakeS3Client{putOutput: &s3.PutObjectOutput{}}
	sqsClient = &fakeSQSClient{sendOutput: &sqs.SendMessageOutput{MessageId: aws.String("message")}, cancelAfter: cancel}
	provider = mustProvider(t, sqsClient, s3Client)
	result, err = provider.Send(ctx, validSendRequest())
	operationError = nil
	if result != nil || !errors.Is(err, context.Canceled) || !errors.Is(err, ErrCanceled) || !errors.As(err, &operationError) || !operationError.OrphanedObject() {
		t.Fatalf("Send cancellation after SQS = %v, want context.Canceled", err)
	}
}

func TestSendCancellationPreservesOrphanStateWithoutSDKOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s3Client := &fakeS3Client{cancelAfter: cancel}
	sqsClient := &fakeSQSClient{sendOutput: &sqs.SendMessageOutput{MessageId: aws.String("message")}}
	provider := mustProvider(t, sqsClient, s3Client)
	result, err := provider.Send(ctx, validSendRequest())
	var operationError *Error
	if result != nil || !errors.Is(err, context.Canceled) || !errors.Is(err, ErrCanceled) || !errors.As(err, &operationError) || !operationError.OrphanedObject() {
		t.Fatalf("Send cancellation = %v, want redacted orphan state", err)
	}
}

func TestReceiveClosesObjectBodyWhenCanceledAfterGet(t *testing.T) {
	payload := []byte("payload")
	encoded, err := EncodeEnvelope(envelopeFor("bucket", "key", payload))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	closed := 0
	body := &countingReadCloser{Reader: bytes.NewReader(payload), onClose: func() { closed++ }}
	sqsClient := &fakeSQSClient{receiveOut: &sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{{MessageId: aws.String("message"), ReceiptHandle: aws.String("receipt"), Body: aws.String(string(encoded))}}}}
	s3Client := &fakeS3Client{getBody: body, cancelAfter: cancel}
	provider := mustProvider(t, sqsClient, s3Client)
	if messages, err := provider.Receive(ctx, validReceiveRequest()); messages != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("Receive result = %#v/%v, want canceled", messages, err)
	}
	if closed != 1 {
		t.Fatalf("GetObject body close count = %d, want 1", closed)
	}
}

func TestReceiveReadsAndVerifiesPayloadWithoutAck(t *testing.T) {
	payload := []byte("hello world")
	envelope := envelopeFor("payloads", "orders/42/payload", payload)
	encoded, err := EncodeEnvelope(envelope)
	if err != nil {
		t.Fatalf("EncodeEnvelope: %v", err)
	}
	order := make([]string, 0, 2)
	sqsClient := &fakeSQSClient{
		receiveOut: &sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{{MessageId: aws.String("message-1"), ReceiptHandle: aws.String("receipt-1"), Body: aws.String(string(encoded))}}},
		order:      &order,
	}
	s3Client := &fakeS3Client{getOutput: &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(payload))}, order: &order}
	provider := mustProvider(t, sqsClient, s3Client)
	messages, err := provider.Receive(context.Background(), ReceiveRequest{QueueURL: "https://sqs.example/queue", MaxNumberOfMessages: 2, VisibilityTimeout: 90, WaitTimeSeconds: 10})
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if len(messages) != 1 || string(messages[0].Payload) != string(payload) || messages[0].ReceiptHandle != "receipt-1" || messages[0].MessageID != "message-1" {
		t.Fatalf("messages = %#v, want one verified message", messages)
	}
	if got := fmt.Sprint(order); got != "[sqs.receive s3.get]" {
		t.Fatalf("call order = %s, want [sqs.receive s3.get]", got)
	}
	if sqsClient.lastReceive == nil || sqsClient.lastReceive.MaxNumberOfMessages != 2 || sqsClient.lastReceive.VisibilityTimeout != 90 || sqsClient.lastReceive.WaitTimeSeconds != 10 {
		t.Fatalf("receive input = %#v", sqsClient.lastReceive)
	}
	if sqsClient.deletes != 0 {
		t.Fatalf("Receive acked messages = %d, want 0", sqsClient.deletes)
	}
}

func TestReceiveFailureDoesNotAckAndBoundsObjectRead(t *testing.T) {
	payload := []byte("hello world")
	envelope := envelopeFor("payloads", "orders/42/payload", payload)
	encoded, err := EncodeEnvelope(envelope)
	if err != nil {
		t.Fatalf("EncodeEnvelope: %v", err)
	}
	objectCause := errors.New("missing object secret key")
	sqsClient := &fakeSQSClient{receiveOut: &sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{{MessageId: aws.String("message"), ReceiptHandle: aws.String("receipt"), Body: aws.String(string(encoded))}}}}
	s3Client := &fakeS3Client{getErr: objectCause}
	provider := mustProvider(t, sqsClient, s3Client)
	if messages, err := provider.Receive(context.Background(), validReceiveRequest()); messages != nil || !errors.Is(err, ErrObjectReadFailed) || !errors.Is(err, objectCause) {
		t.Fatalf("missing object result = %#v/%v, want nil object-read error", messages, err)
	}
	if sqsClient.deletes != 0 {
		t.Fatalf("Receive acked missing object = %d, want 0", sqsClient.deletes)
	}

	s3Client = &fakeS3Client{getOutput: &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(append(payload, 'x')))}}
	provider = mustProvider(t, sqsClient, s3Client)
	if messages, err := provider.Receive(context.Background(), validReceiveRequest()); messages != nil || !errors.Is(err, ErrPayloadSizeMismatch) {
		t.Fatalf("extra payload result = %#v/%v, want size mismatch", messages, err)
	}
}

func TestReceiveRejectsChecksumAndMalformedEnvelope(t *testing.T) {
	payload := []byte("hello world")
	envelope := envelopeFor("payloads", "key", payload)
	envelope.Checksum = strings.Repeat("f", 64)
	encoded, err := EncodeEnvelope(envelope)
	if err != nil {
		t.Fatalf("EncodeEnvelope: %v", err)
	}
	sqsClient := &fakeSQSClient{receiveOut: &sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{{MessageId: aws.String("message"), ReceiptHandle: aws.String("receipt"), Body: aws.String(string(encoded))}}}}
	s3Client := &fakeS3Client{getOutput: &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(payload))}}
	provider := mustProvider(t, sqsClient, s3Client)
	if messages, err := provider.Receive(context.Background(), validReceiveRequest()); messages != nil || !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("checksum result = %#v/%v, want checksum mismatch", messages, err)
	}
	if s3Client.gets != 1 || sqsClient.deletes != 0 {
		t.Fatalf("calls = S3 get/%d SQS delete/%d", s3Client.gets, sqsClient.deletes)
	}

	sqsClient = &fakeSQSClient{receiveOut: &sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{{MessageId: aws.String("message"), ReceiptHandle: aws.String("receipt"), Body: aws.String("not-json")}}}}
	provider = mustProvider(t, sqsClient, &fakeS3Client{})
	if messages, err := provider.Receive(context.Background(), validReceiveRequest()); messages != nil || !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("malformed envelope result = %#v/%v, want invalid envelope", messages, err)
	}
}

func TestReceiveRejectsMalformedMessageIdentityAndBatchSize(t *testing.T) {
	validEnvelope, err := EncodeEnvelope(envelopeFor("bucket", "key", []byte("payload")))
	if err != nil {
		t.Fatalf("EncodeEnvelope: %v", err)
	}
	base := sqstypes.Message{MessageId: aws.String("message"), ReceiptHandle: aws.String("receipt"), Body: aws.String(string(validEnvelope))}
	for _, test := range []struct {
		name    string
		message sqstypes.Message
	}{
		{name: "missing message id", message: func() sqstypes.Message { value := base; value.MessageId = nil; return value }()},
		{name: "oversized body", message: func() sqstypes.Message {
			value := base
			value.Body = aws.String(strings.Repeat("x", MaxEnvelopeSize+1))
			return value
		}()},
	} {
		t.Run(test.name, func(t *testing.T) {
			sqsClient := &fakeSQSClient{receiveOut: &sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{test.message}}}
			provider := mustProvider(t, sqsClient, &fakeS3Client{})
			if messages, err := provider.Receive(context.Background(), validReceiveRequest()); messages != nil || !errors.Is(err, ErrMalformedOutput) {
				t.Fatalf("Receive result = %#v/%v, want malformed output", messages, err)
			}
		})
	}

	messages := make([]sqstypes.Message, 11)
	for index := range messages {
		messages[index] = base
	}
	provider := mustProvider(t, &fakeSQSClient{receiveOut: &sqs.ReceiveMessageOutput{Messages: messages}}, &fakeS3Client{})
	if received, err := provider.Receive(context.Background(), validReceiveRequest()); received != nil || !errors.Is(err, ErrMalformedOutput) {
		t.Fatalf("oversized batch result = %#v/%v, want malformed output", received, err)
	}
}

func TestReceiveRejectsAggregatePayloadBeforeObjectDispatch(t *testing.T) {
	firstPayload := bytes.Repeat([]byte("a"), 6)
	secondPayload := bytes.Repeat([]byte("b"), 6)
	first, err := EncodeEnvelope(envelopeFor("bucket", "first", firstPayload))
	if err != nil {
		t.Fatal(err)
	}
	second, err := EncodeEnvelope(envelopeFor("bucket", "second", secondPayload))
	if err != nil {
		t.Fatal(err)
	}
	message := func(body []byte, id string) sqstypes.Message {
		return sqstypes.Message{
			MessageId:     aws.String(id),
			ReceiptHandle: aws.String("receipt-" + id),
			Body:          aws.String(string(body)),
		}
	}
	sqsClient := &fakeSQSClient{receiveOut: &sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{
		message(first, "first"),
		message(second, "second"),
	}}}
	s3Client := &fakeS3Client{getOutput: &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(firstPayload))}}
	provider, err := New(Options{
		SQSClient:             sqsClient,
		S3Client:              s3Client,
		MaxPayloadSize:        6,
		MaxReceivePayloadSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if messages, err := provider.Receive(context.Background(), ReceiveRequest{QueueURL: "queue", MaxNumberOfMessages: 2}); messages != nil || !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("Receive result = %#v/%v, want aggregate payload error", messages, err)
	}
	if s3Client.gets != 0 {
		t.Fatalf("S3 dispatches = %d, want 0 before aggregate preflight", s3Client.gets)
	}
}

func TestDeleteUsesSQSFirstAndReportsCleanupState(t *testing.T) {
	payload := []byte("hello world")
	envelope := envelopeFor("payloads", "orders/42/payload", payload)
	order := make([]string, 0, 2)
	sqsClient := &fakeSQSClient{deleteOut: &sqs.DeleteMessageOutput{}, order: &order}
	s3Client := &fakeS3Client{deleteOut: &s3.DeleteObjectOutput{}, order: &order}
	provider := mustProvider(t, sqsClient, s3Client)
	if err := provider.Delete(context.Background(), DeleteRequest{QueueURL: "queue", ReceiptHandle: "receipt", Envelope: envelope}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := fmt.Sprint(order); got != "[sqs.delete s3.delete]" {
		t.Fatalf("call order = %s, want [sqs.delete s3.delete]", got)
	}
	if sqsClient.lastDelete == nil || aws.ToString(sqsClient.lastDelete.QueueUrl) != "queue" || aws.ToString(sqsClient.lastDelete.ReceiptHandle) != "receipt" {
		t.Fatalf("SQS delete input = %#v", sqsClient.lastDelete)
	}
	if s3Client.lastDelete == nil || aws.ToString(s3Client.lastDelete.Bucket) != envelope.Bucket || aws.ToString(s3Client.lastDelete.Key) != envelope.Key {
		t.Fatalf("S3 delete input = %#v", s3Client.lastDelete)
	}

	sqsClient = &fakeSQSClient{deleteErr: errors.New("queue private failure")}
	s3Client = &fakeS3Client{}
	provider = mustProvider(t, sqsClient, s3Client)
	if err := provider.Delete(context.Background(), DeleteRequest{QueueURL: "queue", ReceiptHandle: "receipt", Envelope: envelope}); !errors.Is(err, ErrMessageDeleteFailed) {
		t.Fatalf("SQS delete failure = %v, want message delete", err)
	}
	if s3Client.deletes != 0 {
		t.Fatalf("S3 deletes after SQS failure = %d, want 0", s3Client.deletes)
	}

	sqsClient = &fakeSQSClient{deleteOut: &sqs.DeleteMessageOutput{}}
	s3Client = &fakeS3Client{deleteErr: errors.New("object private failure")}
	provider = mustProvider(t, sqsClient, s3Client)
	err := provider.Delete(context.Background(), DeleteRequest{QueueURL: "queue", ReceiptHandle: "receipt", Envelope: envelope})
	if !errors.Is(err, ErrObjectDeleteFailed) {
		t.Fatalf("S3 delete failure = %v, want object delete", err)
	}
	var operationError *Error
	if !errors.As(err, &operationError) || !operationError.QueueDeleted() {
		t.Fatalf("S3 delete failure = %#v, want QueueDeleted", err)
	}
}

func TestDeleteCancellationBetweenQueueAndObject(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sqsClient := &fakeSQSClient{deleteOut: &sqs.DeleteMessageOutput{}, cancelAfter: cancel}
	s3Client := &fakeS3Client{}
	provider := mustProvider(t, sqsClient, s3Client)
	err := provider.Delete(ctx, DeleteRequest{QueueURL: "queue", ReceiptHandle: "receipt", Envelope: envelopeFor("bucket", "key", []byte("payload"))})
	var operationError *Error
	if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrCanceled) || !errors.As(err, &operationError) || !operationError.QueueDeleted() {
		t.Fatalf("Delete cancellation = %v, want context.Canceled", err)
	}
	if s3Client.deletes != 0 {
		t.Fatalf("S3 deletes after cancellation = %d, want 0", s3Client.deletes)
	}
}

func TestDeleteCancellationPreservesQueueStateWithoutSDKOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sqsClient := &fakeSQSClient{cancelAfter: cancel}
	s3Client := &fakeS3Client{}
	provider := mustProvider(t, sqsClient, s3Client)
	err := provider.Delete(ctx, DeleteRequest{QueueURL: "queue", ReceiptHandle: "receipt", Envelope: envelopeFor("bucket", "key", []byte("payload"))})
	var operationError *Error
	if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrCanceled) || !errors.As(err, &operationError) || !operationError.QueueDeleted() {
		t.Fatalf("Delete cancellation = %v, want redacted queue state", err)
	}
	if s3Client.deletes != 0 {
		t.Fatalf("S3 deletes after cancellation = %d, want 0", s3Client.deletes)
	}
}

func TestDeleteObjectSupportsExplicitOrphanCleanup(t *testing.T) {
	s3Client := &fakeS3Client{deleteOut: &s3.DeleteObjectOutput{}}
	provider := mustProvider(t, &fakeSQSClient{}, s3Client)
	if err := provider.DeleteObject(context.Background(), envelopeFor("bucket", "key", []byte("payload"))); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	if s3Client.deletes != 1 {
		t.Fatalf("S3 deletes = %d, want 1", s3Client.deletes)
	}
}

func TestProviderConcurrentSendUsesIndependentInputs(t *testing.T) {
	s3Client := &fakeS3Client{putOutput: &s3.PutObjectOutput{}}
	sqsClient := &fakeSQSClient{sendOutput: &sqs.SendMessageOutput{MessageId: aws.String("message")}}
	provider := mustProvider(t, sqsClient, s3Client)
	var group sync.WaitGroup
	for i := 0; i < 16; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			request := validSendRequest()
			request.Key = fmt.Sprintf("key/%d", index)
			if _, err := provider.Send(context.Background(), request); err != nil {
				t.Errorf("Send(%d): %v", index, err)
			}
		}(i)
	}
	group.Wait()
	if sqsClient.puts != 16 || s3Client.puts != 16 {
		t.Fatalf("calls = S3/%d SQS/%d, want 16/16", s3Client.puts, sqsClient.puts)
	}
}

func validSendRequest() SendRequest {
	return SendRequest{QueueURL: "queue", Bucket: "bucket", Key: "key", Payload: []byte("payload")}
}

func validReceiveRequest() ReceiveRequest {
	return ReceiveRequest{QueueURL: "queue", MaxNumberOfMessages: 1}
}

func envelopeFor(bucket, key string, payload []byte) Envelope {
	return Envelope{Version: EnvelopeVersion, Bucket: bucket, Key: key, ContentSize: int64(len(payload)), Checksum: checksum(payload)}
}

func checksum(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func mustProvider(t *testing.T, sqsClient SQSClient, s3Client S3Client) *Provider {
	t.Helper()
	provider, err := New(Options{SQSClient: sqsClient, S3Client: s3Client, MaxPayloadSize: 1 << 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return provider
}

func cloneSendInput(input *sqs.SendMessageInput) *sqs.SendMessageInput {
	if input == nil {
		return nil
	}
	clone := *input
	clone.QueueUrl = cloneString(input.QueueUrl)
	clone.MessageBody = cloneString(input.MessageBody)
	clone.MessageAttributes = cloneAttributes(input.MessageAttributes)
	return &clone
}

func cloneReceiveInput(input *sqs.ReceiveMessageInput) *sqs.ReceiveMessageInput {
	if input == nil {
		return nil
	}
	clone := *input
	clone.QueueUrl = cloneString(input.QueueUrl)
	clone.MessageAttributeNames = append([]string(nil), input.MessageAttributeNames...)
	clone.MessageSystemAttributeNames = append([]sqstypes.MessageSystemAttributeName(nil), input.MessageSystemAttributeNames...)
	return &clone
}

func cloneDeleteInput(input *sqs.DeleteMessageInput) *sqs.DeleteMessageInput {
	if input == nil {
		return nil
	}
	clone := *input
	clone.QueueUrl = cloneString(input.QueueUrl)
	clone.ReceiptHandle = cloneString(input.ReceiptHandle)
	return &clone
}

func clonePutInput(input *s3.PutObjectInput) *s3.PutObjectInput {
	if input == nil {
		return nil
	}
	clone := *input
	clone.Bucket = cloneString(input.Bucket)
	clone.Key = cloneString(input.Key)
	clone.ContentLength = cloneInt64(input.ContentLength)
	clone.ContentType = cloneString(input.ContentType)
	clone.ChecksumSHA256 = cloneString(input.ChecksumSHA256)
	return &clone
}

func cloneGetInput(input *s3.GetObjectInput) *s3.GetObjectInput {
	if input == nil {
		return nil
	}
	clone := *input
	clone.Bucket = cloneString(input.Bucket)
	clone.Key = cloneString(input.Key)
	return &clone
}

func cloneDeleteObjectInput(input *s3.DeleteObjectInput) *s3.DeleteObjectInput {
	if input == nil {
		return nil
	}
	clone := *input
	clone.Bucket = cloneString(input.Bucket)
	clone.Key = cloneString(input.Key)
	return &clone
}

func cloneReceiveOutput(output *sqs.ReceiveMessageOutput) *sqs.ReceiveMessageOutput {
	if output == nil {
		return nil
	}
	clone := &sqs.ReceiveMessageOutput{Messages: make([]sqstypes.Message, len(output.Messages))}
	for i, message := range output.Messages {
		clone.Messages[i] = message
		clone.Messages[i].Body = cloneString(message.Body)
		clone.Messages[i].MessageId = cloneString(message.MessageId)
		clone.Messages[i].ReceiptHandle = cloneString(message.ReceiptHandle)
	}
	return clone
}

func cloneSendOutput(output *sqs.SendMessageOutput) *sqs.SendMessageOutput {
	if output == nil {
		return nil
	}
	clone := *output
	clone.MessageId = cloneString(output.MessageId)
	return &clone
}

func cloneDeleteOutput(output *sqs.DeleteMessageOutput) *sqs.DeleteMessageOutput {
	if output == nil {
		return nil
	}
	clone := *output
	return &clone
}

func cloneGetOutput(output *s3.GetObjectOutput) *s3.GetObjectOutput {
	if output == nil {
		return nil
	}
	clone := *output
	if output.Body != nil {
		body, err := io.ReadAll(output.Body)
		if err != nil {
			return nil
		}
		_ = output.Body.Close()
		clone.Body = io.NopCloser(bytes.NewReader(body))
	}
	return &clone
}

func cloneAttributes(attributes map[string]sqstypes.MessageAttributeValue) map[string]sqstypes.MessageAttributeValue {
	if attributes == nil {
		return nil
	}
	clone := make(map[string]sqstypes.MessageAttributeValue, len(attributes))
	for key, value := range attributes {
		clone[key] = value
	}
	return clone
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
