# Issue #220 Fixture Closure

Issue: #220
Parent Epic: #215
Date: 2026-06-24

## 결정

Floci/AWS fixture slice와 0.9.0 AWS consumer 작업 이후 #220을 닫는다.

#220은 의도적으로 넓었다. 허용된 구현은 `testcontainers/floci` base
fixture와 service-config smoke coverage다. 남은 candidate family는 열린
fixture backlog로 유지하지 않고, 해당 consumer를 소유한 roadmap issue로
연기한다.

## 완료 증거

| Area | Evidence | Result |
|---|---|---|
| Floci base fixture | PR #265, CI run 28031615786 | Complete |
| Floci service config smoke | PR #266, CI run 28038766214 | Complete |
| S3 local AWS consumer | PR #268 | Complete |
| SQS/SNS local AWS consumer | PR #269 | Complete |
| DynamoDB helper decision | PR #271 | Complete |
| DynamoDB helper implementation | PR #272, CI run 28043143621 | Complete |

## 연기 라우팅

| Candidate | Route | Rationale |
|---|---|---|
| LocalStack | Defer/fallback | Floci가 accepted S3/SQS/SNS/DynamoDB smoke path를 LocalStack account/auth coupling 없이 덮었다. |
| DynamoDB Local | Defer/fallback | `dynamodb/batchwrite`는 Floci 기준으로 검증됐다. Table bootstrap/repository helper는 선택되지 않았다. |
| ElasticMQ | Defer/fallback | SQS/SNS example은 Floci로 통과했다. 미래 SQS-only issue가 blocker를 증명할 때만 추가한다. |
| Neo4j/Memgraph/FalkorDB/AGE | #50/#44 | Graph backend 선택은 Testcontainers expansion 결정이 아니라 graph research 결정이다. |
| Keycloak/Vault/Consul/observability/K3s | Future concrete consumer | 현재 bluetape-go package가 이 fixture들을 소비하지 않는다. |
| ChromaDB/Ollama | Future concrete consumer | 현재 이 repo에는 LLM/vector consumer가 없다. |

## #220 Checklist Mapping

- Candidate list는 matrix와 이 closure note에서 slice로 전환됐다.
- AWS emulator selection은 구현됐다. Floci가 선택 및 구현됐고 fallback
  emulator는 연기됐다.
- Graph fixture는 routing level에서 평가됐다. #50/#44로 연기한다.
- Infrastructure/security fixture는 routing level에서 평가됐다. 구체
  consumer가 생길 때까지 연기한다.
- License/maintenance/startup/CI 고려사항은 matrix와 Floci README에
  기록됐다. Pseudo-version, image behavior, Docker pull/startup caveat,
  serial CI expectation을 포함한다.
- Concrete follow-up issue: 기존 #50/#44가 graph를 소유한다. #62, #63,
  #64, #270 완료 이후 새 AWS emulator follow-up은 필요 없다.

## Validation

- PASS `git diff --check`
- PASS `make fmt-check`
- PASS `make tidy-check`
- PASS `make vet`
- PASS `golangci-lint cache clean && make lint`
