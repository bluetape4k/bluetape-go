# Issue #63 SQS/SNS Examples Plan

Issue: [#63](https://github.com/bluetape4k/bluetape-go/issues/63)  
Type: B Fast Track  
Date: 2026-06-24

## Task List

1. Add `examples/sqs-sns` package docs and compile-checked examples.
2. Add example-local JSON codec, receive, ack/delete, visibility, queue ARN, and
   redrive policy helpers.
3. Add opt-in Floci smoke for SQS send/receive/delete/visibility and SNS to SQS
   fanout.
4. Update root README and README.ko package indexes.
5. Run targeted tests, smoke, race, repo gates, 7-tier review, PR, and CI.

## Validation

- `go test -count=1 ./examples/sqs-sns`
- `go test -race -count=1 ./examples/sqs-sns`
- `BLUETAPE_SQS_SNS_EXAMPLE_SMOKE=1 go test -p 1 -count=1 ./examples/sqs-sns`
- `BLUETAPE_SQS_SNS_EXAMPLE_SMOKE=1 go test -race -p 1 -count=1 ./examples/sqs-sns`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make test`
- `make race`
- `git diff --check`
