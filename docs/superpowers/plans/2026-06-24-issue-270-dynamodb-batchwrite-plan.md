# Issue #270 Plan

## Steps

1. Add `dynamodb/batchwrite` with SDK-native request and client contracts.
2. Implement deterministic table ordering, 25-item chunking, unprocessed-item
   retry, context-aware backoff sleep, typed retry exhaustion, and result
   aggregation.
3. Add unit tests for request splitting, retry behavior, failure paths,
   cancellation, defensive copy, and option handling.
4. Add opt-in Floci smoke coverage for real DynamoDB-compatible execution.
5. Update package README files and root package tables in English and Korean.
6. Run local verification: targeted tests, race tests, smoke tests, format,
   tidy, vet, lint, and diff checks.
7. Create PR with issue metadata parity, 7-tier review evidence, and `## DoD
   Status` as the final PR body heading.

## Risk Controls

- Keep AWS SDK types visible in the public API to avoid a second modeling layer.
- Do not own credentials, endpoints, table creation, or repository concerns.
- Keep retries bounded by default and caller-configurable.
- Use context cancellation before each call and before delayed retry.
