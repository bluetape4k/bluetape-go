# Issue #62 S3 Examples Design

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

Issue: [#62](https://github.com/bluetape4k/bluetape-go/issues/62)  
Stack base: PR #267 / branch `issue-60-aws-helper-boundaries`  
Date: 2026-06-24

## 목표

Add S3 examples that prove the #60 boundary decision: use AWS SDK for Go v2
directly, keep S3 clients caller-owned, and avoid a bluetape service-client
wrapper.

## 범위

- Add `examples/s3` as an examples package, not a production helper package.
- Cover:
  - `PutObject` and `GetObject`;
  - metadata and `ContentType`;
  - streaming upload and streaming download;
  - presigned GET and PUT URLs;
  - S3 not-found error mapping through modeled errors and Smithy API codes;
  - Floci local endpoint configuration with `UsePathStyle`.
- Add English/Korean README files for the example package.
- Add the package to the root README package index.

## Non-Goals

- Do not add a bluetape S3 client wrapper.
- Do not add new dependencies.
- Do not implement KMS or client-side encryption.
- Do not change `testcontainers/floci` behavior.
- Do not merge stacked PRs.

## Evidence

- #60 / PR #267 routes S3 as example-only.
- `testcontainers/floci` already exposes endpoint/config details and requires
  path-style S3 addressing for local endpoints.
- AWS SDK for Go v2 documentation shows direct `s3.Client`, `s3.PresignClient`,
  `PutObject`, `GetObject`, modeled errors, and `BaseEndpoint`/endpoint options
  as the expected integration surface.

## Test Shape

- Normal `go test -count=1 ./examples/s3` compile-checks examples and tests the
  local content-type helper used by the examples.
- `BLUETAPE_S3_EXAMPLE_SMOKE=1 go test -p 1 -count=1 ./examples/s3` starts
  Floci and verifies real S3 calls.
- Race validation runs the same package under `go test -race`.

## Risk Notes

- The smoke test is opt-in to keep ordinary `go test ./...` stable.
- The streaming example uses `io.Pipe` only as a copyable example shape; it does
  not create a public goroutine-safety contract.
- KMS/encryption remains a future issue because key policy and envelope metadata
  require a real consumer.
