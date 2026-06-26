# AWS Research Boundaries

For AWS work, default to direct AWS SDK for Go v2 clients and make bluetape-go
add value only through local test fixtures, copyable examples, or narrow
helpers with repeated service-specific mechanics.

Floci is the default local AWS-style test fixture. S3, SQS, and SNS remain
example-only. DynamoDB batch write retry is the current narrow helper; broader
repository, mapper, expression, or transaction abstractions are application
owned.

KMS belongs with encryption facade research, not a generic AWS wrapper.
Secrets Manager, Parameter Store, CloudWatch, Logs, Kinesis, IMDS, SES, STS,
RDS IAM, SigV4, and S3 Vectors need concrete consumers before new package
tracks are created.
