# Issue #64 DynamoDB Helper Evaluation

> 한국어 연구 요약: 이 문서는 사용자 협업용 조사/결정 기록이다. 아래 표와 목록의 URL, package name, command, issue number, version, source path는 evidence이므로 그대로 보존한다. 의사결정, 선택/보류/거절 사유, 후속 이슈 경계는 한국어 독자가 바로 이해할 수 있도록 이 요약을 우선 적용한다.
> 추가 한국어 해석: 이 문서에서 영어로 남은 표의 값은 원문 근거이며, 실제 채택 여부는 한국어 결정 문장을 따른다. 후속 작업자는 보류와 거절 항목을 새 구현 범위로 착각하지 않아야 한다.\n

Issue: [#64](https://github.com/bluetape4k/bluetape-go/issues/64)
Date: 2026-06-24
Follow-up helper issue: [#270](https://github.com/bluetape4k/bluetape-go/issues/270)
Workshop example issue: [bluetape-go-workshop#61](https://github.com/bluetape4k/bluetape-go-workshop/issues/61)

## 목표

Decide whether DynamoDB support in `bluetape-go` should add package code,
compile-checked examples, or stay on direct AWS SDK for Go v2 calls.

The default remains direct AWS SDK use. A helper is justified only when it
removes repeated Go-specific integration work that the SDK does not already
express cleanly.

## 근거

- #60 selected DynamoDB as the only AWS helper research candidate after S3,
  SQS, and SNS stayed example-only.
- `testcontainers/floci` already proves DynamoDB availability with direct
  `dynamodb.Client` `CreateTable`, `PutItem`, and `GetItem` calls.
- AWS SDK for Go v2 documentation uses direct `dynamodb.Client`,
  `attributevalue`, `expression`, and DynamoDB paginators for normal item,
  query, and update flows.
- AWS SDK for Go v2 DynamoDB examples document the `BatchWriteItem` 25 item
  request limit.
- `bluetape4k-aws` has a broad JVM/Kotlin surface:
  - `DynamoDbBatchExecutor` chunks batch writes and retries unprocessed items.
  - `DynamoItemMapper` maps application values to DynamoDB item maps.
  - `DynamoDbCoroutineRepository` and Spring/Ktor repositories own framework
    integration around enhanced clients.
  - table creators and DAX auto-configuration are JVM framework/runtime
    surfaces, not Go library primitives.

## 결정 Matrix

| Candidate | Decision | Owner | Rationale |
|---|---|---|---|
| Conditional writes | Example/workshop | bluetape-go-workshop#61 | Direct `ConditionExpression` or `expression` builder is the important user-facing contract. A generic helper would hide DynamoDB semantics too early. |
| Optimistic locking | Example/workshop | bluetape-go-workshop#61 | Version attributes and conflict mapping are domain-specific. Show create-if-absent and compare-and-set style updates in a scenario example. |
| Transactions | Direct SDK | none | `TransactWriteItems`/`TransactGetItems` are SDK-native and app-specific. No current repeated bluetape-go contract. |
| Batch write chunking and retry | Narrow helper | #270 | `BatchWriteItem` requires 25 item chunking and caller-owned retry of `UnprocessedItems`. This is the only repeated, service-specific boilerplate with enough evidence for a Go helper. |
| Expression builders | Direct SDK | none | AWS SDK for Go v2 already ships `feature/dynamodb/expression`. Rewrapping it would duplicate the SDK. |
| Repository conventions | Example/workshop | bluetape-go-workshop#61 | JVM repository interfaces depend on coroutines, enhanced client, Spring, and Ktor. Go repositories should be application-shaped, not a library-wide abstraction. |
| Pagination | Direct SDK | none | SDK paginators already provide Go-native iteration. Examples can show cancellation and page handling if needed. |
| Table bootstrap | Example/test-local | none | Floci tests and examples can create local tables directly. A library bootstrap helper would need schema/migration policy that the repo does not own yet. |
| Item mapping | Direct SDK | none | Use `attributevalue.MarshalMap`/`UnmarshalMap` and explicit structs/maps. A generic mapper would duplicate SDK features or force reflection policy. |
| DAX | Defer | none | DAX is deployment/runtime configuration. The JVM auto-configuration is Spring-specific and does not justify a Go package without a consumer. |
| DynamoDB Local | Fallback only | none | Floci already covers the current smoke path. Use DynamoDB Local only if a selected helper cannot be validated with Floci. |

## Helper Boundary For #270

The accepted helper candidate is limited to batch write execution:

- Caller owns `context.Context` and `*dynamodb.Client`.
- API should accept SDK-native `map[string][]types.WriteRequest` or an
  equivalent narrow struct, not domain entities.
- It must chunk requests into DynamoDB's 25 item limit.
- It must retry returned `UnprocessedItems` with bounded attempts/backoff.
- It must stop promptly on context cancellation or deadline.
- It must preserve AWS SDK typed errors and wrap operational context with `%w`.
- It must not add a repository, mapper, transaction, DAX, Spring/Ktor, or
  generic DynamoDB client wrapper.

## Example Boundary

The already-open workshop issue covers the scenario-shaped conditional
repository example:

- create-if-absent,
- optimistic update conflict,
- query by partition key,
- Floci/local emulator tests,
- README plus Korean README.

That example should stay in `bluetape-go-workshop` because it is user-facing
application guidance rather than a core library primitive.

## Stress Requirement

#64 is docs/research-only and adds no Go runtime code.

For #270, stress tests are required only if the helper adds shared state,
goroutines, worker lifecycle, or a goroutine-safe public claim. Regardless of
stress need, race validation remains mandatory for the changed package.

## 결론

Close #64 after recording this decision. Implement only #270 in
`bluetape-go`, and keep conditional write / optimistic locking guidance in the
workshop example track.
