# Issue #63 SQS/SNS Examples Lesson

## What Changed

`examples/sqs-sns` keeps SQS and SNS as an example-only track instead of
introducing bluetape service-client wrappers. The copyable examples use AWS SDK
for Go v2 clients directly and rely on `testcontainers/floci` only for local
integration tests.

## What To Keep

- Keep AWS service clients caller-owned unless a follow-up issue proves repeated
  Go-specific boilerplate.
- Use opt-in Floci smoke tests for emulator-backed examples so normal
  `go test ./...` stays stable.
- Document retry, visibility timeout, DLQ, and SNS-to-SQS policy caveats rather
  than hiding them behind a broad wrapper.

## Verification Reminder

For future AWS example issues, run the normal package test, opt-in Floci smoke,
race test, and serial Docker-backed race smoke before PR creation.
