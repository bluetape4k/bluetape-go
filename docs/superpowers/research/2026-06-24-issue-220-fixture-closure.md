# Issue #220 Fixture Closure

Issue: #220
Parent Epic: #215
Date: 2026-06-24

## Decision

Close #220 after the Floci/AWS fixture slice and 0.9.0 AWS consumer work.

#220 was intentionally broad. The accepted implementation is the `testcontainers/floci`
base fixture plus service-config smoke coverage. Remaining candidate families
are deferred to the roadmap issues that own their consumers rather than kept as
open fixture backlog.

## Completed Evidence

| Area | Evidence | Result |
|---|---|---|
| Floci base fixture | PR #265, CI run 28031615786 | Complete |
| Floci service config smoke | PR #266, CI run 28038766214 | Complete |
| S3 local AWS consumer | PR #268 | Complete |
| SQS/SNS local AWS consumer | PR #269 | Complete |
| DynamoDB helper decision | PR #271 | Complete |
| DynamoDB helper implementation | PR #272, CI run 28043143621 | Complete |

## Deferred Routing

| Candidate | Route | Rationale |
|---|---|---|
| LocalStack | Defer/fallback | Floci covered accepted S3/SQS/SNS/DynamoDB smoke paths without LocalStack account/auth coupling. |
| DynamoDB Local | Defer/fallback | `dynamodb/batchwrite` is validated against Floci; table bootstrap/repository helpers were not selected. |
| ElasticMQ | Defer/fallback | SQS/SNS examples passed through Floci. Add only if a future SQS-only issue proves a blocker. |
| Neo4j/Memgraph/FalkorDB/AGE | #50/#44 | Graph backend selection is a graph research decision, not a Testcontainers expansion decision. |
| Keycloak/Vault/Consul/observability/K3s | Future concrete consumer | No current bluetape-go package consumes these fixtures. |
| ChromaDB/Ollama | Future concrete consumer | No current LLM/vector consumer exists in this repo. |

## #220 Checklist Mapping

- Candidate list converted into slices: completed in the matrix and this closure
  note.
- AWS emulator selection implemented: Floci selected and implemented; fallback
  emulators deferred.
- Graph fixtures evaluated at routing level: deferred to #50/#44.
- Infrastructure/security fixtures evaluated at routing level: deferred until a
  concrete consumer exists.
- License/maintenance/startup/CI considerations recorded: matrix and Floci
  README capture pseudo-version, image behavior, Docker pull/startup caveats,
  and serial CI expectations.
- Concrete follow-up issues: existing #50/#44 own graph; no new AWS emulator
  follow-up is needed after #62, #63, #64, and #270 completed.

## Validation

- PASS `git diff --check`
- PASS `make fmt-check`
- PASS `make tidy-check`
- PASS `make vet`
- PASS `golangci-lint cache clean && make lint`
