package batchwrite

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestWriteAllChunksRequests(t *testing.T) {
	client := &fakeClient{
		outputs: []*dynamodb.BatchWriteItemOutput{
			{},
			{},
		},
	}

	result, err := WriteAll(context.Background(), client, map[string][]types.WriteRequest{
		"orders": writeRequests(30),
	}, WithMaxAttempts(1))
	if err != nil {
		t.Fatalf("WriteAll error = %v", err)
	}
	if result.Attempts != 2 {
		t.Fatalf("Attempts = %d, want 2", result.Attempts)
	}
	if result.Processed != 30 {
		t.Fatalf("Processed = %d, want 30", result.Processed)
	}
	if len(client.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(client.calls))
	}
	if got := countRequestItems(client.calls[0]); got != MaxItemsPerBatch {
		t.Fatalf("first chunk size = %d, want %d", got, MaxItemsPerBatch)
	}
	if got := countRequestItems(client.calls[1]); got != 5 {
		t.Fatalf("second chunk size = %d, want 5", got)
	}
}

func TestWriteAllRetriesUnprocessedItems(t *testing.T) {
	unprocessed := map[string][]types.WriteRequest{
		"orders": writeRequests(2),
	}
	client := &fakeClient{
		outputs: []*dynamodb.BatchWriteItemOutput{
			{UnprocessedItems: unprocessed},
			{},
		},
	}

	result, err := WriteAll(context.Background(), client, map[string][]types.WriteRequest{
		"orders": writeRequests(3),
	})
	if err != nil {
		t.Fatalf("WriteAll error = %v", err)
	}
	if result.Attempts != 2 {
		t.Fatalf("Attempts = %d, want 2", result.Attempts)
	}
	if result.Processed != 3 {
		t.Fatalf("Processed = %d, want 3", result.Processed)
	}
	if len(client.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(client.calls))
	}
	if !reflect.DeepEqual(client.calls[1], unprocessed) {
		t.Fatalf("retry request = %#v, want %#v", client.calls[1], unprocessed)
	}
}

func TestWriteAllExhaustsUnprocessedItems(t *testing.T) {
	firstUnprocessed := map[string][]types.WriteRequest{
		"orders": writeRequests(3),
	}
	finalUnprocessed := map[string][]types.WriteRequest{
		"orders": writeRequests(1),
	}
	client := &fakeClient{
		outputs: []*dynamodb.BatchWriteItemOutput{
			{UnprocessedItems: firstUnprocessed},
			{UnprocessedItems: finalUnprocessed},
		},
	}

	result, err := WriteAll(context.Background(), client, map[string][]types.WriteRequest{
		"orders": writeRequests(3),
	}, WithMaxAttempts(2))
	if err == nil {
		t.Fatal("WriteAll error = nil, want retry exhaustion")
	}
	if !errors.Is(err, ErrUnprocessedItems) {
		t.Fatalf("error = %v, want ErrUnprocessedItems", err)
	}
	var unprocessedErr UnprocessedItemsError
	if !errors.As(err, &unprocessedErr) {
		t.Fatalf("error type = %T, want UnprocessedItemsError", err)
	}
	if unprocessedErr.Attempts != 2 {
		t.Fatalf("error Attempts = %d, want 2", unprocessedErr.Attempts)
	}
	if got := countRequestItems(unprocessedErr.UnprocessedItems); got != 1 {
		t.Fatalf("remaining items = %d, want 1", got)
	}
	if result.Processed != 2 {
		t.Fatalf("Processed = %d, want 2", result.Processed)
	}
}

func TestWriteAllStopsOnContextCancellationBeforeRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &fakeClient{
		outputs: []*dynamodb.BatchWriteItemOutput{
			{UnprocessedItems: map[string][]types.WriteRequest{"orders": writeRequests(1)}},
		},
		afterCall: cancel,
	}

	_, err := WriteAll(ctx, client, map[string][]types.WriteRequest{
		"orders": writeRequests(1),
	}, WithMaxAttempts(2))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestWriteAllWrapsClientError(t *testing.T) {
	cause := &types.ProvisionedThroughputExceededException{
		Message: aws.String("throttled"),
	}
	client := &fakeClient{err: cause}

	_, err := WriteAll(context.Background(), client, map[string][]types.WriteRequest{
		"orders": writeRequests(1),
	})
	if err == nil {
		t.Fatal("WriteAll error = nil, want client error")
	}
	var typed *types.ProvisionedThroughputExceededException
	if !errors.As(err, &typed) {
		t.Fatalf("error = %T %[1]v, want ProvisionedThroughputExceededException", err)
	}
	if typed != cause {
		t.Fatalf("typed error pointer changed")
	}
}

func TestWriteAllRejectsInvalidInput(t *testing.T) {
	client := &fakeClient{}
	cases := []struct {
		name    string
		client  Client
		items   map[string][]types.WriteRequest
		options []Option
		want    error
	}{
		{name: "nil client", client: nil, items: map[string][]types.WriteRequest{"orders": writeRequests(1)}, want: ErrNilClient},
		{name: "nil items", client: client, items: nil, want: ErrEmptyRequestItems},
		{name: "empty table slice", client: client, items: map[string][]types.WriteRequest{"orders": nil}, want: ErrEmptyRequestItems},
		{name: "invalid max attempts", client: client, items: map[string][]types.WriteRequest{"orders": writeRequests(1)}, options: []Option{WithMaxAttempts(0)}, want: ErrInvalidMaxAttempts},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := WriteAll(context.Background(), tt.client, tt.items, tt.options...)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestWriteAllCopiesUnprocessedItems(t *testing.T) {
	unprocessed := map[string][]types.WriteRequest{
		"orders": writeRequests(1),
	}
	client := &fakeClient{
		outputs: []*dynamodb.BatchWriteItemOutput{
			{UnprocessedItems: unprocessed},
			{UnprocessedItems: unprocessed},
		},
	}

	_, err := WriteAll(context.Background(), client, map[string][]types.WriteRequest{
		"orders": writeRequests(1),
	}, WithMaxAttempts(2))
	if err == nil {
		t.Fatal("WriteAll error = nil, want retry exhaustion")
	}
	var unprocessedErr UnprocessedItemsError
	if !errors.As(err, &unprocessedErr) {
		t.Fatalf("error type = %T, want UnprocessedItemsError", err)
	}
	unprocessed["orders"] = nil
	if got := countRequestItems(unprocessedErr.UnprocessedItems); got != 1 {
		t.Fatalf("remaining items after source mutation = %d, want 1", got)
	}
}

func TestWithBackoffDelaysRetries(t *testing.T) {
	var attempts []int
	client := &fakeClient{
		outputs: []*dynamodb.BatchWriteItemOutput{
			{UnprocessedItems: map[string][]types.WriteRequest{"orders": writeRequests(1)}},
			{},
		},
	}

	_, err := WriteAll(context.Background(), client, map[string][]types.WriteRequest{
		"orders": writeRequests(1),
	}, WithBackoff(func(attempt int) time.Duration {
		attempts = append(attempts, attempt)
		return 0
	}))
	if err != nil {
		t.Fatalf("WriteAll error = %v", err)
	}
	if !reflect.DeepEqual(attempts, []int{1}) {
		t.Fatalf("backoff attempts = %v, want [1]", attempts)
	}
}

type fakeClient struct {
	calls     []map[string][]types.WriteRequest
	outputs   []*dynamodb.BatchWriteItemOutput
	err       error
	afterCall func()
}

func (c *fakeClient) BatchWriteItem(_ context.Context, input *dynamodb.BatchWriteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
	c.calls = append(c.calls, cloneRequestItems(input.RequestItems))
	if c.afterCall != nil {
		c.afterCall()
	}
	if c.err != nil {
		return nil, c.err
	}
	if len(c.outputs) == 0 {
		return &dynamodb.BatchWriteItemOutput{}, nil
	}
	out := c.outputs[0]
	c.outputs = c.outputs[1:]
	return out, nil
}

func writeRequests(count int) []types.WriteRequest {
	requests := make([]types.WriteRequest, 0, count)
	for i := 0; i < count; i++ {
		requests = append(requests, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: map[string]types.AttributeValue{
					"id": &types.AttributeValueMemberS{Value: fmt.Sprintf("item-%d", i)},
				},
			},
		})
	}
	return requests
}
