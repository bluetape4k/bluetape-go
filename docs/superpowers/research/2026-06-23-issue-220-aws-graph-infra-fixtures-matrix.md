# Issue #220 AWS/Graph/Infrastructure Fixture Matrix

Issue: [#220](https://github.com/bluetape4k/bluetape-go/issues/220)  
Parent Epic: [#215](https://github.com/bluetape4k/bluetape-go/issues/215)  
Date: 2026-06-23

## 기준선 증거

- 새 #220 worktree에서 `go test ./...`가 한 번
  `github.com/bluetape4k/bluetape-go/ratelimit/redis`에서 실패했다.
  `TestLimiterRefillsFromRedisServerTime`은 refill이 요청을 허용할 것을
  기대했지만, `Allowed=false`와 `RetryAfter=50ms`를 받았다.
- 같은 기준선 실행에서 `testcontainers/*` package는 모두 통과했고,
  `testcontainers/toxiproxy`도 통과했다.
- 이 실패는 #220 fixture 범위 밖이다. 새 Floci slice에 대한 반대 증거가
  아니라 기준선 위험으로 기록한다.

## Candidate 라우팅

| Candidate family | Roadmap consumer | Current evidence | Decision |
|---|---|---|---|
| Floci AWS emulator | #47, #60-#64, #61 | #47은 Floci fixture를 먼저 두라고 말한다. 로컬 문서는 LocalStack보다 Floci-first를 명시한다. 현재 upstream Go module은 `github.com/floci-io/testcontainers-floci-go` pseudo-version `v0.0.0-20260513220955-f6077bc13ae6`로 존재한다. | 이 issue의 첫 slice로 구현한다. |
| LocalStack | #60-#64 | 기존 research는 auth/account/product 적합성 검토가 필요하므로 LocalStack을 compatibility reference로만 남긴다. Testcontainers-Go module은 있지만 선택된 default가 아니다. | 첫 slice로는 연기/거절한다. |
| DynamoDB Local | #64 | DynamoDB repository helper가 선택되면 유용하지만 #64는 아직 research 단계다. | #64로 연기한다. |
| ElasticMQ | #63/#220 | SQS-only example에는 유용하지만 #47/#61은 S3/SQS/SNS/DynamoDB를 한 emulator로 덮기 위해 Floci를 선택한다. | Floci SQS/SNS가 막힐 때만 연기 후 재검토한다. |
| Neo4j/Memgraph/FalkorDB/AGE | #44, #48-#51 | Graph backend 선택은 #50이 명시적으로 gate한다. | #50/#44로 연기한다. |
| Keycloak/Vault/Consul/observability/K3s | Future concrete package or recipe issue | #220은 roadmap issue가 실제 consumer를 만들 때만 평가하라고 한다. 현재 이 fixture를 소비하는 live package는 없다. | 이 slice에서는 연기/no-go다. |
| ChromaDB/Ollama | Optional later LLM/vector work | bluetape-go에는 현재 consumer가 없다. | 연기/no-go다. |

## 선택된 Slice

`testcontainers/floci`를 #220의 첫 slice로 추가하고, 더 넓은 #61
service-example acceptance criteria를 닫지 않은 상태로 #61에 연결한다.

Package는 다음을 노출해야 한다.

- Floci endpoint, region, account ID, availability zone, access key, secret
  key를 typed details와 문서화된 connection detail key로 노출한다.
- AWS SDK configuration value만 필요한 caller를 위해 작은
  `Start(ctx, testing.TB, opts...) Details` API를 제공한다.
- Upstream module feature가 필요한 경우를 위해
  `StartContainer(ctx, testing.TB, opts...) *floci.StartedFlociContainer`
  escape hatch를 제공한다.
- AWS SDK for Go v2 client용
  `LoadConfig(ctx, testing.TB, details, opts...) aws.Config` helper를
  제공한다.

## 연기 항목

- SQS/SNS example은 base Floci fixture가 생긴 뒤 #63에 남긴다.
- DynamoDB repository/conditional-write helper 결정은 #64에 남긴다.
- S3 example 확장은 #62에 남긴다. 이 slice는 fixture 동작을 증명하는
  좁은 S3 smoke test 하나만 포함할 수 있다.
- Graph fixture 작업은 #50/#44에 남긴다.
- Infrastructure/security/observability fixture는 구체 consumer issue가
  생기기 전에는 구현하지 않는다.

## External Dependency Notes

- `github.com/floci-io/testcontainers-floci-go`는 현재 Go 1.25 요구사항과
  Testcontainers-Go v0.42.0을 가진 pseudo-version이다. Repo는 Go 1.26.3과
  Testcontainers-Go v0.42.0을 이미 사용하므로 요구사항은 맞다.
- Upstream module은 기본적으로 `floci/floci:latest`를 사용하고
  `GetEndpoint`, `GetRegion`, `GetAccessKey`, `GetSecretKey`,
  `GetAccountID`, `GetAvailabilityZone`, service config method를 제공한다.
- Wrapper는 AWS SDK service helper를 복제하지 않는다. Emulator setup과
  local AWS config construction을 반복 가능하게 만드는 데 집중한다.

## Closure Update - 2026-06-24

첫 #220 slice는 완료됐다.

- PR #265는 `testcontainers/floci`를 추가했고 GitHub CI run 28031615786을
  통과했다.
- PR #266은 Floci service config smoke coverage를 추가했고 GitHub CI run
  28038766214를 통과했다.
- PR #268은 Floci fixture 위에 direct S3 example을 추가했다.
- PR #269는 Floci fixture 위에 direct SQS/SNS example을 추가했다.
- PR #271은 DynamoDB helper evaluation을 닫고 좁은 package follow-up
  하나만 선택했다.
- PR #272는 `dynamodb/batchwrite`를 추가했고 GitHub CI run 28043143621을
  통과했다.

Consumer issue가 선택하기 전에는 추가 #220 fixture implementation을
더하지 않는다.

- LocalStack, DynamoDB Local, ElasticMQ는 Floci가 accepted 0.9.0 scope의
  S3, SQS, SNS, DynamoDB smoke path를 덮었으므로 fallback-only로 남긴다.
- Neo4j, Memgraph, FalkorDB, PostgreSQL AGE는 graph research #50과 graph
  epic #44가 계속 소유한다.
- Keycloak, Vault, Consul, observability, K3s, ChromaDB, Ollama는 구체
  package, example, integration recipe가 소비할 때까지 연기한다.

이는 모든 listed container를 추가하라는 blanket approval이 아니라,
완료된 first-slice implementation과 roadmap routing decision으로 #220을
닫는 것이다.
