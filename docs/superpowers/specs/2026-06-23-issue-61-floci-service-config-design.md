# Issue #61 Floci Service Config Design

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

Issue: [#61](https://github.com/bluetape4k/bluetape-go/issues/61)  
Stack base: PR #265 / branch `issue-220-aws-graph-infra-fixtures`  
Date: 2026-06-23

## 목표

Complete the next narrow Floci fixture slice by exposing first-party service
config options for S3, SQS, SNS, and DynamoDB and proving those services through
one opt-in Docker smoke test.

## 범위

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

