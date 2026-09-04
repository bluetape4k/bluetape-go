package sqsextended

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func ExampleNew() {
	provider, err := New(Options{
		SQSClient: &exampleSQSClient{},
		S3Client:  &exampleS3Client{},
	})
	if err != nil {
		panic(err)
	}
	result, err := provider.Send(context.Background(), SendRequest{
		QueueURL:    "https://sqs.example/queue",
		Bucket:      "payloads",
		Key:         "orders/42/payload.json",
		Payload:     []byte(`{"order_id":42}`),
		ContentType: "application/json",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(result.MessageID)
	// Output: message-1
}

type exampleSQSClient struct{}

func (*exampleSQSClient) SendMessage(context.Context, *sqs.SendMessageInput, ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	return &sqs.SendMessageOutput{MessageId: aws.String("message-1")}, nil
}

func (*exampleSQSClient) ReceiveMessage(context.Context, *sqs.ReceiveMessageInput, ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	return &sqs.ReceiveMessageOutput{}, nil
}

func (*exampleSQSClient) DeleteMessage(context.Context, *sqs.DeleteMessageInput, ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	return &sqs.DeleteMessageOutput{}, nil
}

type exampleS3Client struct{}

func (*exampleS3Client) PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	return &s3.PutObjectOutput{}, nil
}

func (*exampleS3Client) GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(""))}, nil
}

func (*exampleS3Client) DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return &s3.DeleteObjectOutput{}, nil
}
