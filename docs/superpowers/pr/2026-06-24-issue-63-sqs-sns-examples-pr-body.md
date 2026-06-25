Resolves #63.

## Summary

- Added `examples/sqs-sns` compile-checked examples for direct AWS SDK for Go v2 SQS/SNS usage.
- Covered SQS send, long-poll receive, manual ack/delete, visibility extension, redrive policy JSON, and SNS to SQS fanout.
- Documented retry, DLQ, visibility timeout, and production SNS-to-SQS queue policy caveats.
- Updated root README package indexes in English and Korean.

## Review

- Step 2-R, Step 3-R, and Step 6-R 7-tier review artifacts are included under `docs/superpowers/reviews/`.
- Step 6-R verdict: P0=0, P1=0.
- Go stress requirement: not applicable to this example-only package because it adds no shared mutable state, worker lifecycle, or goroutine-safe public contract; targeted race, smoke race, and serial full race passed.

## Validation

- PASS `go test -count=1 ./examples/sqs-sns`
- PASS `go test -race -count=1 ./examples/sqs-sns`
- PASS `BLUETAPE_SQS_SNS_EXAMPLE_SMOKE=1 go test -p 1 -count=1 ./examples/sqs-sns`
- PASS `BLUETAPE_SQS_SNS_EXAMPLE_SMOKE=1 go test -race -p 1 -count=1 ./examples/sqs-sns`
- PASS `make fmt-check`
- PASS `make tidy-check`
- PASS `make vet`
- PASS `make lint`
- PASS `go test -p 1 -count=1 ./...`
- PASS `go test -race -p 1 -count=1 ./...`
- PASS `git diff --check`

## DoD Status

- [x] Issue #63 scope implemented with direct AWS SDK examples.
- [x] README and README.ko.md remain synchronized for public package behavior.
- [x] Docker-backed Floci smoke test is opt-in and documented.
- [x] 7-tier review completed with main integration fallback where needed.
- [ ] GitHub CI pending.
