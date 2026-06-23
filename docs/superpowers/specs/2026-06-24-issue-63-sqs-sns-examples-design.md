# Issue #63 SQS/SNS Examples Design

Issue: [#63](https://github.com/bluetape4k/bluetape-go/issues/63)  
Date: 2026-06-24

## Goal

Add direct AWS SDK for Go v2 examples for SQS producer/consumer and SNS to SQS
fanout, backed by the existing Floci fixture.

## Boundary

- Do not introduce a bluetape SQS or SNS client wrapper.
- Keep `*sqs.Client` and `*sns.Client` caller-owned.
- Add only example-local helper patterns for repeated message codec, manual ack,
  visibility timeout, and redrive policy snippets.
- Keep Docker-backed Floci validation opt-in so normal `go test ./...` remains
  stable.

## Acceptance Mapping

| Acceptance | Decision |
|---|---|
| Send and receive | `examples/sqs-sns` uses `SendMessage` and `ReceiveMessage`. |
| Manual ack/delete | Examples and smoke test call `DeleteMessage` after handler success. |
| Visibility timeout | Examples and smoke test call `ChangeMessageVisibility`. |
| Retry/dead-letter notes | README pair documents receive-count retry and `RedrivePolicy`; helper builds policy JSON. |
| Long polling | Receive helpers use `WaitTimeSeconds`. |
| SNS fanout to SQS | Smoke test creates topic, queue, subscription, publishes, and receives through SQS. |
| Helper patterns | Example-local JSON codec and ack/visibility helpers only; no exported production helper. |
| Floci smoke | `BLUETAPE_SQS_SNS_EXAMPLE_SMOKE=1 go test -p 1 -count=1 ./examples/sqs-sns`. |

## Stress Gate

This package adds examples and no shared mutable state, worker lifecycle, or
goroutine-safe public contract. Race validation is required; a dedicated stress
helper is not applicable for this example-only package.
