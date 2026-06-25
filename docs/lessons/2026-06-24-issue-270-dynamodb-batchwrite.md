# Issue #270 Lessons

- DynamoDB `BatchWriteItem` deserves only a narrow helper in bluetape-go: the
  high-value behavior is 25-item chunking plus `UnprocessedItems` retry, not a
  repository or mapper surface.
- Keeping `types.WriteRequest` in the API avoids a second data model and lets
  callers continue using AWS SDK expression, item, and client configuration
  tools directly.
- Retry exhaustion should preserve the final unprocessed-item map so callers can
  log, persist, or requeue the exact residual work.
- Floci gives enough confidence for this helper when paired with unit tests and
  race validation; Testcontainers-style smoke remains env-gated to keep ordinary
  `go test ./...` fast.
