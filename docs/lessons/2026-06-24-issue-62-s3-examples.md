# Issue #62 S3 Examples Lesson

## What Changed

`examples/s3` keeps S3 as an example-only track instead of introducing a
bluetape S3 client wrapper. The copyable examples use AWS SDK for Go v2 clients
directly and rely on `testcontainers/floci` only for local integration tests.

## What To Keep

- Keep AWS service clients caller-owned unless a follow-up issue proves repeated
  Go-specific boilerplate.
- Use opt-in Floci smoke tests for emulator-backed examples so normal
  `go test ./...` stays stable.
- Document KMS/client-side encryption as deferred unless a concrete consumer
  needs key policy, envelope metadata, or compatibility behavior.

## Verification Reminder

For future AWS example issues, run the normal package test, opt-in Floci smoke,
race test, and serial Docker-backed race smoke before PR creation.
