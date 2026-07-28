# Issue #64 DynamoDB Helper Evaluation 교훈

- S3, SQS, SNS가 direct-example-only로 남은 뒤 충분한 evidence를 가진 AWS helper
  candidate는 DynamoDB뿐이다.
- JVM repository, mapper, Spring, Ktor, DAX, enhanced-client shape를 Go로 직접
  port하지 않는다. 그것들은 framework surface이지 bluetape-go primitive가 아니다.
- `BatchWriteItem`은 다르다. 25-item chunking과 반환된 `UnprocessedItems` retry는
  반복되는 service-specific boilerplate다. Go helper는 좁고 SDK-native하게 유지한다.
- conditional write와 optimistic locking은 reusable library abstraction보다 scenario
  example이 더 필요하다. key design과 consistency tradeoff를 README에서 설명할 수 있는
  `bluetape-go-workshop`으로 보낸다.
