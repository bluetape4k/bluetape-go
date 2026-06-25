# Issue #64 DynamoDB Helper Evaluation Lessons

- DynamoDB is the only AWS helper candidate with enough evidence after S3,
  SQS, and SNS stayed direct-example-only.
- Do not port JVM repository, mapper, Spring, Ktor, DAX, or enhanced-client
  shapes directly into Go. They are framework surfaces, not bluetape-go
  primitives.
- `BatchWriteItem` is different: 25-item chunking plus returned
  `UnprocessedItems` retry is repeated service-specific boilerplate. Keep the
  Go helper narrow and SDK-native.
- Conditional writes and optimistic locking need scenario examples more than a
  reusable library abstraction. Route that to `bluetape-go-workshop` where the
  README can explain key design and consistency tradeoffs.
