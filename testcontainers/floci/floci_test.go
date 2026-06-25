package flocitestcontainer_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	flocitestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/floci"
)

func TestStartFlociAWSServicesSmoke(t *testing.T) {
	if os.Getenv("BLUETAPE_FLOCI_SMOKE") != "1" {
		t.Skip("set BLUETAPE_FLOCI_SMOKE=1 to run the Docker-backed Floci AWS service smoke test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	details := flocitestcontainer.Start(ctx, t,
		flocitestcontainer.WithS3Config(flocitestcontainer.DefaultS3Config()),
		flocitestcontainer.WithSQSConfig(flocitestcontainer.DefaultSQSConfig()),
		flocitestcontainer.WithSNSConfig(flocitestcontainer.DefaultSNSConfig()),
		flocitestcontainer.WithDynamoDBConfig(flocitestcontainer.DefaultDynamoDBConfig()),
	)
	cfg := flocitestcontainer.LoadConfig(ctx, t, details)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	verifyS3(ctx, t, cfg, suffix)
	verifySQS(ctx, t, cfg, suffix)
	verifySNSFanout(ctx, t, cfg, suffix)
	verifyDynamoDB(ctx, t, cfg, suffix)
}

func verifyS3(ctx context.Context, t *testing.T, cfg aws.Config, suffix string) {
	t.Helper()

	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.UsePathStyle = true
	})

	bucket := "bluetape-floci-" + suffix
	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	const key = "hello.txt"
	const body = "hello from bluetape floci"
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(body),
	}); err != nil {
		t.Fatalf("put object: %v", err)
	}

	got, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("get object: %v", err)
	}
	payload, err := io.ReadAll(got.Body)
	if err != nil {
		t.Fatalf("read object body: %v", err)
	}
	if err := got.Body.Close(); err != nil {
		t.Fatalf("close object body: %v", err)
	}
	if string(payload) != body {
		t.Fatalf("object body = %q, want %q", payload, body)
	}
}

func verifySQS(ctx context.Context, t *testing.T, cfg aws.Config, suffix string) {
	t.Helper()

	client := sqs.NewFromConfig(cfg)
	queueURL := createQueue(ctx, t, client, "bluetape-floci-sqs-"+suffix)

	const body = "hello from floci sqs"
	if _, err := client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    queueURL,
		MessageBody: aws.String(body),
	}); err != nil {
		t.Fatalf("send sqs message: %v", err)
	}

	message := receiveOneMessage(ctx, t, client, queueURL)
	if got := aws.ToString(message.Body); got != body {
		t.Fatalf("sqs message body = %q, want %q", got, body)
	}
	deleteMessage(ctx, t, client, queueURL, message.ReceiptHandle)
}

func verifySNSFanout(ctx context.Context, t *testing.T, cfg aws.Config, suffix string) {
	t.Helper()

	snsClient := sns.NewFromConfig(cfg)
	sqsClient := sqs.NewFromConfig(cfg)

	topicOut, err := snsClient.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String("bluetape-floci-topic-" + suffix),
	})
	if err != nil {
		t.Fatalf("create sns topic: %v", err)
	}
	queueURL := createQueue(ctx, t, sqsClient, "bluetape-floci-fanout-"+suffix)

	attrOut, err := sqsClient.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       queueURL,
		AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
	})
	if err != nil {
		t.Fatalf("get fanout queue attributes: %v", err)
	}
	queueARN := attrOut.Attributes[string(sqstypes.QueueAttributeNameQueueArn)]
	if queueARN == "" {
		t.Fatal("fanout queue ARN must not be empty")
	}

	if _, err := snsClient.Subscribe(ctx, &sns.SubscribeInput{
		TopicArn: topicOut.TopicArn,
		Protocol: aws.String("sqs"),
		Endpoint: aws.String(queueARN),
	}); err != nil {
		t.Fatalf("subscribe sqs queue to sns topic: %v", err)
	}

	const body = "hello from floci sns"
	if _, err := snsClient.Publish(ctx, &sns.PublishInput{
		TopicArn: topicOut.TopicArn,
		Message:  aws.String(body),
	}); err != nil {
		t.Fatalf("publish sns message: %v", err)
	}

	message := receiveOneMessage(ctx, t, sqsClient, queueURL)
	var notification struct {
		Message string `json:"Message"`
	}
	if err := json.Unmarshal([]byte(aws.ToString(message.Body)), &notification); err != nil {
		t.Fatalf("parse sns notification: %v", err)
	}
	if notification.Message != body {
		t.Fatalf("sns fanout message = %q, want %q", notification.Message, body)
	}
	deleteMessage(ctx, t, sqsClient, queueURL, message.ReceiptHandle)
}

func verifyDynamoDB(ctx context.Context, t *testing.T, cfg aws.Config, suffix string) {
	t.Helper()

	client := dynamodb.NewFromConfig(cfg)
	table := "bluetape_floci_" + suffix
	if _, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(table),
		AttributeDefinitions: []dynamodbtypes.AttributeDefinition{
			{AttributeName: aws.String("id"), AttributeType: dynamodbtypes.ScalarAttributeTypeS},
		},
		KeySchema: []dynamodbtypes.KeySchemaElement{
			{AttributeName: aws.String("id"), KeyType: dynamodbtypes.KeyTypeHash},
		},
		BillingMode: dynamodbtypes.BillingModePayPerRequest,
	}); err != nil {
		t.Fatalf("create dynamodb table: %v", err)
	}

	if _, err := client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item: map[string]dynamodbtypes.AttributeValue{
			"id":   &dynamodbtypes.AttributeValueMemberS{Value: "item-1"},
			"name": &dynamodbtypes.AttributeValueMemberS{Value: "floci"},
		},
	}); err != nil {
		t.Fatalf("put dynamodb item: %v", err)
	}

	got, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]dynamodbtypes.AttributeValue{
			"id": &dynamodbtypes.AttributeValueMemberS{Value: "item-1"},
		},
	})
	if err != nil {
		t.Fatalf("get dynamodb item: %v", err)
	}
	name, ok := got.Item["name"].(*dynamodbtypes.AttributeValueMemberS)
	if !ok {
		t.Fatalf("dynamodb name attribute type = %T, want string", got.Item["name"])
	}
	if name.Value != "floci" {
		t.Fatalf("dynamodb name = %q, want %q", name.Value, "floci")
	}
}

func createQueue(ctx context.Context, t *testing.T, client *sqs.Client, name string) *string {
	t.Helper()

	out, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String(name)})
	if err != nil {
		t.Fatalf("create sqs queue %s: %v", name, err)
	}
	return out.QueueUrl
}

func receiveOneMessage(ctx context.Context, t *testing.T, client *sqs.Client, queueURL *string) sqstypes.Message {
	t.Helper()

	out, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            queueURL,
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     3,
	})
	if err != nil {
		t.Fatalf("receive sqs message: %v", err)
	}
	if len(out.Messages) == 0 {
		t.Fatal("expected one sqs message")
	}
	return out.Messages[0]
}

func deleteMessage(ctx context.Context, t *testing.T, client *sqs.Client, queueURL *string, receiptHandle *string) {
	t.Helper()

	if _, err := client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      queueURL,
		ReceiptHandle: receiptHandle,
	}); err != nil {
		t.Fatalf("delete sqs message: %v", err)
	}
}

func TestDetailsConnectionDetails(t *testing.T) {
	details := flocitestcontainer.Details{
		Endpoint:             "http://localhost:4566",
		Region:               "ap-northeast-2",
		AccessKeyID:          "test",
		SecretAccessKey:      "test",
		AccountID:            "000000000000",
		AvailabilityZone:     "ap-northeast-2a",
		DedicatedNetworkName: "floci-network",
	}

	connectionDetails := details.ConnectionDetails()
	cases := map[string]string{
		flocitestcontainer.EndpointKey:             details.Endpoint,
		flocitestcontainer.RegionKey:               details.Region,
		flocitestcontainer.AccessKeyIDKey:          details.AccessKeyID,
		flocitestcontainer.SecretAccessKeyKey:      details.SecretAccessKey,
		flocitestcontainer.AccountIDKey:            details.AccountID,
		flocitestcontainer.AvailabilityZoneKey:     details.AvailabilityZone,
		flocitestcontainer.DedicatedNetworkNameKey: details.DedicatedNetworkName,
	}
	for key, want := range cases {
		got, err := connectionDetails.Require(key)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}
