# Issue 560 Provider Benchmark Matrix

## 결정 요약

이 보고서는 각 family 안에서 semantics가 맞는 operation만 비교한다. universal provider
ranking을 만들지 않는다. 측정값은 [environment.md](outputs/issue-560/environment.md)에
기록된 machine에서 Git SHA `1f1069f4b119957d96158c969f819d6374f902c8` 기준으로 캡처한
짧은 local snapshot이다.

- local row는 API 및 implementation lower bound로 사용하고, distributed-provider winner로
  사용하지 않는다.
- leader와 rate-limit provider는 먼저 operational requirement에서 선택한다. 측정된
  latency는 이 fixture의 cost shape를 보여주지만 production availability 또는 durability를
  증명하지 않는다.
- cache boundary를 보존한다. L1은 reference object를 유지하고, L2 serialization과 network
  work가 remote path를 지배한다.
- graph I/O는 local round-trip latency보다 interchange requirement를 먼저 보고 선택한다.
- PostgreSQL recursive CTE는 강한 first-party traversal baseline으로 유지한다. 이 snapshot은
  graph database를 보편적으로 채택하거나 거절할 근거가 아니다.

모든 chart는 관측된 min, median, max `ns/op`을 표시한다. 낮을수록 좋다. allocation data와
선택되지 않은 모든 row는 linked raw Go benchmark output에 남아 있다.

## 재현 및 근거

family 하나씩 실행한다.

```bash
scripts/capture-provider-benchmark.sh leader-local
scripts/capture-provider-benchmark.sh leader-containers
scripts/capture-provider-benchmark.sh leader-probes
scripts/capture-provider-benchmark.sh ratelimit-local
scripts/capture-provider-benchmark.sh ratelimit-containers
scripts/capture-provider-benchmark.sh cache-local
scripts/capture-provider-benchmark.sh cache-redis
scripts/capture-provider-benchmark.sh graphio
scripts/capture-provider-benchmark.sh graphdb
```

각 raw artifact는 정확한 `go test` command, timestamp, Git SHA, benchmark output,
exit status를 기록한다. container artifact는 provider-reported version과 immutable image
reference도 기록한다. 실패한 development capture는 이름에 `-failed-`를 붙여 별도로 보존하고
report input으로 쓰지 않는다. 각 command는 기본적으로 최대 16 MiB를 캡처한다. overflow는
canonical evidence를 교체하지 않고 fail closed한다. 제한된 환경에서는
`BLUETAPE_PROVIDER_BENCH_MAX_OUTPUT_BYTES`로 더 작은 positive bound를 설정할 수 있다.

tracked snapshot을 갱신하려면 다음 순서를 따른다.

1. clean worktree에서 시작하고, measured Git SHA와 현재 host/runtime details를
   `environment.md`에 기록하며, container-backed family를 위해 Docker가 사용 가능한지 확인한다.
2. 위의 아홉 family를 모두 sequential로 실행한다. container-backed capture를 겹치지 않는다.
3. successful raw artifact의 provider-reported version과 immutable image reference를
   `environment.md`에 reconcile한다. provider가 누락, 중복, 불일치하면 중단한다.
4. `node docs/images/readme-charts/generate-provider-benchmark-summaries.mjs --self-test`를
   실행한 다음, 같은 script를 argument 없이 실행해 모든 chart source, SVG, PNG를 재생성한다.
5. 새 raw min/median/max 값으로 report table을 갱신하고 generated Vega-Lite data와
   cross-check한 뒤 repository diff, redaction, visual chart check를 실행한다.

generator는 canonical artifact의 recorded SHA가 `environment.md`의 authority와 다르면
fail closed한다. partial recapture를 complete matrix로 공개하면 안 된다.

## Leader election

### Workload and semantic boundary

ranked row는 disposable Redis, MongoDB, PostgreSQL, etcd fixture를 대상으로 uncontended
campaign, owned resignation, supported contention, leader lookup, lease-expiry takeover를
측정한다. chart는 ordinary operation으로 uncontended campaign과 lookup을 선택하고, intentional
lease wait이 다른 latency class이므로 expiry takeover를 분리한다. etcd contention은
deterministic same-process fixture가 equivalent multi-session contention row를 만들 수 없어서
ranking하지 않는다.

- Local lower bound: [leader-local.txt](outputs/issue-560/leader-local.txt)
- Container commands and rows: [leader-containers.txt](outputs/issue-560/leader-containers.txt)
- Correctness/deadline probes: [leader-probes.txt](outputs/issue-560/leader-probes.txt)

Metric direction: 같은 operation에서는 낮은 `ns/op`이 좋다. active-holder 및
renewal/deadline probe는 correctness evidence이지 ranking input이 아니다.

| Operation | Provider | Median | Observed min-max |
| --- | --- | ---: | ---: |
| Campaign uncontended | Redis | 199.6 us | 193.2-256.6 us |
| Campaign uncontended | MongoDB | 468.8 us | 402.9-503.6 us |
| Campaign uncontended | PostgreSQL | 1.44 ms | 1.32-1.47 ms |
| Campaign uncontended | etcd | 1.98 ms | 1.97-3.68 ms |
| Leader lookup | Redis | 224.9 us | 202.2-231.7 us |
| Leader lookup | MongoDB | 345.3 us | 315.0-351.0 us |
| Leader lookup | PostgreSQL | 661.8 us | 655.9-780.6 us |
| Leader lookup | etcd | 290.4 us | 290.0-387.9 us |
| Expiry takeover | Redis | 1.00 s | 1.00-1.00 s |
| Expiry takeover | MongoDB | 1.01 s | 1.00-1.01 s |
| Expiry takeover | PostgreSQL | 1.01 s | 1.00-1.01 s |
| Expiry takeover | etcd | 2.09 s | 2.06-2.29 s |

![Leader provider latency chart showing min, median, and max for ordinary operations and isolated expiry takeover on a logarithmic axis](../images/readme-charts/provider-benchmark-leader-summary.png)

### Selection guidance

- Redis를 이미 운영하고 있고 low local-network coordination latency가 별도 consensus system을
  추가하는 것보다 중요하면 Redis가 적합하다.
- MongoDB와 PostgreSQL은 해당 database가 이미 operational authority일 때 새 provider를
  피할 수 있게 한다. 서로 다른 operation profile은 application mix에 맞춰 평가해야 한다.
- etcd는 consensus와 operational model이 이미 요구될 때 여전히 적합하다. 이 snapshot의
  expiry behavior는 network round trip뿐 아니라 provider semantics와 polling/deadline behavior를
  포함한다.

증명되지 않은 것: failover availability, clock-skew tolerance, quorum loss, WAN behavior,
renewal under sustained load, production fencing guarantees. priority decision: correctness
probe를 ranking 밖에 유지하고, provider recommendation을 바꾸기 전에 multi-process/load
evidence를 추가한다.

## Rate limiting

### Workload and semantic boundary

provider row는 Redis와 PostgreSQL에 대해 같은 single-key allow/reject decision 및 같은
eight-worker parallel/distinct-key shape를 비교한다. local token bucket은 API lower bound일
뿐 distributed ranking에 참여하지 않는다.

- Local lower bound: [ratelimit-local.txt](outputs/issue-560/ratelimit-local.txt)
- Container rows: [ratelimit-containers.txt](outputs/issue-560/ratelimit-containers.txt)

Metric direction: equivalent allow decision에서는 낮은 `ns/op`이 좋다.

| Scenario | Redis median | PostgreSQL median |
| --- | ---: | ---: |
| Allow available | 311.7 us | 243.1 us |
| Allow rejected | 210.4 us | 223.6 us |
| Parallel, one key, N=8 | 505.3 us | 10.65 ms |
| Parallel, distinct keys, N=8 | 482.9 us | 11.75 ms |

![Rate-limit provider latency chart comparing Redis and PostgreSQL min, median, and max for equivalent allow decisions on a logarithmic axis](../images/readme-charts/provider-benchmark-ratelimit-summary.png)

### Selection guidance

PostgreSQL은 single-key available path에서 더 낮은 median을 보였고, Redis는 rejected path와
두 eight-worker row에서 더 낮았다. transactional colocation과 단일 operational datastore가
여기 관측된 concurrency shape보다 중요하면 PostgreSQL은 여전히 유효한 선택이다. 이 결과는
profiling signal이지 어느 contract가 보편적으로 더 낫다는 evidence가 아니다.

증명되지 않은 것: sustained arrival rate에서의 fairness, multi-host lock contention,
database pool saturation, failover, cloud cost, partition 중 rate-limit accuracy. follow-up
issue draft: high-contention recommendation을 만들기 전에 connection-pool telemetry와 controlled
multi-client arrival curve로 PostgreSQL parallel path를 profile한다.

## Cache

### Workload and semantic boundary

Local memory, Redis L2, tiered cache, Pub/Sub near-cache row는 서로 다른 boundary를 보존한다.
L1 row는 serialization 없이 reference object를 반환한다. L2와 tiered-L2 row는 remote access와
serialization을 포함한다. Pub/Sub peer invalidation은 publication과 observed peer eviction을
포함한다. RESP3 client tracking은 experimental spike로 남으며 production near-cache row가 아니다.

- Local memory and serialization baselines: [cache-local.txt](outputs/issue-560/cache-local.txt)
- Redis, tiered, and Pub/Sub near-cache rows: [cache-redis.txt](outputs/issue-560/cache-redis.txt)

Metric direction: 같은 cache-path semantics 안에서만 낮은 `ns/op`이 좋다.

| Path, 128 B payload | Median | Observed min-max |
| --- | ---: | ---: |
| Memory get hit | 37.7 ns | 37.5-37.9 ns |
| Tiered L1 hit | 193.4 ns | 174.1-273.7 ns |
| Pub/Sub near-cache local hit | 322.5 ns | 254.5-421.7 ns |
| Redis L2 get hit | 181.0 us | 180.9-246.0 us |
| Tiered L2 hit | 178.0 us | 177.2-178.6 us |
| Pub/Sub peer invalidation observed | 314.0 us | 313.3-320.6 us |

![Cache path latency chart separating hot reference-object reads, serialized remote reads, and Pub/Sub invalidation on a logarithmic axis](../images/readme-charts/provider-benchmark-cache-summary.png)

### Selection guidance

- process-local reference semantics와 가장 낮은 hot-read overhead가 필요하면 memory cache를 사용한다.
- local reference-object L1과 serialized Redis L2가 consistency model에 맞으면 tiered cache를
  사용한다. 두 layer를 서로 다른 semantic 및 performance boundary로 취급한다.
- peer invalidation이 필요하고 eventual invalidation semantics가 허용되면 Pub/Sub near cache를
  사용한다. production row는 RESP3 spike가 아니라 Pub/Sub이다.

Batch put은 `N/A: no public bulk mutation contract`다. 증명되지 않은 것: eviction under
memory pressure, subscriber loss/recovery, cluster failover, WAN invalidation lag,
hot-key amplification, production SLO. priority decision: 이 snapshot에서 cache default를
바꾸지 않는다. production near-cache recommendation을 열기 전에 subscriber recovery와 cluster
failover를 측정한다.

## Graph I/O

### Workload and semantic boundary

CSV, NDJSON, GraphML은 같은 generated graph shape를 사용하고 write, read, round-trip,
record-construction baseline을 별도로 측정한다. chart는 medium `10000V-20000E-5P`
round trip만 비교한다. throughput, allocation, construction baseline을 latency axis에 섞지 않는다.

- Exact command and all rows: [graphio.txt](outputs/issue-560/graphio.txt)

Metric direction: 같은 graph shape와 round-trip operation에서는 낮은 `ns/op`이 좋다.

| Format | Median | Observed min-max |
| --- | ---: | ---: |
| CSV | 120.59 ms | 119.66-120.62 ms |
| NDJSON | 87.35 ms | 86.68-87.67 ms |
| GraphML | 215.27 ms | 213.41-221.33 ms |

![Graph I/O chart comparing CSV, NDJSON, and GraphML medium round-trip min, median, and max latency](../images/readme-charts/provider-benchmark-graphio-summary.png)

### Selection guidance

NDJSON은 이 snapshot에서 가장 낮은 medium round-trip median을 보였다. CSV는 단순 tabular
interchange에 여전히 유용하고, GraphML은 graph-oriented interchange contract를 보존한다.
local latency ordering보다 interoperability와 fidelity requirement를 먼저 보고 format을 선택한다.

Graph-store construction은 `N/A: no shared construction API`다. 증명되지 않은 것: streaming
backpressure, compressed transport, malformed-input behavior at scale, disk I/O,
cross-language compatibility, production dataset skew. priority decision: format priority를
유지한다. production default를 권고하기 전에 streaming/backpressure와 malformed-input evidence를
추가한다.

## Graph traversal providers

### Workload and semantic boundary

Neo4j, Memgraph, PostgreSQL recursive CTE fixture는 같은 generated long-chain 및 deep-wide
shape를 받는다. setup, seeding, version query, verification은 timer 밖에 둔다. ranked
operation은 traversal만이다.

- Exact command and all rows: [graphdb.txt](outputs/issue-560/graphdb.txt)

Metric direction: equivalent seeded traversal에서는 낮은 `ns/op`이 좋다.

| Shape | Neo4j median | Memgraph median | PostgreSQL CTE median |
| --- | ---: | ---: | ---: |
| Long chain, depth 16 | 2.16 ms | 537.5 us | 476.9 us |
| Long chain, depth 64 | 1.67 ms | 608.7 us | 1.06 ms |
| Deep-wide, depth 4 fanout 4 | 6.69 ms | 1.51 ms | 493.4 us |

![Graph traversal chart comparing Neo4j, Memgraph, and PostgreSQL recursive CTE min, median, and max latency for equivalent seeded shapes](../images/readme-charts/provider-benchmark-graphdb-summary.png)

### Selection guidance

PostgreSQL recursive CTE는 depth-16 및 deep-wide row에서 가장 낮은 median을 보였고,
Memgraph는 depth 64에서 가장 낮은 median을 보였다. Neo4j는 이 세 local shape에서 더 느렸다.
이 결과는 relational baseline 유지와 application real query mix 측정을 정당화하지만,
provider가 보편적으로 더 낫다는 증거는 아니다.

증명되지 않은 것: Cypher feature breadth, transactional graph updates, index strategy,
concurrent mixed read/write load, cluster behavior, operational tooling, production-scale
datasets. priority decision: 이 row만으로 새 graph provider를 추가하지 않는다.
workload-specific capability와 operational evidence가 필요하다.

## Global limitations

하나의 local Docker snapshot은 production SLO, cloud cost, WAN behavior, failure recovery,
universal winner를 확립하지 않는다. immutable image와 provider-reported version은 이 snapshot을
auditable하게 만들지만 timeless하게 만들지는 않는다. operational recommendation을 바꾸기 전에
deployment-relevant architecture에서 같은 command를 재실행하고 새 environment manifest를 보존한다.
