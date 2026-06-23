package batchwrite_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/bluetape4k/bluetape-go/dynamodb/batchwrite"
	flocitestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/floci"
)

func TestWriteAllFlociSmoke(t *testing.T) {
	if os.Getenv("BLUETAPE_DYNAMODB_BATCHWRITE_SMOKE") != "1" {
		t.Skip("set BLUETAPE_DYNAMODB_BATCHWRITE_SMOKE=1 to run the Docker-backed Floci DynamoDB batchwrite smoke test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	details := flocitestcontainer.Start(ctx, t, flocitestcontainer.WithDynamoDBConfig(flocitestcontainer.DefaultDynamoDBConfig()))
	cfg := flocitestcontainer.LoadConfig(ctx, t, details)
	client := dynamodb.NewFromConfig(cfg)

	table := "bluetape_batchwrite_" + fmt.Sprintf("%d", time.Now().UnixNano())
	if _, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(table),
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("id"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("id"), KeyType: types.KeyTypeHash},
		},
		BillingMode: types.BillingModePayPerRequest,
	}); err != nil {
		t.Fatalf("create dynamodb table: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = client.DeleteTable(cleanupCtx, &dynamodb.DeleteTableInput{TableName: aws.String(table)})
	})

	requests := make([]types.WriteRequest, 0, 30)
	for i := 0; i < 30; i++ {
		requests = append(requests, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: map[string]types.AttributeValue{
					"id":   &types.AttributeValueMemberS{Value: fmt.Sprintf("item-%02d", i)},
					"name": &types.AttributeValueMemberS{Value: "floci"},
				},
			},
		})
	}

	result, err := batchwrite.WriteAll(ctx, client, map[string][]types.WriteRequest{table: requests})
	if err != nil {
		t.Fatalf("WriteAll smoke error = %v", err)
	}
	if result.Processed != 30 {
		t.Fatalf("Processed = %d, want 30", result.Processed)
	}
	if result.Attempts != 2 {
		t.Fatalf("Attempts = %d, want 2 chunks", result.Attempts)
	}

	scan, err := client.Scan(ctx, &dynamodb.ScanInput{TableName: aws.String(table)})
	if err != nil {
		t.Fatalf("scan dynamodb table: %v", err)
	}
	if got := len(scan.Items); got != 30 {
		t.Fatalf("scan item count = %d, want 30", got)
	}
}
