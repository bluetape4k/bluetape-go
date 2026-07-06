# SQS/SNS Examples

[English](README.md) | [한국어](README.ko.md)

This package contains compile-checked SQS and SNS examples for AWS SDK for Go
v2 and the `testcontainers/floci` fixture. It intentionally does not provide a
bluetape SQS or SNS client wrapper. The examples keep `*sqs.Client` and
`*sns.Client` caller-owned so application code can use AWS SDK request and
response types directly.

## Scope

- SQS `SendMessage`
- SQS `ReceiveMessage` with long polling
- manual acknowledgement with `DeleteMessage`
- visibility timeout and `ChangeMessageVisibility`
- retry and dead-letter queue configuration notes
- SNS topic publish and SQS fanout
- a small JSON message codec pattern for examples
- local Floci endpoint configuration through `testcontainers/floci`

## Boundary

SQS and SNS stay example-only in this package. Add a bluetape helper only after
a follow-up issue proves repeated Go-specific boilerplate that direct AWS SDK
request types do not express well.

Real AWS SNS to SQS fanout normally also needs an SQS queue policy that allows
the topic ARN to call `sqs:SendMessage`. Floci does not require IAM policy
enforcement for the smoke test, but production code should attach the queue
policy before subscribing the queue.

## Diagram

![SQS/SNS example contract map](../../docs/images/readme-diagrams/examples-sqs-sns-contract-map.png)

The contract map shows the example-only boundary: SQS and SNS clients stay
caller-owned, while local helpers cover JSON payloads, queue calls, Floci smoke
setup, fanout payload unwrapping, and retry/dead-letter notes.

![SQS/SNS message sequence](../../docs/images/readme-diagrams/examples-sqs-sns-message-sequence.png)

The sequence follows the smoke flow from JSON encoding through SQS send,
long-poll receive, visibility/delete acknowledgement, SNS publish, SQS fanout
receive, and `snsPayload` unwrapping.

## Retry And Dead Letter Notes

SQS retry is receive-count based: if a message is received and not deleted, it
becomes visible again after the visibility timeout. Configure a dead-letter
queue with the `RedrivePolicy` queue attribute and keep handler retries bounded
by `context.Context` deadlines.

## Test

Compile-check the examples:

```bash
go test -count=1 ./examples/sqs-sns
```

Run the Docker-backed Floci smoke test:

```bash
BLUETAPE_SQS_SNS_EXAMPLE_SMOKE=1 go test -p 1 -count=1 ./examples/sqs-sns
```

Run Testcontainers-backed packages serially when Docker resources are shared.
