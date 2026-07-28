# Issue #63 SQS/SNS Examples Design

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

Issue: [#63](https://github.com/bluetape4k/bluetape-go/issues/63)  
Date: 2026-06-24

## 목표

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
