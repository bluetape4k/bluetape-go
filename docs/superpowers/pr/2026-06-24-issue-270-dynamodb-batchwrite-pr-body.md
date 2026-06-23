Resolves #270.

## Summary

- Added `dynamodb/batchwrite`, a narrow AWS SDK for Go v2 `BatchWriteItem`
  helper for 25-item chunking and `UnprocessedItems` retry.
- Kept the API SDK-native with caller-owned clients, `context.Context`, and
  `map[string][]types.WriteRequest` input.
- Added unit, race, and opt-in Floci smoke coverage for chunking, retry,
  cancellation, error preservation, and real DynamoDB-compatible execution.
- Updated English and Korean package docs plus root package tables.

## Review

- Step 2-R, Step 3-R, and Step 6-R 7-tier review artifacts are included under
  `docs/superpowers/reviews/`.
- Step 6-R verdict: P0=0, P1=0.
- Go stress: `GoroutineStressTester` and `AsyncJobTester` are not applicable;
  the helper owns no goroutines or shared worker state. Cancellation and race
  behavior are covered by targeted tests.

## Validation

- PASS `go test -count=1 ./dynamodb/batchwrite`
- PASS `go test -race -count=1 ./dynamodb/batchwrite`
- PASS `BLUETAPE_DYNAMODB_BATCHWRITE_SMOKE=1 go test -p 1 -count=1 ./dynamodb/batchwrite`
- PASS `BLUETAPE_DYNAMODB_BATCHWRITE_SMOKE=1 go test -race -p 1 -count=1 ./dynamodb/batchwrite`
- PASS `make fmt-check`
- PASS `make tidy-check`
- PASS `make vet`
- PASS `golangci-lint cache clean && make lint`
- PASS `make test`
- PASS `make race`
- PENDING GitHub CI

## DoD Status

- [x] `BatchWriteItem` 25-item chunking implemented.
- [x] `UnprocessedItems` retry and retry-exhaustion error implemented.
- [x] SDK-native caller-owned client contract preserved.
- [x] Unit, race, and Floci smoke tests added.
- [x] English and Korean docs updated.
- [x] 7-tier review completed with P0=0 P1=0.
- [x] Final local gates completed.
- [ ] GitHub CI pending.
