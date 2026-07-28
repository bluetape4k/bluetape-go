# Provider Benchmark Matrix 교훈 (#560)

**Related issue:** #560

**Affected packages:** `leader`, `ratelimit`, `cache`, `graph`, `graph/graphio`,
`testcontainers/*`

## L1: timing보다 semantic equivalence가 먼저다

### 문제

같은 metric의 row라도 서로 다른 contract를 측정할 수 있다. local reference-object cache hit, serialized Redis
hit, lease-expiry wait, graph traversal은 모두 `ns/op`를 보고하지만, 하나의 ranking으로 묶으면 각 숫자에 의미를
주는 경계가 지워진다.

### 결정

각 benchmark name은 provider, scenario, concurrency 또는 shape, 필요한 경우 payload를 인코딩한다. report와 chart는
equivalent scenario만 비교한다. deadline-driven leader takeover는 분리하고, local leader/rate-limit row는 distributed
competitor가 아니라 lower-bound API baseline으로 둔다.

### 증거

`leader/provider_benchmark_test.go`, `ratelimit/provider_benchmark_test.go`,
`cache/provider_benchmark_test.go`, `graph/provider_benchmark_test.go`, 그리고
`graph/graphio/provider_benchmark_test.go`는 scenario boundary를 정의한다.
`docs/images/readme-charts/generate-provider-benchmark-summaries.mjs`의 strict chart parser는 reporting 전에
missing, unknown, duplicate, non-finite, failed row를 거부한다.

### 향후 가드

두 provider가 같은 public operation과 verification contract를 노출할 수 없으면 row를 `N/A`로 표시하거나 separate
panel을 사용한다. 서로 다른 semantics를 하나의 benchmark name으로 normalize하지 않는다.

## L2: deadline과 sleep behavior는 ordinary latency가 아니라 probe evidence다

### 문제

lease expiry와 active-holder behavior는 의도적으로 waiting을 포함한다. 이를 ordinary campaign 또는 lookup row와 섞으면
fast operation이 눌려 보이고 잘못된 provider ranking을 유도한다.

### 결정

expiry takeover는 자체 command section과 chart panel을 가진다. active-holder 및 renewal/deadline behavior는 correctness
evidence로 `leader-probes.txt`에 남기고 ranking하지 않는다.

### 증거

`scripts/capture-provider-benchmark.sh leader-containers`는 ordinary section과 expiry section을 독립적으로 실행한다.
chart parser는 두 section을 모두 요구하고 sample을 합치지 않는다.

### 향후 가드

timer가 repeatable provider work를 둘러쌀 때만 ordinary benchmark를 사용한다. 명시적 deadline, sleep, renewal,
failure-transition assertion은 bounded test 또는 probe에 둔다.

## L3: disposable fixture에는 immutable provenance와 observed provenance가 모두 필요하다

### 문제

mutable tag는 의도만 기록할 뿐 실제 실행된 service를 기록하지 않는다. Redis image tag에서 `7.4`를 도출하면 provider가
보고한 `7.4.9`가 숨겨지고, 같은 pinned image가 family별로 일관되지 않아 보인다.

### 결정

모든 container image를 review된 digest로 pin하고, untimed setup 중 service version을 query하며, reported version이 pinned
image authority와 맞는지 검증한다. 모든 successful container artifact와 `environment.md`에 두 값을 모두 기록한다.

### 증거

`docs/research/outputs/issue-560/environment.md`는 immutable fixture 여섯 개와 observed version을 기록한다. leader,
rate-limit, cache, graph benchmark fixture는 version이 없거나 configured authority와 맞지 않으면 fail-closed한다.

### 향후 가드

tag 또는 dependency version을 provider-reported runtime evidence로 대체하지 않는다. immutable image identity와 service
version이 모두 있기 전까지 new provider는 publication에서 `N/A`다.

## L4: concurrent benchmark는 모든 worker를 deterministic하게 join해야 한다

### 문제

첫 winner 또는 error 뒤 바로 반환하면 worker가 cleanup까지 계속 실행되고 다음 iteration을 오염시키며, 완료되지 않은 work의
latency를 보고할 수 있다.

### 결정

concurrent scenario는 common gate 뒤에서 worker를 시작하고, 첫 terminal result에서 peer를 cancel하며, verification 및 cleanup 전에
모든 worker를 join한다. setup, seed, cleanup은 timer 밖에 두고 provider operation과 required completion은 timer 안에 둔다.

### 증거

`TestRunLeaderRoundJoinsAllWorkers`, `TestRunLeaderRoundStartsAllWorkersBeforeWinnerCancellation`, 그리고 rate-limit parallel
benchmark check는 deterministic completion을 보존한다. race test는 invariant를 뒷받침하지만 named join assertion을 대체하지 않는다.

### 향후 가드

새 parallel row는 performance claim에 쓰기 전에 start coordination, first-error propagation, complete join, exact side-effect
count, cleanup ordering을 증명해야 한다.

## L5: parser work와 graph materialization은 별도 비용이다

### 문제

Graph I/O read/round-trip row는 parsing과 graph-store construction을 조용히 섞을 수 있다. 현재 format/provider 간 shared
construction API가 없으므로 construction ranking은 semantically stable하지 않다.

### 결정

Graph I/O는 write, read, round-trip row 옆에 `RecordConstructionBaseline`을 포함하지만 report chart는 equivalent format round
trip만 그린다. graph-store construction은 명시적으로 `N/A: no shared construction API`다.

### 증거

`graph/graphio/provider_benchmark_test.go`는 각 format과 shape에 대해 네 operation을 모두 생성하고, `graphio.txt`는 raw sample
180개를 보존한다. generated graph I/O chart는 medium round-trip row만 선택한다.

### 향후 가드

parser benchmark에서 store-ingestion throughput을 추론하지 않는다. 이 비교를 도입하기 전에 shared construction contract와 별도
검증 fixture를 추가한다.

## L6: raw capture는 atomic, fail-closed여야 하며 failure를 보존해야 한다

### 문제

canonical artifact에 직접 쓰면 failed 또는 interrupted run이 좋은 evidence를 덮어쓸 수 있다. 과도한 redaction은 benchmark
output을 숨기면서 성공처럼 보이게 할 수도 있다.

### 결정

raw output은 repository 밖 private directory에 capture하고, complete artifact를 sanitize 및 rescan한 뒤 zero-exit result만 atomic
publish한다. non-zero 또는 blocked capture는 기존 canonical file을 대체하지 않고 timestamped `-failed-` 이름으로 보존한다. signal
cleanup은 shell unwind가 끝날 때까지 private state를 유지한다.

### 증거

`scripts/capture-provider-benchmark_test.sh`는 clean publication, dirty-source rejection, output confinement, secret blocking,
publication failure, bounded-output behavior를 검증한다. issue output directory는 successful canonical file 9개와 함께 development
failure artifact 4개를 보존한다.

### 향후 가드

benchmark capture helper는 interruption, filesystem failure, redaction failure, stale-canonical preservation을 테스트하기 전에는
complete가 아니다. partial stream을 canonical로 publish하지 않는다.

## L7: universal winner보다 selection guidance가 더 오래 간다

### 문제

Apple Silicon host 하나와 local Docker runtime 하나로는 production SLO, cloud cost, failure recovery, WAN behavior, operational
fit을 증명할 수 없다.

### 결정

report는 min/median/max evidence, operation boundary, provider selection condition, 명시적 `not proven` list를 제시한다. raw
output, environment, table, Vega-Lite source, SVG, PNG를 보존해 future run과 비교할 수 있게 하되 이 snapshot을 timeless하게
취급하지 않는다.

### 증거

`docs/research/2026-07-20-issue-560-provider-benchmark-matrix.md`는 다섯 family decision을 담고 모든 canonical artifact를 link한다.
`README.md`와 `README.ko.md`는 같은 capture surface를 노출하고, 숫자를 production ranking으로 복사하지 말라고 경고한다.

### 향후 가드

operational recommendation을 바꾸기 전 deployment-relevant architecture에서 다시 실행한다. snapshot이 instrumentation gap을 드러내면
benchmark issue 안에서 provider semantics를 바꾸지 말고 telemetry와 workload requirement가 있는 focused follow-up을 만든다.
