package sqssnsexample_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	flocitestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/floci"
)

type orderMessage struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func Example_sendReceiveAndManualAck() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg := aws.Config{} // Load with config.LoadDefaultConfig in application code.
	client := sqs.NewFromConfig(cfg)

	queueURL, err := createQueue(ctx, client, "orders", nil)
	if err != nil {
		return
	}
	payload, err := encodeJSONMessage(orderMessage{ID: "order-1", Status: "created"})
	if err != nil {
		return
	}

	if _, err := client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(payload),
	}); err != nil {
		return
	}

	messages, err := receiveMessages(ctx, client, queueURL, 1, 10, 30)
	if err != nil {
		return
	}
	for _, message := range messages {
		if _, err := decodeJSONMessage[orderMessage](message); err != nil {
			return
		}
		if err := deleteMessage(ctx, client, queueURL, aws.ToString(message.ReceiptHandle)); err != nil {
			return
		}
	}
}

func Example_visibilityTimeoutAndDeadLetterNotes() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg := aws.Config{}
	client := sqs.NewFromConfig(cfg)

	dlqURL, err := createQueue(ctx, client, "orders-dlq", nil)
	if err != nil {
		return
	}
	dlqARN, err := queueARN(ctx, client, dlqURL)
	if err != nil {
		return
	}
	policy, err := redrivePolicy(dlqARN, 5)
	if err != nil {
		return
	}

	queueURL, err := createQueue(ctx, client, "orders", map[string]string{
		string(sqstypes.QueueAttributeNameRedrivePolicy):     policy,
		string(sqstypes.QueueAttributeNameVisibilityTimeout): "30",
	})
	if err != nil {
		return
	}

	messages, err := receiveMessages(ctx, client, queueURL, 1, 10, 30)
	if err != nil {
		return
	}
	if len(messages) == 0 {
		return
	}

	receiptHandle := aws.ToString(messages[0].ReceiptHandle)
	if err := changeMessageVisibility(ctx, client, queueURL, receiptHandle, 60); err != nil {
		return
	}
	if err := deleteMessage(ctx, client, queueURL, receiptHandle); err != nil {
		return
	}
}

func Example_snsFanoutToSQS() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg := aws.Config{}
	snsClient := sns.NewFromConfig(cfg)
	sqsClient := sqs.NewFromConfig(cfg)

	topic, err := snsClient.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String("orders"),
	})
	if err != nil {
		return
	}
	queueURL, err := createQueue(ctx, sqsClient, "orders-fanout", nil)
	if err != nil {
		return
	}
	queueARN, err := queueARN(ctx, sqsClient, queueURL)
	if err != nil {
		return
	}

	if _, err := snsClient.Subscribe(ctx, &sns.SubscribeInput{
		TopicArn:              topic.TopicArn,
		Protocol:              aws.String("sqs"),
		Endpoint:              aws.String(queueARN),
		ReturnSubscriptionArn: true,
	}); err != nil {
		return
	}

	if _, err := snsClient.Publish(ctx, &sns.PublishInput{
		TopicArn: topic.TopicArn,
		Message:  aws.String(`{"id":"order-1","status":"created"}`),
	}); err != nil {
		return
	}
}

func TestSQSSNSExampleSmoke(t *testing.T) {
	if os.Getenv("BLUETAPE_SQS_SNS_EXAMPLE_SMOKE") != "1" {
		t.Skip("set BLUETAPE_SQS_SNS_EXAMPLE_SMOKE=1 to run the Docker-backed Floci SQS/SNS example smoke test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	details := flocitestcontainer.Start(ctx, t,
		flocitestcontainer.WithSQSConfig(flocitestcontainer.DefaultSQSConfig()),
		flocitestcontainer.WithSNSConfig(flocitestcontainer.DefaultSNSConfig()),
	)
	cfg := flocitestcontainer.LoadConfig(ctx, t, details)
	sqsClient := sqs.NewFromConfig(cfg)
	snsClient := sns.NewFromConfig(cfg)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	queueURL, err := createQueue(ctx, sqsClient, "bluetape-sqs-sns-"+suffix, map[string]string{
		string(sqstypes.QueueAttributeNameVisibilityTimeout): "2",
	})
	if err != nil {
		t.Fatalf("create queue: %v", err)
	}

	payload, err := encodeJSONMessage(orderMessage{ID: "order-1", Status: "created"})
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}
	if _, err := sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(payload),
		MessageAttributes: map[string]sqstypes.MessageAttributeValue{
			"content-type": {
				DataType:    aws.String("String"),
				StringValue: aws.String("application/json"),
			},
		},
	}); err != nil {
		t.Fatalf("send message: %v", err)
	}

	messages, err := receiveMessages(ctx, sqsClient, queueURL, 1, 3, 2)
	if err != nil {
		t.Fatalf("receive message: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("received %d messages, want 1", len(messages))
	}
	got, err := decodeJSONMessage[orderMessage](messages[0])
	if err != nil {
		t.Fatalf("decode message: %v", err)
	}
	if got.ID != "order-1" || got.Status != "created" {
		t.Fatalf("message = %+v, want order-1 created", got)
	}

	receiptHandle := aws.ToString(messages[0].ReceiptHandle)
	if err := changeMessageVisibility(ctx, sqsClient, queueURL, receiptHandle, 3); err != nil {
		t.Fatalf("change visibility: %v", err)
	}
	if err := deleteMessage(ctx, sqsClient, queueURL, receiptHandle); err != nil {
		t.Fatalf("delete message: %v", err)
	}
	empty, err := receiveMessages(ctx, sqsClient, queueURL, 1, 1, 1)
	if err != nil {
		t.Fatalf("receive after delete: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("received %d messages after delete, want 0", len(empty))
	}

	topic, err := snsClient.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String("bluetape-sqs-sns-topic-" + suffix),
	})
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	fanoutQueueURL, err := createQueue(ctx, sqsClient, "bluetape-sqs-sns-fanout-"+suffix, nil)
	if err != nil {
		t.Fatalf("create fanout queue: %v", err)
	}
	fanoutQueueARN, err := queueARN(ctx, sqsClient, fanoutQueueURL)
	if err != nil {
		t.Fatalf("get fanout queue arn: %v", err)
	}
	if _, err := snsClient.Subscribe(ctx, &sns.SubscribeInput{
		TopicArn:              topic.TopicArn,
		Protocol:              aws.String("sqs"),
		Endpoint:              aws.String(fanoutQueueARN),
		ReturnSubscriptionArn: true,
	}); err != nil {
		t.Fatalf("subscribe fanout queue: %v", err)
	}

	fanoutBody, err := encodeJSONMessage(orderMessage{ID: "order-2", Status: "published"})
	if err != nil {
		t.Fatalf("encode fanout message: %v", err)
	}
	if _, err := snsClient.Publish(ctx, &sns.PublishInput{
		TopicArn: topic.TopicArn,
		Message:  aws.String(fanoutBody),
	}); err != nil {
		t.Fatalf("publish fanout message: %v", err)
	}

	fanoutMessages, err := receiveUntil(ctx, sqsClient, fanoutQueueURL, 1, 2, 0)
	if err != nil {
		t.Fatalf("receive fanout message: %v", err)
	}
	if len(fanoutMessages) != 1 {
		t.Fatalf("received %d fanout messages, want 1", len(fanoutMessages))
	}
	if got := snsPayload(aws.ToString(fanoutMessages[0].Body)); got != fanoutBody {
		t.Fatalf("fanout payload = %q, want %q", got, fanoutBody)
	}
	if err := deleteMessage(ctx, sqsClient, fanoutQueueURL, aws.ToString(fanoutMessages[0].ReceiptHandle)); err != nil {
		t.Fatalf("delete fanout message: %v", err)
	}
}

func TestJSONMessageCodec(t *testing.T) {
	body, err := encodeJSONMessage(orderMessage{ID: "order-1", Status: "created"})
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}
	got, err := decodeJSONMessage[orderMessage](sqstypes.Message{Body: aws.String(body)})
	if err != nil {
		t.Fatalf("decode message: %v", err)
	}
	if got.ID != "order-1" || got.Status != "created" {
		t.Fatalf("message = %+v, want order-1 created", got)
	}
}

func TestRedrivePolicy(t *testing.T) {
	got, err := redrivePolicy("arn:aws:sqs:ap-northeast-2:000000000000:orders-dlq", 5)
	if err != nil {
		t.Fatalf("redrive policy: %v", err)
	}

	var policy struct {
		DeadLetterTargetARN string `json:"deadLetterTargetArn"`
		MaxReceiveCount     string `json:"maxReceiveCount"`
	}
	if err := json.Unmarshal([]byte(got), &policy); err != nil {
		t.Fatalf("decode redrive policy: %v", err)
	}
	if policy.DeadLetterTargetARN != "arn:aws:sqs:ap-northeast-2:000000000000:orders-dlq" {
		t.Fatalf("dead letter arn = %q", policy.DeadLetterTargetARN)
	}
	if policy.MaxReceiveCount != "5" {
		t.Fatalf("max receive count = %q, want 5", policy.MaxReceiveCount)
	}
}

func TestSNSPayload(t *testing.T) {
	raw := `{"id":"order-1","status":"created"}`
	enveloped := `{"Type":"Notification","Message":"{\"id\":\"order-1\",\"status\":\"created\"}"}`
	if got := snsPayload(raw); got != raw {
		t.Fatalf("raw payload = %q, want %q", got, raw)
	}
	if got := snsPayload(enveloped); got != raw {
		t.Fatalf("enveloped payload = %q, want %q", got, raw)
	}
}

func createQueue(ctx context.Context, client *sqs.Client, name string, attributes map[string]string) (string, error) {
	out, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName:  aws.String(name),
		Attributes: attributes,
	})
	if err != nil {
		return "", fmt.Errorf("create queue %s: %w", name, err)
	}
	return aws.ToString(out.QueueUrl), nil
}

func encodeJSONMessage(v any) (string, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal message: %w", err)
	}
	return string(payload), nil
}

func decodeJSONMessage[T any](message sqstypes.Message) (T, error) {
	var value T
	body := aws.ToString(message.Body)
	if body == "" {
		return value, errors.New("message body is empty")
	}
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		return value, fmt.Errorf("decode message body: %w", err)
	}
	return value, nil
}

func receiveMessages(
	ctx context.Context,
	client *sqs.Client,
	queueURL string,
	maxMessages int32,
	waitTimeSeconds int32,
	visibilityTimeoutSeconds int32,
) ([]sqstypes.Message, error) {
	out, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(queueURL),
		MaxNumberOfMessages: maxMessages,
		WaitTimeSeconds:     waitTimeSeconds,
		VisibilityTimeout:   visibilityTimeoutSeconds,
		MessageAttributeNames: []string{
			"All",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("receive messages: %w", err)
	}
	return out.Messages, nil
}

func receiveUntil(
	ctx context.Context,
	client *sqs.Client,
	queueURL string,
	maxMessages int32,
	waitTimeSeconds int32,
	visibilityTimeoutSeconds int32,
) ([]sqstypes.Message, error) {
	for {
		messages, err := receiveMessages(ctx, client, queueURL, maxMessages, waitTimeSeconds, visibilityTimeoutSeconds)
		if err != nil {
			return nil, err
		}
		if len(messages) > 0 {
			return messages, nil
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
}

func changeMessageVisibility(ctx context.Context, client *sqs.Client, queueURL, receiptHandle string, timeoutSeconds int32) error {
	_, err := client.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(queueURL),
		ReceiptHandle:     aws.String(receiptHandle),
		VisibilityTimeout: timeoutSeconds,
	})
	if err != nil {
		return fmt.Errorf("change message visibility: %w", err)
	}
	return nil
}

func deleteMessage(ctx context.Context, client *sqs.Client, queueURL, receiptHandle string) error {
	_, err := client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	if err != nil {
		return fmt.Errorf("delete message: %w", err)
	}
	return nil
}

func queueARN(ctx context.Context, client *sqs.Client, queueURL string) (string, error) {
	out, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(queueURL),
		AttributeNames: []sqstypes.QueueAttributeName{
			sqstypes.QueueAttributeNameQueueArn,
		},
	})
	if err != nil {
		return "", fmt.Errorf("get queue attributes: %w", err)
	}
	arn := out.Attributes[string(sqstypes.QueueAttributeNameQueueArn)]
	if arn == "" {
		return "", errors.New("queue ARN attribute is empty")
	}
	return arn, nil
}

func redrivePolicy(deadLetterQueueARN string, maxReceiveCount int) (string, error) {
	if deadLetterQueueARN == "" {
		return "", errors.New("dead letter queue ARN is empty")
	}
	if maxReceiveCount <= 0 {
		return "", errors.New("max receive count must be positive")
	}
	policy := struct {
		DeadLetterTargetARN string `json:"deadLetterTargetArn"`
		MaxReceiveCount     string `json:"maxReceiveCount"`
	}{
		DeadLetterTargetARN: deadLetterQueueARN,
		MaxReceiveCount:     fmt.Sprintf("%d", maxReceiveCount),
	}
	payload, err := json.Marshal(policy)
	if err != nil {
		return "", fmt.Errorf("marshal redrive policy: %w", err)
	}
	return string(payload), nil
}

func snsPayload(body string) string {
	var notification struct {
		Message string `json:"Message"`
	}
	if err := json.Unmarshal([]byte(body), &notification); err == nil && notification.Message != "" {
		return notification.Message
	}
	return body
}
