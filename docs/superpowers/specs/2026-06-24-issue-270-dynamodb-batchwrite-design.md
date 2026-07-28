# Issue #270 DynamoDB BatchWrite Helper Design

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

Issue: #270
Parent research: #64
Work type: Type B fast-track implementation

## 목표

Add a narrow AWS SDK for Go v2 helper that makes DynamoDB `BatchWriteItem`
callers handle the service's 25-item request limit and `UnprocessedItems`
retry loop without introducing a repository, mapper, or generic client wrapper.

## 범위

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

## 검증 Contract

- Unit tests cover chunking, retry, exhausted retry, cancellation, client errors,
  invalid input, defensive copy, and backoff invocation.
- Race test must pass for the package.
- Docker-backed Floci smoke test must prove the helper against a DynamoDB
  endpoint.
- Package README and Korean README stay in sync with root package tables.
