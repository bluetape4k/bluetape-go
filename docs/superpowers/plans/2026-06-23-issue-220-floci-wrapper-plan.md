# Issue #220 Floci Wrapper Implementation Plan

Issue: [#220](https://github.com/bluetape4k/bluetape-go/issues/220)  
Spec: `docs/superpowers/specs/2026-06-23-issue-220-floci-wrapper-design.md`  
Date: 2026-06-23

## Goal

Add `testcontainers/floci` as the first #220 AWS emulator fixture slice.

## Baseline

Fresh worktree: `issue-220-aws-graph-infra-fixtures` from `origin/develop`
at `1c4f5d4`.

Baseline `go test ./...` failed once in unrelated
`ratelimit/redis TestLimiterRefillsFromRedisServerTime`; all Testcontainers
packages passed. Treat full-suite failures in that package as baseline unless
the new diff touches rate limiting.

## Tasks

### T1 - RED Tests

Create `testcontainers/floci/floci_test.go`.

Cases:

- `TestStartFlociS3Smoke` is gated by `BLUETAPE_FLOCI_SMOKE=1`, starts Floci,
  loads AWS config, creates a bucket, puts one object, gets it back, closes the
  response body, and asserts the body text.
- `TestDetailsConnectionDetails` checks all exported detail keys.
- `TestDetailsFromNilContainerFails` uses a fake `testing.TB` helper only if
  the package needs to prove nil failure behavior without Docker; otherwise
  keep nil behavior documented and covered by code review.

Expected RED:

```bash
go test -p 1 -count=1 ./testcontainers/floci
```

fails because the package/dependencies are not implemented.

### T2 - Dependencies

Add:

- `github.com/floci-io/testcontainers-floci-go@v0.0.0-20260513220955-f6077bc13ae6`
- AWS SDK for Go v2 modules needed by the public helper/tests:
  `aws`, `config`, `credentials`, and `service/s3`.

Run `go mod tidy`.

### T3 - Package Implementation

Create:

- `testcontainers/floci/doc.go`
- `testcontainers/floci/floci.go`

Implementation notes:

- Use upstream `floci.NewFlociContainer()` and apply `ContainerOption` values.
- Register cleanup through `tb.Cleanup` with a bounded cleanup context for
  upstream `Stop`.
- Convert upstream start errors through `testcleanup.FormatStartError` where
  possible; otherwise preserve the causal error.
- Return test credentials only as Floci-local details; do not read production
  AWS credentials.
- `LoadConfig` should prepend region, base endpoint, and static credentials,
  then append caller options.

### T4 - README Pair

Create:

- `testcontainers/floci/README.md`
- `testcontainers/floci/README.ko.md`

Include:

- import path;
- `Start` + `LoadConfig` usage;
- `ConnectionDetails` + `tcserver.ExportEnv` example;
- S3 local endpoint `UsePathStyle` note;
- supported first slice and deferrals to #61/#62/#63/#64;
- Docker/Testcontainers serial execution guidance;
- dynamic endpoint and test-credentials warning.

### T5 - Validation

Run:

```bash
go test -p 1 -count=1 ./testcontainers/floci
go test -race -p 1 -count=1 ./testcontainers/floci
BLUETAPE_FLOCI_SMOKE=1 go test -p 1 -count=1 ./testcontainers/floci
BLUETAPE_FLOCI_SMOKE=1 go test -race -p 1 -count=1 ./testcontainers/floci
go test -p 1 -count=1 ./testcontainers/server ./testcontainers/floci
go test -race -p 1 -count=1 ./testcontainers/server ./testcontainers/floci
rg -n "floci.endpoint|BLUETAPE_FLOCI_ENDPOINT|UsePathStyle|S3|#61|#62|#63|#64" testcontainers/floci/README.md testcontainers/floci/README.ko.md
make fmt-check
make tidy-check
make vet
make lint
make test
make race
git diff --check
```

If `make test` or `make race` reproduces only the baseline `ratelimit/redis`
refill timing failure, rerun the affected command once. If it remains isolated,
record the exact package/test and proceed with targeted Floci evidence.

## Risks

- `floci/floci:latest` can drift. The first slice documents this risk; a pinned
  image follow-up should be filed if CI becomes unstable.
- Upstream `testcontainers-floci-go` currently has no semantic version tag.
  Use the observed pseudo-version and record it in PR evidence.
- Floci S3 local endpoints require path-style S3 clients.
