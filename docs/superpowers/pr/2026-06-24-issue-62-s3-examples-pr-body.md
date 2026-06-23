Resolves #62.

Stacked on #267 / `issue-60-aws-helper-boundaries`.

## Summary

- Added `examples/s3` compile-checked examples for direct AWS SDK for Go v2 S3 usage.
- Covered put/get, metadata, content type detection, streaming upload/download, presigned GET/PUT URLs, and not-found error mapping.
- Documented Floci path-style endpoint setup and KMS/client-side encryption as out of scope until a concrete Go consumer needs it.
- Updated root README package indexes in English and Korean.

## Review

- Step 2-R, Step 3-R, and Step 6-R 7-tier review artifacts are included under `docs/superpowers/reviews/`.
- Step 6-R verdict: P0=0, P1=0.
- Go stress requirement: not applicable to this example-only package because it adds no shared mutable state or public concurrency primitive; smoke and race gates cover the Docker-backed example path.

## Verification

- PASS `go test -count=1 ./examples/s3`
- PASS `go test -race -count=1 ./examples/s3`
- PASS `BLUETAPE_S3_EXAMPLE_SMOKE=1 go test -p 1 -count=1 ./examples/s3`
- PASS `BLUETAPE_S3_EXAMPLE_SMOKE=1 go test -race -p 1 -count=1 ./examples/s3`
- PASS `make fmt-check`
- PASS `make tidy-check`
- PASS `make vet`
- PASS `make lint`
- PASS `make test`
- PASS `make race`
- PASS `git diff --check`

## DoD Status

- [x] Issue #62 scope implemented with direct AWS SDK examples.
- [x] README and README.ko.md remain synchronized for public package behavior.
- [x] Docker-backed Floci smoke test is opt-in and documented.
- [x] 7-tier review completed with main integration fallback where needed.
- [ ] GitHub CI pending.
