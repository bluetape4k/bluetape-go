# Issue #270 교훈

- bluetape-go에서 DynamoDB `BatchWriteItem`은 좁은 helper만 가질 가치가 있다. 고가치
  behavior는 repository나 mapper surface가 아니라 25-item chunking과
  `UnprocessedItems` retry다.
- API에 `types.WriteRequest`를 유지하면 두 번째 data model을 피하고 caller가 AWS SDK의
  expression, item, client configuration tool을 계속 직접 사용할 수 있다.
- retry exhaustion은 final unprocessed-item map을 보존해야 한다. caller가 정확한 residual
  work를 log, persist, requeue할 수 있어야 하기 때문이다.
- 이 helper에는 unit test와 race validation을 함께 둔 Floci evidence로 충분하다.
  Testcontainers-style smoke는 일반 `go test ./...`를 빠르게 유지하기 위해 env-gated로
  남긴다.
