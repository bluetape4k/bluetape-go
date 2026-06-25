# Issue #270 DynamoDB BatchWrite Helper Design

Issue: #270
Parent research: #64
Work type: Type B fast-track implementation

## Goal

Add a narrow AWS SDK for Go v2 helper that makes DynamoDB `BatchWriteItem`
callers handle the service's 25-item request limit and `UnprocessedItems`
retry loop without introducing a repository, mapper, or generic client wrapper.

## Scope

- Package: `dynamodb/batchwrite`.
- Input remains SDK-native: `map[string][]types.WriteRequest`.
- Client remains caller-owned: any value with the AWS SDK v2 `BatchWriteItem`
  method can be used.
- `context.Context` controls cancellation and retry sleeps.
- Retry exhaustion returns a typed error that preserves remaining unprocessed
  items and supports `errors.Is(err, ErrUnprocessedItems)`.

## Non-Goals

- Conditional writes, optimistic locking, and transaction helpers.
- Repository abstractions or item mapper conventions.
- DAX, table bootstrap, Spring/Ktor-style surfaces, or generic AWS client
  wrappers.
- Internal worker pools or goroutine ownership.

## API Contract

- `WriteAll(ctx, client, requestItems, options...)` splits request items into
  chunks of at most 25 items.
- Service errors stop immediately and are wrapped with attempt context.
- DynamoDB `UnprocessedItems` are retried within the configured per-chunk attempt
  budget.
- `WithMaxAttempts`, `WithBackoff`, `WithReturnConsumedCapacity`, and
  `WithReturnItemCollectionMetrics` expose only the necessary controls.

## Validation Contract

- Unit tests cover chunking, retry, exhausted retry, cancellation, client errors,
  invalid input, defensive copy, and backoff invocation.
- Race test must pass for the package.
- Docker-backed Floci smoke test must prove the helper against a DynamoDB
  endpoint.
- Package README and Korean README stay in sync with root package tables.
