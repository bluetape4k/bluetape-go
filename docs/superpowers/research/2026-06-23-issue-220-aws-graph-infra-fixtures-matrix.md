# Issue #220 AWS/Graph/Infrastructure Fixture Matrix

Issue: [#220](https://github.com/bluetape4k/bluetape-go/issues/220)  
Parent Epic: [#215](https://github.com/bluetape4k/bluetape-go/issues/215)  
Date: 2026-06-23

## Baseline Evidence

- `go test ./...` on a fresh #220 worktree failed once in
  `github.com/bluetape4k/bluetape-go/ratelimit/redis`:
  `TestLimiterRefillsFromRedisServerTime` expected refill to allow a request
  but received `Allowed=false` with `RetryAfter=50ms`.
- All `testcontainers/*` packages passed in that same baseline run, including
  `testcontainers/toxiproxy`.
- The failure is outside #220 fixture scope and is recorded as a baseline risk,
  not as evidence against the new Floci slice.

## Candidate Routing

| Candidate family | Roadmap consumer | Current evidence | Decision |
|---|---|---|---|
| Floci AWS emulator | #47, #60-#64, #61 | #47 says Floci fixture first. Local docs state Floci-first over LocalStack; current upstream Go module exists at `github.com/floci-io/testcontainers-floci-go` pseudo-version `v0.0.0-20260513220955-f6077bc13ae6`. | Implement first slice in this issue. |
| LocalStack | #60-#64 | Existing research keeps LocalStack as compatibility reference because auth/account/product fit needs review. Testcontainers-Go has a module, but it is not the selected default. | Defer/reject as first slice. |
| DynamoDB Local | #64 | Useful if DynamoDB repository helpers are selected, but #64 is still research. | Defer to #64. |
| ElasticMQ | #63/#220 | Useful for SQS-only examples, but #47/#61 chooses Floci to cover S3/SQS/SNS/DynamoDB under one emulator. | Defer unless Floci SQS/SNS proves blocked. |
| Neo4j/Memgraph/FalkorDB/AGE | #44, #48-#51 | Graph backend selection is explicitly gated by #50. | Defer to #50/#44. |
| Keycloak/Vault/Consul/observability/K3s | Future concrete package or recipe issue | #220 says evaluate only when a roadmap issue creates a real consumer. No live package consumes these fixtures now. | Defer/no-go for this slice. |
| ChromaDB/Ollama | Optional later LLM/vector work | No current consumer in bluetape-go. | Defer/no-go. |

## Selected Slice

Add `testcontainers/floci` as the #220 first slice and connect it to #61 without
closing the broader #61 service-example acceptance criteria.

The package should expose:

- Floci endpoint, region, account ID, availability zone, access key, and secret
  key as typed details and documented connection detail keys.
- A small `Start(ctx, testing.TB, opts...) Details` API for callers that only
  need AWS SDK configuration values.
- A `StartContainer(ctx, testing.TB, opts...) *floci.StartedFlociContainer`
  escape hatch for upstream module features.
- A `LoadConfig(ctx, testing.TB, details, opts...) aws.Config` helper for AWS
  SDK for Go v2 clients.

## Deferrals

- SQS/SNS examples stay in #63 after the base Floci fixture exists.
- DynamoDB repository/conditional-write helper decisions stay in #64.
- S3 example expansion stays in #62; this slice may include one narrow S3 smoke
  test only to prove the fixture works.
- Graph fixture work stays in #50/#44.
- Infrastructure/security/observability fixtures require a concrete consumer
  issue before implementation.

## External Dependency Notes

- `github.com/floci-io/testcontainers-floci-go` is currently a pseudo-version
  with Go 1.25 requirement and Testcontainers-Go v0.42.0. The repo is Go 1.26.3
  and already uses Testcontainers-Go v0.42.0, so the requirement fits.
- The upstream module uses `floci/floci:latest` by default and provides
  `GetEndpoint`, `GetRegion`, `GetAccessKey`, `GetSecretKey`, `GetAccountID`,
  `GetAvailabilityZone`, and service config methods.
- The wrapper should not duplicate AWS SDK service helpers; it should make
  emulator setup and local AWS config construction repeatable.

