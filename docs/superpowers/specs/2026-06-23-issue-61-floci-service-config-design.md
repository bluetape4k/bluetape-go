# Issue #61 Floci Service Config Design

Issue: [#61](https://github.com/bluetape4k/bluetape-go/issues/61)  
Stack base: PR #265 / branch `issue-220-aws-graph-infra-fixtures`  
Date: 2026-06-23

## Goal

Complete the next narrow Floci fixture slice by exposing first-party service
config options for S3, SQS, SNS, and DynamoDB and proving those services through
one opt-in Docker smoke test.

## Scope

- Keep the package `testcontainers/floci`.
- Add aliases for upstream Floci service config structs:
  - `S3Config`
  - `SQSConfig`
  - `SNSConfig`
  - `DynamoDBConfig`
- Add default config helpers:
  - `DefaultS3Config`
  - `DefaultSQSConfig`
  - `DefaultSNSConfig`
  - `DefaultDynamoDBConfig`
- Add `ContainerOption` adapters:
  - `WithS3Config`
  - `WithSQSConfig`
  - `WithSNSConfig`
  - `WithDynamoDBConfig`
- Extend the opt-in smoke test behind `BLUETAPE_FLOCI_SMOKE=1` to cover:
  - S3 create bucket, put object, get object;
  - SQS create queue, send, receive, delete;
  - SNS create topic, subscribe SQS queue, publish, receive fanout through SQS;
  - DynamoDB create table, put item, get item.
- Update `README.md` and `README.ko.md`.

## Non-goals

- Do not build S3/SQS/SNS/DynamoDB client wrappers.
- Do not close #62/#63/#64; those issues own richer examples and helper
  decisions.
- Do not attempt to disable every other Floci service in this slice. Upstream
  defaults remain the stability baseline unless a later issue proves a safe
  service-minimal profile.

## Review Notes

- AWS SDK for Go v2 service clients remain caller-owned.
- Floci service config aliases are intentionally thin wrappers over upstream
  config types, so callers can tune supported Floci settings without importing
  upstream directly.
- The smoke stays opt-in to keep normal `go test ./...` stable.

