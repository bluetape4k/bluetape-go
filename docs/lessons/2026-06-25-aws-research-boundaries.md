# AWS Research Boundary 교훈

AWS 작업에서는 direct AWS SDK for Go v2 client를 default로 삼고, bluetape-go는 local
test fixture, copyable example, 또는 반복되는 service-specific mechanic을 가진 좁은
helper로만 가치를 더한다.

Floci는 기본 local AWS-style test fixture다. S3, SQS, SNS는 example-only로 남는다.
DynamoDB batch write retry가 현재의 좁은 helper다. 더 넓은 repository, mapper,
expression, transaction abstraction은 application-owned다.

KMS는 generic AWS wrapper가 아니라 encryption facade research에 속한다. Secrets
Manager, Parameter Store, CloudWatch, Logs, Kinesis, IMDS, SES, STS, RDS IAM, SigV4,
S3 Vectors는 new package track을 만들기 전에 concrete consumer가 필요하다.

향후 AWS issue는 caller가 반복해서 복사할 실제 코드 조각을 먼저 확인해야 한다. SDK 호출을
감싸는 일반 wrapper만으로는 Go caller에게 충분한 가치가 없다. 테스트 fixture, retry 경계,
payload ownership처럼 service별 실패 형태가 분명한 부분만 bluetape-go package 후보로 올린다.
