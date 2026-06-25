# Issue #220 Floci Testcontainers Wrapper Design

Issue: [#220](https://github.com/bluetape4k/bluetape-go/issues/220)  
Related: [#47](https://github.com/bluetape4k/bluetape-go/issues/47),
[#61](https://github.com/bluetape4k/bluetape-go/issues/61)  
Date: 2026-06-23

## Goal

Add the first #220 AWS fixture slice as a narrow `testcontainers/floci` package
that starts Floci for AWS SDK for Go v2 integration tests and exposes stable
connection details.

## Non-goals

- Do not wrap AWS SDK service clients beyond local test config construction.
- Do not add LocalStack, ElasticMQ, DynamoDB Local, graph DB, infrastructure, or
  observability fixtures in this slice.
- Do not close #61 unless S3/SQS/SNS/DynamoDB service smoke acceptance is fully
  covered; this slice only creates the base Floci fixture and one narrow smoke.

## Package

Create package `testcontainers/floci` with package name
`flocitestcontainer`.

## Public Contract

Constants:

- `EndpointKey = "floci.endpoint"`
- `RegionKey = "floci.region"`
- `AccessKeyIDKey = "floci.access_key_id"`
- `SecretAccessKeyKey = "floci.secret_access_key"`
- `AccountIDKey = "floci.account_id"`
- `AvailabilityZoneKey = "floci.availability_zone"`
- `DedicatedNetworkNameKey = "floci.dedicated_network_name"`

Types:

- `type ContainerOption func(*floci.FlociContainer)`
- `type Details struct { Endpoint, Region, AccessKeyID, SecretAccessKey, AccountID, AvailabilityZone, DedicatedNetworkName string }`

Functions:

- `Start(ctx context.Context, tb testing.TB, opts ...ContainerOption) Details`
- `StartContainer(ctx context.Context, tb testing.TB, opts ...ContainerOption) *floci.StartedFlociContainer`
- `DetailsFromContainer(tb testing.TB, container *floci.StartedFlociContainer) Details`
- `LoadConfig(ctx context.Context, tb testing.TB, details Details, opts ...func(*config.LoadOptions) error) aws.Config`
- `(Details) ConnectionDetails() tcserver.ConnectionDetails`

## Behavior

- `StartContainer` builds `floci.NewFlociContainer()`, applies all caller
  `ContainerOption` values in order, starts the upstream Floci container, and
  registers cleanup with `testing.TB.Cleanup`.
- Cleanup calls upstream `Stop` with a bounded cleanup context derived from the
  test context values but not cancelled by the parent test deadline.
- `Start` returns `DetailsFromContainer(StartContainer(...))`.
- `DetailsFromContainer` fails fast through `testing.TB` on nil container and extracts endpoint,
  region, access key, secret key, account ID, availability zone, and dedicated
  network name.
- `LoadConfig` calls AWS SDK for Go v2 `config.LoadDefaultConfig` with region,
  base endpoint, and static test credentials, then applies additional caller
  options.
- S3 callers must set `UsePathStyle` on their S3 client when using local
  endpoints; document this rather than hiding service-specific behavior.

## Tests

- `TestStartFlociS3Smoke`:
  - opt-in with `BLUETAPE_FLOCI_SMOKE=1` so normal `go test ./...` remains
    stable while targeted Docker lanes still prove the emulator contract;
  - bounded context timeout;
  - start Floci with only S3 enabled when upstream config supports it;
  - call `LoadConfig`;
  - create an S3 bucket through AWS SDK for Go v2;
  - put and get one object;
  - close `GetObject` body;
  - assert returned body.
- `TestDetailsConnectionDetails`:
  - verify all documented keys are populated from `Details`.
- `TestDetailsFromNilContainerFails`:
  - verify nil container fails through `testing.TB`.

## Validation

- `go test -p 1 -count=1 ./testcontainers/floci`
- `go test -race -p 1 -count=1 ./testcontainers/floci`
- `BLUETAPE_FLOCI_SMOKE=1 go test -p 1 -count=1 ./testcontainers/floci`
- `BLUETAPE_FLOCI_SMOKE=1 go test -race -p 1 -count=1 ./testcontainers/floci`
- `go test -p 1 -count=1 ./testcontainers/server ./testcontainers/floci`
- `go test -race -p 1 -count=1 ./testcontainers/server ./testcontainers/floci`
- `rg -n "floci.endpoint|BLUETAPE_FLOCI_ENDPOINT|UsePathStyle|S3|#61|#62|#63|#64" testcontainers/floci/README.md testcontainers/floci/README.ko.md`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make test`
- `make race`
- `git diff --check`

If `make test` reproduces the baseline `ratelimit/redis`
`TestLimiterRefillsFromRedisServerTime` failure, record it as unrelated and
prove the changed Floci package with targeted serial/race tests.

## Acceptance Checklist

| Requirement | Status |
|---|---|
| #220 ties implemented server to live roadmap issue. | Planned: #47/#61. |
| Heavy emulator introduced through narrow smoke test. | Planned: one S3 smoke. |
| LocalStack/MiniStack/Floci choice records risk. | Planned: matrix and spec. |
| Graph fixtures not implemented before #50. | Planned deferral. |
| Infrastructure fixtures require concrete consumer. | Planned deferral. |
