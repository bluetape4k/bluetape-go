# dynamodb/batchwrite

[English](README.md) | [한국어](README.ko.md)

`dynamodb/batchwrite` is a narrow helper for the AWS SDK for Go v2
`BatchWriteItem` loop. It keeps callers on SDK-native `types.WriteRequest`
maps and only handles the DynamoDB operational contract:

- split writes into 25-item requests,
- retry returned `UnprocessedItems`,
- respect caller `context.Context` cancellation and deadlines,
- return AWS SDK service errors with their typed error intact.

It is not a repository abstraction, mapper, transaction layer, DAX wrapper, or
general DynamoDB client facade.

## Usage

```go
ctx := context.Background()
client := dynamodb.NewFromConfig(cfg)

items := map[string][]types.WriteRequest{
    "orders": {
        {
            PutRequest: &types.PutRequest{
                Item: map[string]types.AttributeValue{
                    "id":     &types.AttributeValueMemberS{Value: "order-1"},
                    "status": &types.AttributeValueMemberS{Value: "created"},
                },
            },
        },
    },
}

result, err := batchwrite.WriteAll(ctx, client, items,
    batchwrite.WithMaxAttempts(5),
    batchwrite.WithBackoff(func(attempt int) time.Duration {
        return time.Duration(attempt) * 100 * time.Millisecond
    }),
)
```

Use the AWS SDK `attributevalue` package or explicit `types.AttributeValue`
values before calling `WriteAll`. Conditional writes and optimistic locking use
normal DynamoDB `PutItem`, `UpdateItem`, or transaction APIs; `BatchWriteItem`
does not support per-item conditions.

## Errors

- `ErrNilClient`: the caller did not provide a DynamoDB client.
- `ErrEmptyRequestItems`: no write requests were provided.
- `ErrInvalidMaxAttempts`: retry attempts were zero or negative.
- `ErrUnprocessedItems`: retries were exhausted and DynamoDB still returned
  `UnprocessedItems`.

Service errors are wrapped with attempt context using `%w`, so callers can keep
using `errors.As` for AWS SDK typed errors.

## Smoke Test

The Floci smoke test is opt-in because it needs Docker:

```bash
BLUETAPE_DYNAMODB_BATCHWRITE_SMOKE=1 go test -p 1 -count=1 ./dynamodb/batchwrite
```
