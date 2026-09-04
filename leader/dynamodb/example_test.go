package dynamodbleader_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/bluetape4k/bluetape-go/leader"
	dynamodbleader "github.com/bluetape4k/bluetape-go/leader/dynamodb"
)

type exampleClient struct{}

func (exampleClient) PutItem(context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	return &dynamodb.PutItemOutput{}, nil
}

func (exampleClient) UpdateItem(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	return &dynamodb.UpdateItemOutput{}, nil
}

func (exampleClient) DeleteItem(context.Context, *dynamodb.DeleteItemInput, ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	return &dynamodb.DeleteItemOutput{}, nil
}

func (exampleClient) GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return &dynamodb.GetItemOutput{}, nil
}

func ExampleNew() {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	elector, err := dynamodbleader.New(
		exampleClient{},
		"leader-table",
		leader.Options{
			Group:         "billing-workers",
			MemberID:      "worker-1",
			Lease:         30 * time.Second,
			RenewInterval: 10 * time.Second,
		},
		dynamodbleader.WithLogger(logger),
	)
	if err != nil {
		panic(err)
	}
	if err := elector.Campaign(context.Background()); err != nil {
		panic(err)
	}
	defer func() { _ = elector.Resign(context.Background()) }()
	fmt.Println(elector.IsLeader())
	// Output:
	// true
}
