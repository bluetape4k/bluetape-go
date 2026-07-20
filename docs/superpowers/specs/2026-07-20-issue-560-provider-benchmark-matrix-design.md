# Issue #560 Provider Benchmark Matrix Design

Status: Approved on 2026-07-20 after Step 2-R convergence
Issue: [#560](https://github.com/bluetape4k/bluetape-go/issues/560)
Milestone: `0.19.0`
Target branch: `perf/issue-560-provider-benchmark-matrix`

## 문제

`bluetape-go`에는 같은 계약을 구현하는 provider가 여러 개 있지만, 현재 비교 가능한
벤치마크 증거는 일부 package에 흩어져 있다. GraphDB의 Neo4j/Memgraph 비교와 일부
cache local-path benchmark는 존재하지만, leader와 rate limiter에는 provider 간 동일
시나리오 비교가 없고 graph I/O에는 format 비교 benchmark가 없다. 기존 결과 역시 한
환경, 한 명령, 한 raw output 집합으로 연결되어 있지 않다.

이 상태에서는 호출자가 다음 질문에 근거 있게 답하기 어렵다.

- 경쟁이 없는 경로와 contention 경로에서 각 provider의 비용은 어느 정도인가?
- local cache hit, Redis L2 hit, tiered miss, near-cache invalidation은 서로 어떤 비용을
  추가하는가?
- CSV, NDJSON, GraphML은 동일한 graph shape를 읽고 쓸 때 어떤 trade-off를 보이는가?
- Neo4j와 Memgraph의 결과는 어떤 fixture와 명령으로 재현할 수 있는가?

이 이슈는 production 기본값이나 provider 순위를 정하는 작업이 아니다. 현재 구현된
provider의 같은 의미 경로를 한 번의 로컬 snapshot으로 측정하고, 원시 결과와 환경,
재현 명령, 해석 한계를 함께 보존하는 작업이다.

## 목표

1. 현재 구현되어 있고 같은 family 안에 두 개 이상의 실제 구현이 있는 항목을 모두
   비교한다.
2. 각 family 안에서 의미가 같은 시나리오만 같은 표와 차트에 배치한다.
3. correctness pre-check를 통과한 fixture만 timer 구간에 진입시킨다.
4. local/in-process baseline과 networked provider 결과를 명시적으로 구분한다.
5. exact command, 실행 일시/시간대, Git SHA, dirty state, hardware/OS/runtime,
   Docker와 provider version, container image digest, raw output을 보존한다.
6. 결과의 metric direction, 관찰 사실, 가설, 선택 기준, 확인하지 못한 사항을 분리한다.
7. production API와 dependency를 추가하지 않고 benchmark/test/document 경계 안에서
   완료한다.

## 범위

### 포함

| Family | 비교 대상 | local baseline |
|---|---|---|
| Leader election | Redis, MongoDB, PostgreSQL, etcd | 필요 시 in-memory fixture를 API overhead 참고값으로만 표시 |
| Rate limiter | Redis, PostgreSQL | `ratelimit.TokenBucket`을 local algorithm baseline으로 별도 표시 |
| Cache | memory L1, Redis L2, tiered L1/L2, Pub/Sub near-cache | path별 의미를 분리하고 서로를 하나의 winner 순위로 합치지 않음 |
| Graph I/O | paired CSV, NDJSON, GraphML | 없음 |
| GraphDB | Neo4j, Memgraph | PostgreSQL recursive CTE를 relational baseline으로, driver record conversion을 local baseline으로 별도 표시 |

### 제외

- 실제 provider가 하나뿐인 Redis lock과 Redis Streams audit publisher
- 아직 구현되지 않은 web adapter, spatial/reverse-geocoding provider
- 열린 이슈인 FalkorDB #547, Kubernetes Lease #557, Consul #558, DynamoDB #559
- production API, exported benchmark abstraction, runtime dependency, provider 기본값 변경
- WAN, multi-region, failover, sustained soak, cloud-managed service, 비용 비교
- 한 환경의 결과를 일반적인 provider 우열 또는 production SLO로 해석하는 주장

새 provider가 이 PR과 독립적으로 merge되더라도 #560 branch를 rebase한 시점에 production
implementation과 conformance coverage가 모두 존재하지 않으면 이번 snapshot에는 넣지 않는다.
후속 provider는 같은 artifact contract로 별도 측정한다.

## 현재 근거와 선택적 채택

| 근거 | 채택 또는 조정 |
|---|---|
| `bluetape4k-leader/benchmark/README.md` | active-holder skip, N-contender contention, mixed acquire/skip, renewal, group semaphore 시나리오를 Go 계약에 맞게 축소 채택 |
| `bluetape4k-graph/docs/benchmark/2026-05-28-graphdb-adoption-decision-report.md` | graph shape별 선택 기준, PostgreSQL relational baseline과 universal winner 금지 원칙 채택 |
| `bluetape4k-graph/benchmark/graph-io-benchmark/README.md` | encode/decode, payload size, graph shape를 분리하는 구조 채택 |
| `bluetape4k-cache-lettuce/Benchmark.md` | L1 hit, L2 hit, miss, write, remove, payload별 분리 채택 |
| `bluetape4k-projects/infra/lettuce/Benchmark.md`와 `redisson/Benchmark.md` | serialization 비용과 Redis round trip을 분리하고 client 특성에 따른 결론을 제한하는 원칙 채택 |
| `bluetape4k-projects/docs/benchmarks/2026-05-29-web-framework-benchmark.md` | request-path 측정 원칙만 참고하며, Go web adapter가 없으므로 실행 범위에서는 제외 |
| `docs/research/benchmark-artifact-template.md` | 환경, 명령, raw output, metric direction, snapshot boundary, traceability 구조 채택 |
| `docs/research/2026-07-09-issue-438-graph-neo4j-benchmark.md` | Testcontainers opt-in, serial execution, chart source와 PNG/SVG 보존 형태 채택 |
| `docs/research/2026-07-09-issue-439-audit-outbox-benchmark.md` | measured/hypothesis/not-proven 분리와 raw evidence 연결 방식 채택 |

JVM 프로젝트의 thread 수, coroutine 구조, JMH 설정과 절대 수치는 Go로 이식하지 않는다.
시나리오의 의미와 증거 보존 규칙만 재사용한다.

## 대안

| 접근 | 결정 | 이유 |
|---|---|---|
| family root의 external test package에 provider matrix를 배치 | 선택 | production package를 추가하지 않고 sibling provider를 import할 수 있으며, 기존 package-local benchmark 관례와 `go test` discovery를 유지한다. |
| `benchmark/providerbench` 중앙 test-only package 신설 | 기각 | 한 곳에서 실행하기 쉽지만 모든 provider fixture와 의미가 결합되고 변경 blast radius가 커진다. test file만 있는 별도 package도 repository 탐색성을 낮춘다. |
| 기존 benchmark 결과만 문서에서 재조합 | 기각 | leader, rate limiter, graph I/O와 Redis L2/tiered의 동등 시나리오가 없어 issue acceptance를 충족하지 못한다. |
| reusable production benchmark framework 추가 | 기각 | benchmark 전용 결합을 exported API와 runtime build에 밀어 넣으며 현재 호출자 요구가 없다. |

## 선택한 구조

```text
family-local benchmark files                 retained evidence

leader/provider_benchmark_test.go       \
ratelimit/provider_benchmark_test.go     \
cache/provider_benchmark_test.go          +--> docs/research/outputs/issue-560/*.txt
graph/graphio/provider_benchmark_test.go  /              |
graph/provider_benchmark_test.go --------/               v
                                             issue-560 report + tables
                                                        |
                                                        v
                                      Vega-Lite source + SVG + PNG
```

새 benchmark file은 모두 external test package 또는 기존 external benchmark package를
사용한다. 공용 production helper를 만들지 않는다. Redis, PostgreSQL, MongoDB는
`testcontainers/redis`, `testcontainers/postgres`, `testcontainers/mongodb`의
`testing.TB` helper를 직접 재사용한다. etcd는 현재 private `_test.go` fixture의
platform별 digest, readiness, client-close 순서를 test-only helper로 추출하거나 동일
계약을 최소 범위로 적용한다. GraphDB는 공식 Testcontainers module과 PostgreSQL helper를
사용한다. provider별 선택은 아래 lifecycle matrix로 고정한다.

| Provider | container/image authority | readiness/schema | teardown |
|---|---|---|---|
| Redis | `testcontainers/redis`와 reviewed multi-arch digest | helper ping; 고유 key prefix | client close, namespace delete, registered container cleanup |
| PostgreSQL | `testcontainers/postgres`와 reviewed multi-arch digest | helper ping; benchmark-owned schema/table | schema drop, DB close, registered container cleanup |
| MongoDB | `testcontainers/mongodb`와 reviewed multi-arch digest | helper ping; 고유 database/collection | collection/database cleanup, client disconnect, registered container cleanup |
| etcd | `leader/etcd`의 platform digest contract | endpoint status; 고유 encoded prefix | exact-prefix delete, client close, registered container cleanup |
| Neo4j/Memgraph | Testcontainers module/request와 reviewed multi-arch digest | driver verify-connectivity; 고유 run property | checked graph delete, driver close, registered container cleanup |

동적 run ID는 내부에서 생성한 최대 32자의 lowercase hexadecimal만 허용한다. caller 입력,
branch 이름, issue 제목을 namespace나 SQL identifier에 넣지 않는다. 값은 parameterized
query/key builder로 전달하고 동적 identifier가 불가피하면 고정 allowlist와 dialect별
quoting을 모두 통과시킨다.

모든 container benchmark는 opt-in 환경 변수로 보호하고 `-p 1`로 순차 실행한다.
로컬 benchmark는 Docker 없이 실행 가능해야 한다. 하나의 provider fixture가 시작에
실패하면 그 command는 실패하며, 나머지 provider 수치만으로 완성된 matrix를 주장하지
않는다.

Container benchmark는 현재 process가 생성하고 identity를 보유한 disposable container만
사용한다. ambient DSN/endpoint override, 공유 개발 DB, cloud/public service 연결을
받지 않는다. provenance를 증명하지 못하면 schema/key cleanup 전에 fail closed한다.
random mapped port와 isolated Docker network를 사용하며 고정 host port를 열지 않는다.

## 공통 측정 계약

### Timer 경계

각 benchmark는 다음 순서를 따른다.

1. container/client/schema/namespace를 준비한다.
2. 한 번 이상의 operation으로 semantic correctness를 확인한다.
3. benchmark마다 고유한 run ID와 key 범위를 만든다.
4. `b.ResetTimer()` 직전에 setup을 끝낸다.
5. timed loop에는 측정 대상 API call과 결과의 최소 검증만 포함한다.
6. cleanup과 fixture 종료는 timer 밖에서 수행한다.

`b.ReportAllocs()`를 켜고 기본 `ns/op`, `B/op`, `allocs/op`를 기록한다. payload를
처리하는 경우 `b.SetBytes`로 `MB/s`도 기록한다. 부가 metric을 추가하면 단위와 방향을
보고서에 정의한다.

Fixture startup/readiness에는 90초, 일반 provider operation에는 10초,
asynchronous invalidation 관찰에는 2초의 상한을 기본으로 둔다. 각 호출은 fresh bounded
context를 사용한다. cleanup은 `context.WithoutCancel`에서 파생한 별도 bounded context로
namespace를 정리한 뒤 client를 닫고 마지막에 container를 종료한다. cancel/close/delete/
terminate 오류를 모두 수집해 benchmark를 실패시킨다.

동시성 시나리오는 test-local barrier, buffered result channel, first-error cancellation,
bounded wait를 공통 protocol로 사용한다. worker goroutine 안에서 `b.Fatal`을 호출하지
않고 모든 결과를 drain/join한 뒤 메인 goroutine에서 판정한다. worker가 남은 round는
실패 증거이며 다음 iteration으로 진행하지 않는다.

### 실행 안정성

- local suite는 최소 `-count=5`로 실행한다.
- container latency suite는 fixture 비용을 제외한 고정 iteration을 위해
  `-benchtime=100x -count=3`을 기본값으로 사용한다. smoke 결과가 command deadline을
  넘길 것으로 보이면 근거와 함께 iteration을 낮추되 provider별로 동일하게 적용하고
  min/median/max를 보고한다. time-window correctness probe는 latency suite와 분리해
  한 번만 실행한다.
- Testcontainers family는 동시에 실행하지 않는다.
- 실행 image는 reviewed platform별 immutable digest로 고정한다. 사람이 읽을 tag와 실제
  resolved digest, Docker platform/client/server, provider가 응답한 service version을
  환경 파일에 함께 기록하고 불일치하면 실행 전에 실패한다.
- 시작 전 `git status --porcelain`이 비어 있어야 한다. 결과 생성 후에는 측정 대상
  코드와 결과 파일만 dirty일 수 있으므로 보고서가 기록하는 SHA와 dirty state를
  명확히 구분한다.
- benchmark 중 provider 오류, cleanup 오류, 잘못된 결과 수가 하나라도 발생하면 raw
  output을 보존하되 비교 표에는 성공값으로 넣지 않는다.
- fixture/operation 실패는 마지막 sanitized error와 필요한 bounded container log만
  남긴다. DSN, URL userinfo, password, token, authorization header, endpoint, host path,
  proxy/registry 설정과 container identifier는 raw output과 `environment.md`에
  commit하지 않는다. allowlisted 환경 필드만 기록하고 artifact 전체에 secret-pattern
  scan을 통과시킨다.

### 해석 계약

`ns/op`, `B/op`, `allocs/op`는 낮을수록 좋고 `MB/s`는 높을수록 좋다. 그러나 의미가
다른 작업은 수치가 같아도 비교하지 않는다. local baseline은 networked provider의
대체재가 아니며 하한선 참고값이다. 표와 chart 제목에는 workload, payload/shape,
contention과 runtime을 포함한다. 보고서는 다음 세 범주를 분리한다.

- measured evidence: raw output에서 직접 확인한 값
- interpretation/hypothesis: 구현과 환경을 근거로 한 설명
- not proven: 다른 환경, 장애, 비용, 확장성, production SLO 등 이번 실행이 증명하지 않은 것

## Family별 시나리오

### Leader election

`leader/provider_benchmark_test.go`는 package `leader_test`에서 Redis, MongoDB,
PostgreSQL, etcd provider를 생성하고 위 lifecycle matrix의 helper/contract를 사용한다.
`BenchmarkProviderLeaderLocal/LocalHarness/CampaignContention/N=8`은 실제 provider가 아닌
test-local atomic one-winner stub으로 barrier-to-join harness overhead만 측정한다. 이 row는
API/동시성 하한선이며 distributed provider 순위나 기능 근거로 사용하지 않는다.

| 시나리오 | 분류 | 의미와 검증 |
|---|---|---|
| `CampaignUncontended` | latency | 빈 group 획득 성공과 `IsLeader` 확인 |
| `ResignOwned` | latency | 획득한 owner의 resign과 backend absence 확인 |
| `CampaignContention/N=8` | round latency | barrier 이후 첫 owner publish, loser cancel/join, 정확히 한 winner와 cleanup 확인 |
| `LeaderLookup` | latency | seeded owner token 조회 |
| `ExpiryTakeover` | operational latency | 원 owner renewal을 중단한 뒤 새 owner 획득; 실제 expiry가 포함된 별도 chart |
| `ActiveHolderCancellation` | correctness probe | 기존 owner 아래 contender를 고정 deadline으로 취소하고 owner 불변 확인; deadline-dominated이므로 provider latency 순위 금지 |
| `RenewalPersistence` | correctness probe | 최소 한 renewal interval 뒤 owner 유지; sleep-dominated이므로 `ns/op` 표에서 제외 |
| `CancellationCleanup` | correctness probe | campaign cancellation 뒤 worker join과 backend cleanup proof |
| `StaleOwnerRejected` | correctness probe | expiry/takeover 뒤 이전 owner의 resign/renew가 새 owner를 제거하지 않음 |

일반 elector가 공통으로 제공하지 않는 group semaphore나 strategic election은 provider
coverage가 대칭이 아니므로 matrix에서 제외한다. latency round는 고유 group을 사용한다.
lease는 고정값 30초, 각 contender attempt와 전체 barrier-to-join round는 각각 5초와
10초 이내로 제한한다. bounded `Resign`, backend exact-absence/owner proof와 renewal
worker join을 완료한 뒤에만 다음 round로 이동한다. expiry 또는 indeterminate cleanup이
발생한 latency round는 실패시킨다. `ExpiryTakeover`만 의도적으로 expiry를 측정하며
contention winner 집계와 섞지 않는다. 이 계약은 기준선에서 관찰된 PostgreSQL 1초 lease
flake가 정상 takeover를 duplicate winner로 오인한 문제를 방지한다.

### Rate limiter

`ratelimit/provider_benchmark_test.go`는 package `ratelimit_test`에서 Redis와 PostgreSQL
limiter를 같은 capacity/refill 정책으로 구성한다.

| 시나리오 | 의미 | 검증 |
|---|---|---|
| `AllowAvailable` | 충분한 token이 있는 단일 key 허용 | allowed=true와 remaining 범위 |
| `AllowRejected` | 소진된 bucket의 거부 | allowed=false와 retry metadata의 유효성 |
| `AllowParallel/N=8` | 같은 key에 동시 요청 | 허용 수가 seeded capacity를 넘지 않음 |
| `AllowDistinctKeys/N=8` | 서로 다른 key에 동시 요청 | 모든 key의 결과가 독립적임 |

local `TokenBucket`은 동일한 distributed semantics를 제공하지 않으므로 `LocalBaseline`
section에만 둔다. Redis와 PostgreSQL의 시간/rounding 차이는 conformance가 허용하는
범위로 normalize하고, exact remaining/retry 값이 아니라 허용 수와 계약 불변식을 비교한다.

### Cache

`cache/provider_benchmark_test.go`는 package `cache_test`에서 memory cache,
`cache/redisvalue.ValueCache`, `cache/redisvalue.TieredCache`,
`cache/redisnear.NearCache`의 대표 경로를 측정한다. 모두 같은 logical record와 128 B,
4 KiB 두 payload profile을 사용한다. L1에는 decoded Go value를 두고 serialization은
Redis L2 경계에서만 발생한다.

| section | 시나리오 |
|---|---|
| `Memory` | hit, miss, set, get-or-load hot |
| `RedisL2` | get hit, get miss, set, delete |
| `Tiered` | L1 hit, L1 miss/L2 hit, L1+L2 miss/load, write-through |
| `NearCachePubSub` | local hit, local miss, publish set/delete, peer invalidation 관찰 |
| `SerializationBaseline` | 같은 serializer의 marshal/unmarshal만 측정 |

RESP3 `CLIENT TRACKING`은 #536의 spike이며 production near-cache 구현이 아니다. 따라서
이번 matrix의 near-cache row는 현재 production Pub/Sub provider만 포함하고, RESP3는
미포함 사유와 후속 채택 조건을 보고서에 기록한다. 서로 다른 section은 별도 표/차트로
표현한다. L1 hit가 Redis L2 hit보다 빠르다는 사실만으로 coherence나 다중 process
정합성 우위를 주장하지 않는다.

현재 cache API에는 batch put이 없으므로 JVM cache precedent의 batch-put row는
`N/A: no public bulk mutation contract`로 보고서에 기록한다. Near-cache invalidation은
subscription ready handshake를 timer 전에 확인하고 subscriber error를 수집한다.
`PeerInvalidation`은 publish부터 peer eviction 관찰까지의 end-to-end latency로 정의하며
2초 안에 관찰되지 않으면 실패한다. priming과 polling setup은 timer 밖에 두고 subscriber,
cache, client background work를 모두 join/close한다.

### Graph I/O

`graph/graphio/provider_benchmark_test.go`는 package `graphio_test`에서 CSV, NDJSON,
GraphML을 같은 deterministic record set으로 비교한다.

| shape | vertex/edge | properties | 목적 |
|---|---:|---|---|
| `Small` | 100 / 200 | scalar 3개 | 고정 overhead |
| `Medium` | 10,000 / 20,000 | scalar 5개 | throughput와 allocation |
| `WideProperties` | 1,000 / 2,000 | scalar 20개 | property encoding 비용 |

각 shape에 대해 `Write`, `Read`, `RoundTrip`, `RecordConstructionBaseline`을
분리한다. Write는 미리 만든 records를
format bytes로 만드는 비용, Read는 timer 밖에서 만든 bytes를 records로 복원하는 비용,
RoundTrip은 둘을 합친 비용이다. CSV의 두 stream bytes는 합산한다. output byte 수와
`MB/s`를 기록하되 density와 latency는 별도 metric으로 다룬다. GraphML의 bounded subset,
CSV property mode, NDJSON ordering을 동일 정보가 round-trip되는 설정으로 고정한다.
`graph/graphio`에는 parser와 record materialization 사이의 public boundary나 공용 graph
store/builder가 없다. 따라서 `Read`는 parser+record materialization end-to-end이고,
`RecordConstructionBaseline`은 동일 record set을 bytes parsing 없이 생성하는 하한선이다.
두 수치를 빼서 parser-only 비용이라고 주장하지 않으며 graph-store construction은
`N/A: no shared construction API`로 명시한다.
`MB/s`는 실제 encoded bytes를 소비하는 Write/Read/RoundTrip에만 보고하고,
`RecordConstructionBaseline`에는 적용하지 않는다. 모든 timed loop는 결과를 test-local
sink에 소비하고 timer 밖에서 불변식을 재확인해 compiler 제거를 방지한다.

### GraphDB

`graph/provider_benchmark_test.go` package `graph_test`에 새 path-shaped container
matrix를 둔다. Neo4j와 Memgraph는 같은 Cypher subset, PostgreSQL은 recursive CTE로
동일하게 도달 가능한 vertex ID 집합을 반환한다.

| shape | seed | query contract |
|---|---|---|
| `LongChain/Depth16` | 단일 chain 64 vertices | root에서 정확히 16 hop까지의 ordered IDs |
| `LongChain/Depth64` | 단일 chain 128 vertices | root에서 정확히 64 hop까지의 ordered IDs |
| `DeepWide/Depth4Fanout4` | 4-ary tree depth 4 | root에서 depth 4 이하 distinct IDs |

모든 runtime은 같은 logical IDs/edges를 seed하고 timer 전 expected ID set을 검증한다.
seed/index/schema setup은 timer 밖에 두며 timed loop는 parameterized traversal과 결과
materialization만 포함한다. 각 sub-benchmark는 `b.ResetTimer`를 명시적으로 호출한다.
기존 `graph/neo4j/benchmark_test.go`의 CRUD/result-mapping row는 adapter regression
baseline으로 재실행할 수 있지만 provider 선택 chart와 path-shaped comparison에는
포함하지 않는다. cleanup error를 무시하는 기존 callback은 checked bounded cleanup으로
수정한 뒤에만 #560 evidence로 사용할 수 있다.

기존 #438 raw artifact는 설계 선례이지 #560의 실행 증거가 아니다. #560 보고서에는 현재
HEAD, 현재 환경에서 새로 실행한 raw output만 measured evidence로 사용한다.

## 실행 명령 계약

Top-level benchmark와 probe 이름은 다음으로 고정한다.

- `BenchmarkProviderLeaderLocal`, `BenchmarkProviderLeaderContainers`,
  `TestProviderLeaderBenchmarkProbes`
- `BenchmarkProviderRateLimitLocal`, `BenchmarkProviderRateLimitContainers`
- `BenchmarkProviderCacheLocal`, `BenchmarkProviderCacheRedis`
- `BenchmarkGraphIOFormats`, `BenchmarkGraphProviderTraversalContainers`

Provider/scenario sub-benchmark는 `Provider/Scenario/ShapeOrPayload` grammar를 사용한다.
최종 report는 아래 command를 축약하거나 재작성하지 않고 실행 header 그대로 보존한다.

```bash
go test -timeout=10m -run '^$' -bench '^BenchmarkProviderLeaderLocal$' -benchmem -count=5 ./leader
go test -timeout=10m -run '^$' -bench '^BenchmarkProviderRateLimitLocal$' -benchmem -count=5 ./ratelimit
go test -timeout=10m -run '^$' -bench '^BenchmarkProviderCacheLocal$' -benchmem -count=5 ./cache
go test -timeout=10m -run '^$' -bench '^BenchmarkGraphIOFormats$' -benchmem -count=5 ./graph/graphio

BLUETAPE_LEADER_PROVIDER_BENCH=1 go test -timeout=30m -p 1 -run '^$' -bench '^BenchmarkProviderLeaderContainers$/(Redis|MongoDB|PostgreSQL|etcd)/(CampaignUncontended|ResignOwned|CampaignContention|LeaderLookup)$' -benchtime=100x -count=3 -benchmem ./leader
BLUETAPE_LEADER_PROVIDER_BENCH=1 go test -timeout=10m -p 1 -run '^$' -bench '^BenchmarkProviderLeaderContainers$/(Redis|MongoDB|PostgreSQL|etcd)/ExpiryTakeover$' -benchtime=1x -count=3 -benchmem ./leader
BLUETAPE_LEADER_PROVIDER_BENCH=1 go test -timeout=15m -p 1 -run '^TestProviderLeaderBenchmarkProbes$' ./leader
BLUETAPE_RATELIMIT_PROVIDER_BENCH=1 go test -timeout=30m -p 1 -run '^$' -bench '^BenchmarkProviderRateLimitContainers$' -benchtime=100x -count=3 -benchmem ./ratelimit
BLUETAPE_CACHE_PROVIDER_BENCH=1 go test -timeout=30m -p 1 -run '^$' -bench '^BenchmarkProviderCacheRedis$' -benchtime=100x -count=3 -benchmem ./cache
BLUETAPE_GRAPH_PROVIDER_BENCH=1 go test -timeout=30m -p 1 -run '^$' -bench '^BenchmarkGraphProviderTraversalContainers$' -benchtime=100x -count=3 -benchmem ./graph
```

환경 변수 하나는 한 family만 제어한다. 일반 `go test ./...`와 `make ci`는 container
benchmark를 실행하지 않는다. `scripts/capture-provider-benchmark.sh <family>`를 단일
재현/capture entry point로 추가한다. 이 script는 위 allowlisted command만 실행하고,
command/UTC timestamp/Git SHA header, combined stdout/stderr와 exit status를 repository 밖의
private 임시 파일에 기록한다. 성공한 전체 실행만 redaction scan 후 canonical `.txt`로
atomic rename한다. 실패 output도 먼저 고정 규칙으로 sanitize하고 재검사한 뒤
`<family>-failed-<timestamp>.txt`에 보존하며 마지막 성공 artifact를 덮어쓰지 않는다.
정제 후에도 금지 패턴이 남으면 output 본문은 폐기하고 command/exit/redaction-blocked
metadata만 보존한다. script 자체는 원래 non-zero exit를 그대로 반환한다.

## Artifact 구조

```text
docs/research/2026-07-20-issue-560-provider-benchmark-matrix.md
docs/research/outputs/issue-560/
  environment.md
  leader-local.txt
  leader-containers.txt
  ratelimit-local.txt
  ratelimit-containers.txt
  cache-local.txt
  cache-redis.txt
  graphio.txt
  graphdb.txt
scripts/capture-provider-benchmark.sh
docs/images/readme-charts/
  provider-benchmark-<family>-summary.vl.json
  provider-benchmark-<family>-summary.svg
  provider-benchmark-<family>-summary.png
  generate-provider-benchmark-summaries.mjs
```

한 chart에 의미가 다른 family를 합치지 않는다. chart는 대표적인 동일 시나리오의
`ns/op`만 보여주고, allocation과 상세 row는 표와 raw output에 남긴다. Vega-Lite source,
generator, SVG, PNG를 모두 version control에 보존하고 `bluetape-diagram`의 렌더링/시각
검증 절차를 따른다. 모든 chart는 인접한 동일 데이터 표, 구체적인 alt text, 단위 label,
충분한 contrast와 pattern/label을 제공하고 색상만으로 provider를 구분하지 않는다.

`environment.md`는 UTC와 local timestamp/timezone, OS/kernel/arch, CPU model, logical
CPU 수, RAM, Go version, Git SHA와 pre/post dirty state, Docker client/server/platform,
각 configured tag와 resolved digest, provider-reported service version을 필수로 기록한다.
민감한 host/container identifier와 전체 환경 dump는 금지한다.

보고서는 각 family마다 다음 순서를 사용한다.

1. workload와 semantic boundary
2. environment와 exact command/raw output 링크
3. metric direction
4. 결과 표와 chart
5. measured evidence
6. 선택 기준과 제한된 해석
7. caveat와 not proven
8. provider priority/instrumentation follow-up decision

최종 report는 영어로 작성하고 `README.md`와 `README.ko.md`에 같은 report 링크,
snapshot caveat와 capture script 사용법을 각각 자연스럽게 추가한다. README에는 결과
숫자를 복제하지 않아 stale snapshot을 만들지 않는다. 한 family라도 실패하면 report에
`blocked/incomplete`를 표시하고 성공 family만으로 전체 issue가 완료됐다고 안내하지 않는다.

측정 결과가 provider 우선순위를 바꾸거나 instrumentation gap을 드러내면 기존 issue를
링크하고 follow-up issue 초안을 report에 남긴다. 새 GitHub issue 생성은 현재 PR 생성
승인에 포함되지 않으므로 별도 사용자 승인을 받는다. 이 PR은 runtime default를 바꾸지
않으며 rollback은 benchmark/test/docs artifact commit의 revert다.

## 실패 모드와 방어

| 실패 모드 | 방어 |
|---|---|
| 의미가 다른 operation을 한 순위표에 배치 | family/section/scenario를 모두 같게 한 row만 비교하고 local baseline을 분리 |
| container startup, CPU scaling, Docker VM 잡음이 결과를 지배 | image/platform/environment 보존, family별 순차 실행, 반복값과 snapshot 한계 공개 |
| 짧은 lease가 contention 중 만료되어 두 winner로 관찰 | operation 최악 시간의 10배 이상 lease, correctness pre-check, expiry takeover를 timed winner로 세지 않음 |
| 이전 run의 key/schema/data가 결과를 오염 | 고유 run ID와 namespace, timer 밖 seed/cleanup, cleanup failure 시 command 실패 |
| serialization 비용을 Redis RTT로 오인 | serializer-only baseline과 Redis L2 operation을 별도 section으로 측정 |
| 일부 provider 실행 실패 후 불완전 matrix를 완성으로 표시 | 실패 raw output을 보존하고 해당 family report/DoD를 blocked로 유지 |
| generated table/chart와 raw output 불일치 | raw output에서 결과를 추출해 source data를 만들고 row-to-file traceability를 자체 검토 |
| benchmark 때문에 평상시 test가 느려지거나 Docker를 요구 | container suite opt-in, 일반 test에서 skip, `-p 1` 재현 명령 제공 |
| 결과 생성 전후 Git 상태 혼동 | 측정 대상 SHA와 pre-run clean 상태, artifact 생성 후 dirty 상태를 environment에 각각 기록 |
| raw output에 DSN/secret/host 정보가 남음 | allowlisted environment capture, sanitized errors, artifact secret-pattern scan 실패 시 commit 금지 |
| ambient/shared service를 cleanup | current-process disposable container provenance 확인 실패 시 mutation 전 fail closed |
| command 실패가 기존 성공 artifact를 덮음 | combined output/exit status를 실패 파일로 보존하고 성공 결과만 atomic rename |

## 검증

### 코드와 benchmark

- 변경 benchmark package targeted tests
- local benchmark smoke (`-benchtime=1x`)와 최종 `-count=5`
- container benchmark smoke와 최종 family별 serial command
- `scripts/capture-provider-benchmark.sh` family별 성공/실패/atomic-replace test
- `go test ./...`
- `make fmt-check`, `make tidy-check`, `make vet`, `make lint`, `make test`, `make race`
- `make ci`

Testcontainers-backed package를 동시에 실행하지 않는다. `make test`와 `make ci`에서 발견된
기존 flake와 새 regression을 구분하고, 새 실패는 수리 전까지 완료로 처리하지 않는다.

### 문서와 chart

- `git diff --check`
- 모든 표 row가 exact command와 raw output file로 추적되는지 확인
- raw output/environment allowlist와 secret-pattern scan
- metric direction과 snapshot boundary가 각 비교 section 앞에 있는지 확인
- universal winner/production ranking 표현 검색
- generator를 다시 실행해 committed SVG/PNG와 일치하는지 확인
- SVG와 PNG를 실제 크기로 열어 label, clipping, color, scale을 시각 검토
- adjacent data table, alt text, contrast, non-color-only provider 식별 확인

## Acceptance mapping

| Issue acceptance | 설계 증거 |
|---|---|
| 현재 provider family별 비교 artifact | 포함 범위의 다섯 family, family-local benchmark와 단일 집계 report |
| 결과 표 | report의 family/scenario별 표 |
| chart | 의미가 같은 대표 row만 담는 family별 Vega-Lite/SVG/PNG |
| 분석 | measured, interpretation, selection rule, not proven 분리 |
| raw output | `docs/research/outputs/issue-560/*.txt` |
| 재현 명령과 환경 | capture script, verbatim command header와 필수-field `environment.md` |
| caveat | snapshot boundary, local/network 분리, noise와 semantic 제한 |
| 보편적 winner 주장 금지 | family/scenario별 선택 기준만 기록하고 production ranking 금지 검토 |

## Step 2-R review

| Lens | 최초 결과 | 반영 | 최신 결과 |
|---|---:|---|---:|
| Performance | P0=0, P1=4 | deadline/sleep 지배 leader row를 probe로 분리하고 누락된 leader 경로, path-shaped GraphDB/PostgreSQL baseline, graph I/O construction boundary 추가 | P0=0, P1=0 |
| Stability | P0=0, P1=4 | bounded context/cleanup, barrier와 worker join, deterministic lease round, near-cache readiness/error contract 추가 | P0=0, P1=0 |
| Security | P0=0, P1=3 | artifact redaction/scan, disposable-container provenance, immutable image digest, bounded namespace와 random port 계약 추가 | P0=0, P1=0 |
| Operator/Ops | P0=0, P1=4 | provider lifecycle matrix, 필수 environment field, overall timeout/diagnostics, atomic raw capture 추가 | P0=0, P1=0 |
| Developer/API | P0=0, P1=3 | GraphDB workload와 timer/pre-check 수정, shared fixture reuse와 exact benchmark 이름 확정 | P0=0, P1=0 |
| User/caller | P0=0, P1=2 | bilingual README discoverability, 단일 capture entry point, partial-result blocking과 chart accessibility 추가 | P0=0, P1=0 |
| Main integration | P0=0, P1=0 | issue acceptance, repo conventions, public API/dependency boundary와 side-effect gate 재확인 | P0=0, P1=0 |

Performance native lane은 제한 시간 안에 결과를 반환하지 않아 중단했고 main-session
performance fallback으로 검토했다. User/caller native lane은 session thread 한도 때문에
시작하지 못해 main-session equivalent review로 대체했다. 두 fallback 모두 같은 artifact와
evidence-only severity contract를 사용했다. Stability, Security, Operator/Ops,
Developer/API lane은 수정 후 재검토에서 PASS를 반환했다. 모든 P2는 이 spec에 반영하거나
batch put과 graph-store construction처럼 현재 API가 없는 경우 근거 있는 `N/A`로
명시했다.

## 완료 조건

- 포함된 다섯 family가 모두 correctness pre-check와 최종 benchmark command를 통과한다.
- 각 성공 수치가 current-head raw output과 exact command로 역추적된다.
- report, tables, chart source, SVG, PNG와 environment artifact가 모두 존재한다.
- public API와 production dependency 변화가 없다.
- `README.md`와 `README.ko.md`가 같은 benchmark report link, snapshot caveat와
  capture script 사용법을 제공한다.
- 계획된 검증과 7-Tier review에서 P0/P1이 0이다.
- Type A lesson gate가 reusable learning 또는 근거 있는 `N/A`로 닫힌다.
- PR은 `develop <- perf/issue-560-provider-benchmark-matrix`로 생성하되 merge는 CI, review,
  thread 확인 후 별도의 최신 사용자 승인을 기다린다.
