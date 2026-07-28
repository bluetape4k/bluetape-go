# Issue #560 provider benchmark matrix 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 필수 하위 스킬: 사용 superpowers:subagent-driven-development (권장) 또는 superpowers:executing-plans to 이 계획을 작업 단위로 구현. 단계는 checkbox (`- [ ]`) 추적 문법을 사용.

**목표:** 추가 reproducible, semantically bounded benchmark matrices 및 retained evidence for every currently implemented multi-provider family in issue #560.

**아키텍처:** 유지 each benchmark harness in its family root as external 테스트 code, reuse 기존 Testcontainers helpers, 및 add 없음 production API 또는 runtime dependency. A fail-safe capture script records exact commands 및 sanitized raw output; one 영문 report, bilingual README links, 및 reproducible chart sources aggregate 만 equivalent scenarios.

**기술 스택:** Go 1.26.3 benchmarks/테스트, 기존 Redis/MongoDB/PostgreSQL/etcd/Neo4j/Testcontainers dependencies, POSIX shell, Node.js chart generators, Vega-Lite JSON, SVG/PNG, Markdown.

---

## 파일 지도

생성:

- `leader/provider_benchmark_test.go` — leader latency rows, deterministic concurrent rounds, 및 correctness probes.
- `ratelimit/provider_benchmark_test.go` — Redis/PostgreSQL rate-limiter rows 및 local baseline.
- `cache/provider_benchmark_test.go` — memory, Redis L2, tiered, near-cache, 및 serializer sections.
- `graph/graphio/provider_benchmark_test.go` — CSV/NDJSON/GraphML shapes 및 construction baseline.
- `graph/provider_benchmark_test.go` — Neo4j/Memgraph traversal 및 PostgreSQL recursive-CTE baseline.
- `scripts/capture-provider-benchmark.sh` — al낮음listed family execution 및 atomic raw-output capture.
- `scripts/capture-provider-benchmark_test.sh` — success, failure, redaction, 및 atomic-replacement 계약.
- `docs/research/outputs/issue-560/environment.md` — sanitized environment manifest.
- `docs/research/outputs/issue-560/*.txt` — exact successful benchmark/probe outputs.
- `docs/research/2026-07-20-issue-560-provider-benchmark-matrix.md` — final report.
- `docs/images/readme-charts/generate-provider-benchmark-summaries.mjs` — raw-output parser 및 chart renderer.
- `docs/images/readme-charts/provider-benchmark-*-summary.vl.json` — chart data/spec sources.
- `docs/images/readme-charts/provider-benchmark-*-summary.svg` — reviewable vector charts.
- `docs/images/readme-charts/provider-benchmark-*-summary.png` — Markdown-compatible raster charts.
- `docs/lessons/2026-07-20-issue-560-provider-benchmark-matrix.md` — Type A reusable lesson.

Modify:

- `testcontainers/redis/redis.go`, `redis_test.go` — immutable Redis image authority.
- `testcontainers/postgres/postgres.go`, `postgres_test.go` — immutable PostgreSQL image authority.
- `testcontainers/mongodb/mongodb.go`, `mongodb_test.go` — immutable MongoDB image authority.
- `graph/neo4j/benchmark_test.go` — checked bounded cleanup 및 immutable Neo4j/Memgraph images.
- `README.md`, `README.ko.md` — matching report link, snapshot caveat, 및 capture command.

다음을 하지 않는다: modify exported production contracts in `leader`, `ratelimit`, `cache`,
`graph`, 또는 `graph/graphio`. 다음을 하지 않는다: add dependencies 또는 benchmark live/cloud/공개
services.

## spec coverage matrix

| Spec requirement | Plan proof |
|---|---|
| Five implemented multi-provider families | Tasks 2-6 |
| Equivalent scenarios 및 local/network separation | Tasks 2-6 family-specific sub-benchmark grammar |
| Fixture reuse 및 immutable provenance | 작업 1 및 Tasks 2-6 fixture construction |
| Bounded contexts, worker joins, checked cleanup | Tasks 2-6 focused 계약 테스트 |
| Leader acquire/resign/contention/expiry/cancellation/stale-owner coverage | 작업 2 latency rows 및 probes |
| Cache L1/L2/tiered/near-cache plus serialization | 작업 4 |
| Graph I/O parser/materialization 및 construction boundary | 작업 5 |
| Path-shaped GraphDB plus PostgreSQL baseline | 작업 6 |
| Exact commands, redaction, failure-safe raw output | 작업 7 |
| Fresh current-HEAD evidence 및 environment | 작업 8 |
| Tables, charts, selection analysis, caveats | 작업 9 |
| Bilingual discoverability | 작업 9 |
| No 공개 API/dependency/default change | Tasks 1-10 diff 및 module checks |
| Full verification 및 P0/P1=0 | Tasks 10-11 |

## 단계 3-R Plan 리뷰 기록

The review target was commit `7bb2a79`; repairs be낮음 are applied in the next plan commit.

| Lens | Execution | Initial P0/P1 | Resolution |
|---|---|---:|---|
| Performance | Native lane timed out; main integration fallback performed | 0/3 | Defined the local leader baseline, separated expiry timing, 및 fixed Graph I/O/result-consumption boundaries |
| Stability | Independent verifier lane | 0/4 | Added blocked-peer drain 테스트, exact-once lifecycle ordering, serial 패키지 gates, 및 bounded expiry policy |
| Security | Independent code-reviewer lane | 0/2 | Verified exact digest references 및 made both success/failure capture sanitize 전에 retention |
| Operator/Ops | Native lane creation stalled; main integration fallback performed | 0/2 | Made service-version collection post-capture 및 made PNG rasterization deterministic/fail-closed |
| Developer/API | Main-session equivalent 후 native runtime stall | 0/2 | Removed capture-name drift 및 avoided double container termination while retaining 기존 helpers |
| User/호출자 | Main-session equivalent 후 native runtime stall | 0/1 | Kept README reproduction arguments exact 및 separated non-comparable chart/report sections |

Main integration result 후 repair: `P0=0`, `P1=0`. The stalled native paths were 아님 waited on
again; the main session completed those read-만 perspectives as required by the repository
fallback rule.

### 작업 1: Pin Container Provenance Without Changing Helper APIs

**파일:**
- Modify: `testcontainers/redis/redis.go`
- Modify: `testcontainers/redis/redis_test.go`
- Modify: `testcontainers/postgres/postgres.go`
- Modify: `testcontainers/postgres/postgres_test.go`
- Modify: `testcontainers/mongodb/mongodb.go`
- Modify: `testcontainers/mongodb/mongodb_test.go`
- Modify: `graph/neo4j/benchmark_test.go`

- [ ] **단계 1: Resolve reviewed multi-architecture image digests**

사용 the reviewed multi-architecture index digests be낮음. Before editing code, inspect each exact
tag-plus-digest reference 및 verify that it contains both `linux/amd64` 및 `linux/arm64`
descriptors:

```bash
docker manifest inspect --verbose redis:7.4-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99
docker manifest inspect --verbose postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777
docker manifest inspect --verbose mongo:7.0@sha256:340c1c56fb10e95cf79ff547f8664b96bc6ead9909bc355238cbf865a9695a6f
docker manifest inspect --verbose neo4j:5.26.0@sha256:5a015e53de1895e7eee1574ae0325cf8c4b89587222778108c594bdd45a474b5
docker manifest inspect --verbose memgraph/memgraph:3.5.0@sha256:b411deeb2341698f4f7a0d69535c8937c341e924f66962aa3e70acb63c7a5bd1
```

| Image tag | Reviewed index digest |
|---|---|
| `redis:7.4-alpine` | `sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99` |
| `postgres:16-alpine` | `sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777` |
| `mongo:7.0` | `sha256:340c1c56fb10e95cf79ff547f8664b96bc6ead9909bc355238cbf865a9695a6f` |
| `neo4j:5.26.0` | `sha256:5a015e53de1895e7eee1574ae0325cf8c4b89587222778108c594bdd45a474b5` |
| `memgraph/memgraph:3.5.0` | `sha256:b411deeb2341698f4f7a0d69535c8937c341e924f66962aa3e70acb63c7a5bd1` |

예상: every exact reference resolves to an index/manifest-list whose Linux descriptors include
both architectures. Separately resolve each mutable tag 및 record whether it still points at the
reviewed index; tag drift is reported but cannot change the pinned code authority. 다음을 하지 않는다: proceed
if an exact reviewed digest 없음 longer resolves 또는 lacks either target.

- [ ] **단계 2: Write failing image-authority 테스트**

추가 same-패키지 테스트 for the three 공유 helpers:

```go
func TestDefaultImageIsImmutable(t *testing.T) {
	const want = "redis:7.4-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99"
	if defaultImage != want { t.Fatalf("defaultImage=%q want=%q", defaultImage, want) }
	if !regexp.MustCompile(`^[^[:space:]@]+@sha256:[0-9a-f]{64}$`).MatchString(defaultImage) {
		t.Fatalf("defaultImage is not digest pinned: %q", defaultImage)
	}
}
```

In `graph/neo4j/benchmark_test.go`, introduce constants
`neo4jBenchmarkImage` 및 `memgraphBenchmarkImage`; add
`TestGraphBenchmarkImagesAreImmutable` 함께 the same pattern.

- [ ] **단계 3: 실행 테스트 및 verify RED**

```bash
go test -p 1 -count=1 ./testcontainers/redis ./testcontainers/postgres ./testcontainers/mongodb ./graph/neo4j -run 'Test(DefaultImageIsImmutable|GraphBenchmarkImagesAreImmutable)'
```

예상: FAIL because current helper 및 GraphDB image strings are mutable tags.

- [ ] **단계 4: Pin tag-plus-digest constants 및 keep constructor signatures unchanged**

사용 these exact tag-plus-index-digest constants:

```go
const defaultImage = "redis:7.4-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99"
```

Apply the corresponding exact table entry to PostgreSQL, MongoDB, Neo4j, 및 Memgraph. 다음을 하지 않는다: add
environment overrides 또는 exported image options. 업데이트 GraphDB sub-benchmark display names to
retain the human-readable tag rather than rendering the full digest.

- [ ] **단계 5: 실행 focused 및 공유-helper 테스트**

```bash
gofmt -w testcontainers/redis/*.go testcontainers/postgres/*.go testcontainers/mongodb/*.go graph/neo4j/benchmark_test.go
go test -p 1 -count=1 ./testcontainers/redis ./testcontainers/postgres ./testcontainers/mongodb ./graph/neo4j
```

예상: PASS; 없음 exported API diff.

- [ ] **단계 6: 커밋**

```bash
git add testcontainers/redis testcontainers/postgres testcontainers/mongodb graph/neo4j/benchmark_test.go
git commit -m "Pin provider fixtures to reviewed images" \
  -m "Constraint: benchmark evidence must resolve the same service images on amd64 and arm64" \
  -m "Rejected: ambient image overrides | they make destructive fixture provenance ambiguous" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: focused testcontainer helper and graph benchmark tests"
```

### 작업 2: 추가 the Leader Provider Matrix 및 Correctness Probes

**파일:**
- 생성: `leader/provider_benchmark_test.go`

- [ ] **단계 1: Write failing deterministic concurrency-helper 테스트**

정의 테스트-만 result types 전에 provider construction:

```go
type leaderRoundResult struct {
	member string
	err    error
	won    bool
}

type leaderRoundOptions struct {
	workers       int
	attemptLimit  time.Duration
	roundLimit    time.Duration
}

func TestRunLeaderRoundJoinsEveryWorker(t *testing.T) {
	var active atomic.Int64
	results, err := runLeaderRound(context.Background(),
		leaderRoundOptions{workers: 8, attemptLimit: time.Second, roundLimit: 2 * time.Second},
		func(ctx context.Context, member string) (bool, error) {
			active.Add(1)
			defer active.Add(-1)
			return member == "member-0", nil
		})
	if err != nil { t.Fatal(err) }
	if len(results) != 8 || active.Load() != 0 { t.Fatalf("results=%d active=%d", len(results), active.Load()) }
}

func TestRunLeaderRoundCancelsAndDrainsOnError(t *testing.T) {
	var active atomic.Int64
	_, err := runLeaderRound(context.Background(),
		leaderRoundOptions{workers: 8, attemptLimit: 100 * time.Millisecond, roundLimit: time.Second},
		func(ctx context.Context, member string) (bool, error) {
			active.Add(1)
			defer active.Add(-1)
			if member == "member-0" { return false, errors.New("injected") }
			<-ctx.Done()
			return false, ctx.Err()
		})
	if err == nil { t.Fatal("expected error") }
	if active.Load() != 0 { t.Fatalf("active=%d", active.Load()) }
}
```

The helper must use a start barrier, buffered result channel, first-오류 cancellation, bounded
wait, 및 main-goroutine assertion. Worker goroutines must never call `b.Fatal`.

- [ ] **단계 2: 실행 테스트 및 verify RED**

```bash
go test -count=1 ./leader -run '^TestRunLeaderRound'
```

예상: FAIL because the helper is undefined.

- [ ] **단계 3: 구현 the concurrency helper 및 fixture interface**

```go
type leaderBenchFixture struct {
	name      string
	newElector func(member, group string) (leader.Elector, error)
	observe   func(context.Context, string) (string, error)
	cleanup   func(context.Context, string) error
	close     func(context.Context) error
}
```

Redis/MongoDB/PostgreSQL fixtures must call 기존 `testcontainers/*` helpers. The etcd
fixture copies 만 the current `leader/etcd` platform-digest selection, readiness, 및
client-close order because its fixture is 패키지-private. Every fixture uses a 90-second startup
context, 10-second operation contexts, internally generated 32-character 낮음ercase-hex prefixes,
및 checked namespace/client cleanup. For 공유 `StartServer` fixtures, register the
benchmark-owned namespace/client coordinator 후 the server so LIFO cleanup performs
`namespace -> client -> existing Started container cleanup` exactly once. The copied etcd fixture
owns 및 checks its explicit `namespace -> client -> container` teardown. Every cleanup path uses
an independently bounded `context.WithoutCancel` 및 surfaces joined 오류.
Before timing, each container fixture emits one sanitized `provider_version` 및 pinned
`image_reference` line without endpoint, DSN, credential, container ID, 또는 host-path fields.

- [ ] **단계 4: 추가 exact latency benchmark rows**

추가:

```go
func BenchmarkProviderLeaderLocal(b *testing.B)
func BenchmarkProviderLeaderContainers(b *testing.B)
```

`BenchmarkProviderLeaderLocal` emits exactly
`LocalHarness/CampaignContention/N=8` using a 테스트-local atomic one-winner stub. It measures 만
the 공유 start-barrier/result-drain/join overhead, consumes every result through a 테스트-local
sink, 및 is labeled as a 낮음er-bound concurrency-harness baseline rather than a provider.

The container function gates on `BLUETAPE_LEADER_PROVIDER_BENCH=1` 및 emits:

```text
Redis/CampaignUncontended
Redis/ResignOwned
Redis/CampaignContention/N=8
Redis/LeaderLookup
Redis/ExpiryTakeover
MongoDB/{CampaignUncontended,ResignOwned,CampaignContention/N=8,LeaderLookup,ExpiryTakeover}
PostgreSQL/{CampaignUncontended,ResignOwned,CampaignContention/N=8,LeaderLookup,ExpiryTakeover}
etcd/{CampaignUncontended,ResignOwned,CampaignContention/N=8,LeaderLookup,ExpiryTakeover}
```

사용 a 30-second lease for non-expiry rounds, 5-second attempt bounds, 10-second round bounds,
unique groups per round, `b.ReportAllocs()`, explicit `b.ResetTimer()`, 및 timer-paused
cleanup. `ExpiryTakeover` uses a one-second lease (the common etcd-compatible minimum), polls at
25 milliseconds, 및 has a lease-plus-five-second observation bound. The capture script runs it
separately 함께 `-benchtime=1x -count=3`; it stays in its own raw/report/chart section rather than
the ordinary 100-iteration latency command.

- [ ] **단계 5: 추가 non-ranked correctness probes**

```go
func TestProviderLeaderBenchmarkProbes(t *testing.T)
```

Under the same opt-in gate, run `ActiveHolderCancellation`, `RenewalPersistence`,
`CancellationCleanup`, 및 `StaleOwnerRejected` once per provider. 검증 exact owner
preservation/replacement, bounded goroutine drain, bounded resign, 및 backend absence 또는
replacement proof. 다음을 하지 않는다: emit benchmark timing rows for these probes.

- [ ] **단계 6: 실행 RED/GREEN smoke 및 race checks**

```bash
go test -count=1 ./leader -run '^TestRunLeaderRound'
go test -run '^$' -bench '^BenchmarkProviderLeaderLocal$' -benchtime=1x ./leader
go test -race -count=1 ./leader -run '^TestRunLeaderRound'
```

예상: PASS. 다음을 하지 않는다: start containers in this step.

- [ ] **단계 7: 커밋**

```bash
git add leader/provider_benchmark_test.go
git commit -m "Measure equivalent leader lifecycle paths" \
  -m "Constraint: blocking and renewal-window probes cannot be ranked as ordinary latency rows" \
  -m "Rejected: one-second contention leases | valid expiry takeover can appear as duplicate ownership" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: leader helper tests, local smoke benchmark, and focused race test" \
  -m "Not-tested: opt-in provider containers run during evidence collection"
```

### 작업 3: 추가 the Rate-Limiter Provider Matrix

**파일:**
- 생성: `ratelimit/provider_benchmark_test.go`

- [ ] **단계 1: Write failing round-계약 테스트**

```go
func TestRunRateLimitRoundJoinsWorkersAndCapsAllowed(t *testing.T) {
	var calls atomic.Int64
	results, err := runRateLimitRound(context.Background(), 8, time.Second,
		func(context.Context, string) (ratelimit.Result, error) {
			n := calls.Add(1)
			return ratelimit.Result{Allowed: n <= 3}, nil
		})
	if err != nil { t.Fatal(err) }
	if got := countAllowed(results); got != 3 { t.Fatalf("allowed=%d", got) }
}

func TestRunRateLimitRoundDrainsAfterError(t *testing.T) {
	var active atomic.Int64
	_, err := runRateLimitRound(context.Background(), 8, time.Second,
		func(ctx context.Context, key string) (ratelimit.Result, error) {
			active.Add(1)
			defer active.Add(-1)
			if strings.HasSuffix(key, "-0") { return ratelimit.Result{}, errors.New("injected") }
			<-ctx.Done()
			return ratelimit.Result{}, ctx.Err()
		})
	if err == nil { t.Fatal("expected error") }
	if active.Load() != 0 { t.Fatalf("active=%d", active.Load()) }
}
```

- [ ] **단계 2: 검증 RED, implement helper, verify GREEN**

```bash
go test -count=1 ./ratelimit -run '^TestRunRateLimitRound'
```

Expected 전에 implementation: FAIL. 구현 barrier start, buffered results, first-오류
cancellation, bounded join, 및 main-goroutine assertions; rerun for PASS.

- [ ] **단계 3: 추가 local 및 container benchmark functions**

```go
func BenchmarkProviderRateLimitLocal(b *testing.B)
func BenchmarkProviderRateLimitContainers(b *testing.B)
```

`Local/TokenBucket/{Al낮음Available,Al낮음Rejected}` is a separate algorithm baseline.
Redis 및 PostgreSQL emit `Al낮음Available`, `Al낮음Rejected`, `Al낮음Parallel/N=8`, 및
`Al낮음DistinctKeys/N=8` 함께 identical capacity/refill inputs. Each iteration uses an internal
hex namespace, timer-paused seed/reset, fresh 10-second contexts, 및 checks that same-key al낮음ed
count never exceeds capacity. Gate containers on
`BLUETAPE_RATELIMIT_PROVIDER_BENCH=1`.

Each container fixture calls the 기존 `StartServer`, whose container cleanup is registered
first. Register one benchmark-owned cleanup coordinator afterward; Go's LIFO cleanup order then
proves `namespace deletion -> client close -> existing Started container cleanup`, without a
second `Terminate` call. Give the coordinator an independently bounded `context.WithoutCancel`,
join 및 surface its lifecycle 오류, 및 let the 기존 server cleanup report termination
오류. 추가 a fake coordinator 테스트 that records namespace/client order 및 injects one 오류 at
each stage to prove the later stage still runs 및 the combined 오류 is returned; rely on the
기존 `testcontainers/server` 테스트 for the final registered container cleanup.
Emit the sanitized Redis/PostgreSQL `provider_version` 및 pinned `image_reference` once 전에
timing.

- [ ] **단계 4: 실행 focused 테스트 및 local smoke**

```bash
gofmt -w ratelimit/provider_benchmark_test.go
go test -count=1 ./ratelimit -run '^TestRunRateLimitRound'
go test -run '^$' -bench '^BenchmarkProviderRateLimitLocal$' -benchtime=1x ./ratelimit
go test -race -count=1 ./ratelimit -run '^TestRunRateLimitRound'
```

예상: PASS without Docker.

- [ ] **단계 5: 커밋**

```bash
git add ratelimit/provider_benchmark_test.go
git commit -m "Compare distributed rate-limit paths" \
  -m "Constraint: provider rounding differs, so invariants outrank exact remaining-token equality" \
  -m "Rejected: local token bucket in the provider ranking | it has no network or shared-state semantics" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: round helper tests, local smoke benchmark, and focused race test"
```

### 작업 4: 추가 the Cache Path Matrix

**파일:**
- 생성: `cache/provider_benchmark_test.go`

- [ ] **단계 1: Write failing payload 및 invalidation-observation 테스트**

```go
func TestBenchmarkPayloadSizes(t *testing.T) {
	for _, size := range []int{128, 4 << 10} {
		value := benchmarkCacheValue(size)
		if len(value.Payload) != size { t.Fatalf("size=%d got=%d", size, len(value.Payload)) }
	}
}

func TestObservePeerInvalidationTimesOutAndDrains(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := observePeerInvalidation(ctx, func() bool { return false })
	if !errors.Is(err, context.DeadlineExceeded) { t.Fatalf("error=%v", err) }
}
```

- [ ] **단계 2: 검증 RED 및 implement deterministic helpers**

```bash
go test -count=1 ./cache -run '^Test(BenchmarkPayloadSizes|ObservePeerInvalidation)'
```

Expected 전에 implementation: FAIL. 구현 deterministic payload bytes, internally generated
hex keys, a bounded observation loop 함께 없음 leaked ticker/goroutine, 및 a serializer wrapper
that counts marshal/unmarshal calls.

- [ ] **단계 3: 추가 exact local 및 Redis benchmark sections**

```go
func BenchmarkProviderCacheLocal(b *testing.B)
func BenchmarkProviderCacheRedis(b *testing.B)
```

Emit payload sub-benchmarks for `128B` 및 `4KiB`:

```text
Memory/{GetHit,GetMiss,Set,GetOrLoadHot}/{128B,4KiB}
SerializationBaseline/{Marshal,Unmarshal}/{128B,4KiB}
RedisL2/{GetHit,GetMiss,Set,Delete}/{128B,4KiB}
Tiered/{L1Hit,L2Hit,LoadMiss,WriteThrough}/{128B,4KiB}
NearCachePubSub/{LocalHit,LocalMiss,PublishSet,PublishDelete,PeerInvalidation}/{128B,4KiB}
```

사용 the approved `redisvalue.DefaultConfig()` copied per cache, decoded values in L1, JSON
serialization 만 at L2, 및 없음 batch-put row. 검증 the report later records
`N/A: no public bulk mutation contract`. For `PeerInvalidation`, complete subscription
readiness 전에 timing, measure publish-to-peer-eviction observation, use the two-second bound,
surface subscriber 오류, 및 check every `Close`.

The Redis fixture calls 기존 `StartServer` first, then registers one benchmark-owned cleanup
coordinator. LIFO cleanup yields `namespace deletion -> near-cache/subscriber close -> Redis client
close -> 기존 Started container cleanup`, without double termination. Give the coordinator an
independently bounded `context.WithoutCancel`, join 및 surface every coordinator 오류, 및 rely on
the 기존 server cleanup to report termination 오류. 추가 a fake coordinator 테스트 함께 injected
오류 to prove later stages still execute 및 the combined 오류 is returned.
Emit the sanitized Redis `provider_version` 및 pinned `image_reference` once 전에 timing.

- [ ] **단계 4: 실행 focused 테스트, local smoke, 및 race**

```bash
gofmt -w cache/provider_benchmark_test.go
go test -count=1 ./cache -run '^Test(BenchmarkPayloadSizes|ObservePeerInvalidation)'
go test -run '^$' -bench '^BenchmarkProviderCacheLocal$' -benchtime=1x ./cache
go test -race -count=1 ./cache -run '^Test(BenchmarkPayloadSizes|ObservePeerInvalidation)'
```

예상: PASS without Docker.

- [ ] **단계 5: 커밋**

```bash
git add cache/provider_benchmark_test.go
git commit -m "Separate local and Redis cache costs" \
  -m "Constraint: L1 stores decoded values while serialization belongs only to the L2 boundary" \
  -m "Rejected: one cache winner table | hit, miss, invalidation, and serialization have different semantics" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: cache helper tests, local smoke benchmark, and focused race test"
```

### 작업 5: 추가 the Graph I/O Format Matrix

**파일:**
- 생성: `graph/graphio/provider_benchmark_test.go`

- [ ] **단계 1: Write failing deterministic-shape 및 round-trip 테스트**

```go
func TestBenchmarkGraphShapeIsDeterministic(t *testing.T) {
	first := benchmarkGraphRecords(100, 200, 3)
	second := benchmarkGraphRecords(100, 200, 3)
	if diff := cmp.Diff(first, second); diff != "" { t.Fatal(diff) }
}

func TestBenchmarkFormatsRoundTripEquivalentRecords(t *testing.T) {
	records := benchmarkGraphRecords(100, 200, 3)
	for _, format := range benchmarkGraphFormats() {
		encoded := format.write(t, records)
		decoded := format.read(t, encoded)
		want := normalizeBenchmarkRecords(records)
		got := normalizeBenchmarkRecords(decoded)
		if diff := cmp.Diff(want, got); diff != "" { t.Fatalf("%s (-want +got):\n%s", format.name, diff) }
	}
}
```

사용 만 deterministic safe scalar values so CSV formula escaping does 아님 change semantics.

- [ ] **단계 2: 검증 RED, implement format adapters, verify GREEN**

```bash
go test -count=1 ./graph/graphio -run '^TestBenchmark(GraphShape|Formats)'
```

Expected 전에 implementation: FAIL. 구현 테스트-local adapters for paired CSV, NDJSON, 및
GraphML using 공개 APIs 및 identical logical records.

- [ ] **단계 3: 추가 shape/operation benchmarks**

```go
func BenchmarkGraphIOFormats(b *testing.B)
```

Emit `Small/100V-200E-3P`, `Medium/10000V-20000E-5P`, 및
`WideProperties/1000V-2000E-20P` under each format 함께 `Write`, `Read`,
`RoundTrip`, 및 `RecordConstructionBaseline`. Pause timing for fixture byte generation,
call `b.SetBytes(totalEncodedBytes)` 만 for Write/Read/RoundTrip, report allocations, consume
each timed result through 테스트-local sinks, 및 assert normalized record counts plus representative
IDs 후 timing. `RecordConstructionBaseline` has 없음 MB/s value because it consumes 없음 encoded
bytes. Never subtract the construction baseline to invent parser-만 numbers.

- [ ] **단계 4: 실행 focused 테스트 및 smoke**

```bash
gofmt -w graph/graphio/provider_benchmark_test.go
go test -count=1 ./graph/graphio -run '^TestBenchmark(GraphShape|Formats)'
go test -run '^$' -bench '^BenchmarkGraphIOFormats$' -benchtime=1x ./graph/graphio
```

예상: PASS.

- [ ] **단계 5: 커밋**

```bash
git add graph/graphio/provider_benchmark_test.go
git commit -m "Compare graph formats at equivalent boundaries" \
  -m "Constraint: graphio exposes parser plus record materialization, not a shared graph-store builder" \
  -m "Rejected: subtracting construction baselines | benchmark subtraction does not isolate parser cost" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: deterministic format round trips and graphio smoke benchmark"
```

### 작업 6: 추가 Path-Shaped GraphDB 및 PostgreSQL Baselines

**파일:**
- 생성: `graph/provider_benchmark_test.go`
- Modify: `graph/neo4j/benchmark_test.go`

- [ ] **단계 1: Write failing shape 및 result-equivalence 테스트**

```go
func TestTraversalShapesHaveExpectedReachability(t *testing.T) {
	tests := []struct { shape traversalShape; want int }{
		{longChainShape(16, 64), 17},
		{longChainShape(64, 128), 65},
		{deepWideShape(4, 4), 341},
	}
	for _, tt := range tests {
		if got := len(tt.shape.expectedIDs); got != tt.want { t.Fatalf("%s ids=%d want=%d", tt.shape.name, got, tt.want) }
	}
}

func TestNormalizeTraversalIDsRejectsMissingOrDuplicateRows(t *testing.T) {
	if _, err := normalizeTraversalIDs([]string{"v0", "v0"}, []string{"v0"}); err == nil {
		t.Fatal("expected duplicate-row error")
	}
}
```

- [ ] **단계 2: 검증 RED 및 implement pure shape helpers**

```bash
go test -count=1 ./graph -run '^Test(TraversalShapes|NormalizeTraversalIDs)'
```

Expected 전에 implementation: FAIL. 구현 deterministic vertex/edge seeds, expected ordered
ID sets, duplicate/missing detection, 및 internal 낮음ercase-hex run IDs.

- [ ] **단계 3: 구현 disposable runtime fixtures**

정의:

```go
type traversalRuntime struct {
	name    string
	seed    func(context.Context, traversalShape) error
	query   func(context.Context, traversalShape) ([]string, error)
	cleanup func(context.Context) error
	close   func(context.Context) error
}
```

Neo4j 및 Memgraph use the same parameterized Cypher subset 및 official Testcontainers
modules/request. PostgreSQL uses `testcontainers/postgres`, a benchmark-owned schema, indexed
`vertices(id)` 및 `edges(from_id,to_id)`, 및 a parameterized recursive CTE. No ambient DSN
또는 endpoint is accepted. All runtimes verify expected IDs 전에 `b.ResetTimer()`, use fresh
10-second query contexts, then checked cleanup/client/container shutdown. PostgreSQL uses the
기존 `StartServer` registration 및 a later benchmark-owned schema/client coordinator so LIFO
cleanup terminates the container exactly once. Neo4j/Memgraph own their raw Testcontainers
termination. All three paths use independently bounded `context.WithoutCancel` cleanup 및 surface
joined 오류.
Emit each sanitized database `provider_version` 및 pinned `image_reference` once 전에 timing.

- [ ] **단계 4: 추가 exact traversal benchmark**

```go
func BenchmarkGraphProviderTraversalContainers(b *testing.B)
```

Gate on `BLUETAPE_GRAPH_PROVIDER_BENCH=1`. Emit:

```text
Neo4j/{LongChain/Depth16,LongChain/Depth64,DeepWide/Depth4Fanout4}
Memgraph/{LongChain/Depth16,LongChain/Depth64,DeepWide/Depth4Fanout4}
PostgreSQLRecursiveCTE/{LongChain/Depth16,LongChain/Depth64,DeepWide/Depth4Fanout4}
```

The timed loop contains 만 parameterized traversal 및 result materialization. Seed/index/schema
및 correctness checks remain outside timing. 추가 `b.ReportAllocs()` 및 explicit
`b.ResetTimer()`.

- [ ] **단계 5: 강화 기존 GraphDB cleanup**

교체 ignored cleanup 오류 in `graph/neo4j/benchmark_test.go` 함께 a bounded
`context.WithoutCancel` cleanup that reports deletion, driver close, 및 container termination
failures. 다음을 하지 않는다: move 기존 CRUD rows into the provider selection chart.

- [ ] **단계 6: 실행 pure 테스트 및 compile/smoke gate**

```bash
gofmt -w graph/provider_benchmark_test.go graph/neo4j/benchmark_test.go
go test -count=1 ./graph -run '^Test(TraversalShapes|NormalizeTraversalIDs)'
go test -run '^$' -bench '^BenchmarkGraphProviderTraversalContainers$' -benchtime=1x ./graph
go test -count=1 ./graph/neo4j
```

예상: PASS; container benchmark reports SKIP without its environment variable.

- [ ] **단계 7: 커밋**

```bash
git add graph/provider_benchmark_test.go graph/neo4j/benchmark_test.go
git commit -m "Measure graph databases on path-shaped work" \
  -m "Constraint: GraphDB adoption evidence requires equivalent traversal results and a relational baseline" \
  -m "Rejected: CRUD-only provider ranking | it does not represent long-chain or deep-wide access" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: traversal shape tests, opt-in skip smoke, and graph adapter tests"
```

### 작업 7: 추가 Fail-Safe Benchmark 캡처

**파일:**
- 생성: `scripts/capture-provider-benchmark.sh`
- 생성: `scripts/capture-provider-benchmark_test.sh`

- [ ] **단계 1: Write the failing shell 계약 테스트**

The 테스트 creates a temporary fake `go` executable 및 output directory, then verifies:

```sh
assert_success_writes_atomic_canonical_output
assert_failure_preserves_previous_success
assert_failure_writes_timestamped_failure_output
assert_unknown_family_fails_before_command
assert_secret_pattern_blocks_canonical_output
assert_secret_bearing_failure_is_sanitized_before_retention
assert_command_timestamp_sha_and_exit_status_headers_exist
```

Invoke:

```bash
bash scripts/capture-provider-benchmark_test.sh
```

예상: FAIL because the capture script does 아님 exist.

- [ ] **단계 2: 구현 an al낮음listed POSIX-shell entry point**

Accept exactly:

```text
leader-local leader-containers leader-probes
ratelimit-local ratelimit-containers
cache-local cache-redis
graphio graphdb
```

Map those names to these exact commands; 없음 호출자-supplied command fragment is accepted:

```text
leader-local: go test -timeout=10m -run ^$ -bench ^BenchmarkProviderLeaderLocal$ -benchmem -count=5 ./leader
leader-containers[ordinary]: env BLUETAPE_LEADER_PROVIDER_BENCH=1 go test -timeout=30m -p 1 -run ^$ -bench ^BenchmarkProviderLeaderContainers$/(Redis|MongoDB|PostgreSQL|etcd)/(CampaignUncontended|ResignOwned|CampaignContention|LeaderLookup)$ -benchtime=100x -count=3 -benchmem ./leader
leader-containers[expiry]: env BLUETAPE_LEADER_PROVIDER_BENCH=1 go test -timeout=10m -p 1 -run ^$ -bench ^BenchmarkProviderLeaderContainers$/(Redis|MongoDB|PostgreSQL|etcd)/ExpiryTakeover$ -benchtime=1x -count=3 -benchmem ./leader
leader-probes: env BLUETAPE_LEADER_PROVIDER_BENCH=1 go test -timeout=15m -p 1 -run ^TestProviderLeaderBenchmarkProbes$ ./leader
ratelimit-local: go test -timeout=10m -run ^$ -bench ^BenchmarkProviderRateLimitLocal$ -benchmem -count=5 ./ratelimit
ratelimit-containers: env BLUETAPE_RATELIMIT_PROVIDER_BENCH=1 go test -timeout=30m -p 1 -run ^$ -bench ^BenchmarkProviderRateLimitContainers$ -benchtime=100x -count=3 -benchmem ./ratelimit
cache-local: go test -timeout=10m -run ^$ -bench ^BenchmarkProviderCacheLocal$ -benchmem -count=5 ./cache
cache-redis: env BLUETAPE_CACHE_PROVIDER_BENCH=1 go test -timeout=30m -p 1 -run ^$ -bench ^BenchmarkProviderCacheRedis$ -benchtime=100x -count=3 -benchmem ./cache
graphio: go test -timeout=10m -run ^$ -bench ^BenchmarkGraphIOFormats$ -benchmem -count=5 ./graph/graphio
graphdb: env BLUETAPE_GRAPH_PROVIDER_BENCH=1 go test -timeout=30m -p 1 -run ^$ -bench ^BenchmarkGraphProviderTraversalContainers$ -benchtime=100x -count=3 -benchmem ./graph
```

In the shell implementation, each token above is passed through `set --` 및 `"$@"`; the display
header re-quotes those fixed tokens for readability. The bracketed leader labels identify the two
fixed commands written to the single `leader-containers.txt` artifact 및 are 아님 CLI arguments.

The script uses `set -eu`, a cleanup trap, an output-directory override 만 for its own tracked
artifact destination, 및 a `case` that stores commands as positional arguments rather than
`eval`. It writes command, UTC timestamp, Git SHA, pre-run clean state, combined stdout/stderr,
및 exit status to a mode-`0700` temporary directory outside the repository. It rejects a dirty
pre-run tree except the documented artifact-output directory. On success it scans the private
stream, copies 만 sanitized content to an artifact-local temporary file, rescans it, 및
atomically renames it. On failure it applies the same sanitize-및-rescan pipeline 전에 retaining
a timestamped `*-failed-*.txt`; if prohibited content survives, retain metadata 함께
`redaction_status: blocked` but discard the stream body. 보존 the previous canonical file 및
return the original non-zero status in 모든 failure cases.

For `leader-containers`, execute 및 record two al낮음listed commands in order: ordinary rows 함께
`-benchtime=100x -count=3` while excluding `ExpiryTakeover`, then 만 `ExpiryTakeover` 함께
`-benchtime=1x -count=3`. A failure in either command makes the whole family capture fail.

- [ ] **단계 3: 검증 GREEN 및 shell syntax**

```bash
bash -n scripts/capture-provider-benchmark.sh scripts/capture-provider-benchmark_test.sh
bash scripts/capture-provider-benchmark_test.sh
```

예상: PASS 함께 모든 seven named assertions.

- [ ] **단계 4: 커밋 code 전에 measurements**

```bash
git add scripts/capture-provider-benchmark.sh scripts/capture-provider-benchmark_test.sh
git commit -m "Preserve benchmark evidence without overwriting failures" \
  -m "Constraint: raw output must retain exact commands while excluding credentials and host-local identifiers" \
  -m "Rejected: direct shell redirection | it loses exit provenance and can overwrite the last valid run" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: shell syntax and fake-command capture contract"
```

### 작업 8: 실행 the Matrix Sequentially 및 보존 Raw 증거

**파일:**
- 생성: `docs/research/outputs/issue-560/environment.md`
- 생성: `docs/research/outputs/issue-560/*.txt`

- [ ] **단계 1: 검증 a clean measurement HEAD**

```bash
git status --porcelain
git rev-parse HEAD
go version
docker version
docker info
```

예상: empty Git status; record 만 al낮음listed environment fields. 다음을 하지 않는다: paste full
`docker info` into the artifact.

- [ ] **단계 2: Write the sanitized host/runtime portion of the environment manifest**

기록 UTC/local timestamp 및 timezone, OS/kernel/arch, CPU model, logical CPUs, RAM, Go version,
Git SHA 및 clean pre-run state, Docker client/server/platform, 및 each configured
tag-plus-reviewed-digest. Never record DSNs, endpoints, container IDs, host paths, registry
credentials, proxy configuration, 또는 the complete environment. Provider-reported versions are
added 만 후 successful container captures; do 아님 write placeholder values.

- [ ] **단계 3: 실행 local families**

```bash
scripts/capture-provider-benchmark.sh leader-local
scripts/capture-provider-benchmark.sh ratelimit-local
scripts/capture-provider-benchmark.sh cache-local
scripts/capture-provider-benchmark.sh graphio
```

예상: four canonical outputs 함께 exit status 0 및 five samples per benchmark row.

- [ ] **단계 4: 실행 container families one at a time**

```bash
scripts/capture-provider-benchmark.sh leader-containers
scripts/capture-provider-benchmark.sh leader-probes
scripts/capture-provider-benchmark.sh ratelimit-containers
scripts/capture-provider-benchmark.sh cache-redis
scripts/capture-provider-benchmark.sh graphdb
```

예상: each command exits 0 전에 the next begins. If any command fails, preserve its failure
artifact, stop downstream reporting for that family, repair the benchmark/fixture in its owning
task, commit, return to a clean HEAD, 및 rerun every affected family.

After 모든 five container commands succeed, parse the al낮음listed `provider_version` fields from
their canonical outputs into `environment.md`; fail if any executed provider is missing, duplicated,
또는 inconsistent across families.

- [ ] **단계 5: Validate evidence integrity**

```bash
test "$(find docs/research/outputs/issue-560 -maxdepth 1 -name '*.txt' ! -name '*-failed-*' | wc -l | tr -d ' ')" -eq 9
test "$(rg --no-filename '^exit_status: 0$' docs/research/outputs/issue-560/*.txt | wc -l | tr -d ' ')" -eq 10
if rg -n '^exit_status: [^0]' docs/research/outputs/issue-560/*.txt; then exit 1; fi
if rg -n '(://[^/@[:space:]]+:[^/@[:space:]]+@|(?i)(password|passwd|token|secret|authorization)[=:][^[:space:]]+)' docs/research/outputs/issue-560; then exit 1; fi
```

예상: exactly nine canonical files 및 ten zero exit headers because `leader-containers` has
two fixed command sections; 없음 non-zero canonical header; the secret scan has 없음 matches. Manually
confirm row counts 및 min/median/max availability for 모든 container scenarios.

- [ ] **단계 6: 커밋 raw evidence**

```bash
git add docs/research/outputs/issue-560
git commit -m "Retain current-head provider measurements" \
  -m "Constraint: all container families ran serially from one clean committed HEAD" \
  -m "Rejected: partial successful-family publication | issue 560 requires every implemented multi-provider family" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: nine capture commands, leader probes, artifact exit headers, and secret scan"
```

### 작업 9: 생성 Charts, Report Decisions, 및 Link Both READMEs

**파일:**
- 생성: `docs/research/2026-07-20-issue-560-provider-benchmark-matrix.md`
- 생성: `docs/images/readme-charts/generate-provider-benchmark-summaries.mjs`
- 생성: `docs/images/readme-charts/provider-benchmark-*-summary.vl.json`
- 생성: `docs/images/readme-charts/provider-benchmark-*-summary.svg`
- 생성: `docs/images/readme-charts/provider-benchmark-*-summary.png`
- Modify: `README.md`
- Modify: `README.ko.md`

- [ ] **단계 1: Load the chart workf낮음 전에 visual work**

Read 및 fol낮음 `bluetape-diagram/SKILL.md`. Reuse the current repository chart style 및 retain
generator, Vega-Lite JSON, SVG, 및 PNG.

- [ ] **단계 2: Write a failing strict raw-output parser**

The Node generator must reject unknown, missing, duplicate, non-finite, 또는 non-zero-exit rows.
Before implementing parsing, add a self-테스트 mode:

```bash
node docs/images/readme-charts/generate-provider-benchmark-summaries.mjs --self-test
```

Expected 전에 parser implementation: FAIL. The GREEN self-테스트 covers one valid fixture 및
one valid two-command leader fixture plus unknown/missing/duplicate/non-finite/오류-exit fixtures.
The parser treats command sections independently, requires the exact expected section count, 및
never merges ordinary leader latency samples 함께 `ExpiryTakeover` samples.

- [ ] **단계 3: 생성 five family chart sets**

Produce leader, rate limiter, cache, graph I/O, 및 GraphDB chart sets. Chart 만 equivalent
latency rows, keep `ExpiryTakeover` separate, use min/median/max 또는 오류 bars for container
samples, label units, provide sufficient contrast 및 direct provider labels/patterns, 및 never
depend on color alone. 유지 allocation/density details in tables rather than mixing units. 사용
repository-relative data paths 및 portable system font stacks; do 아님 embed 사용자-home font paths.
The generator writes Vega-Lite JSON 및 SVG directly, invokes `rsvg-convert` 함께 fixed dimensions
to produce each PNG, 및 fails clearly if rasterization is unavailable.

- [ ] **단계 4: Visually inspect rendered assets**

Open every PNG 및 SVG at document scale. 검증 labels, clipping, units, contrast, ordering,
pattern/label distinction, 및 consistency 함께 the adjacent raw-derived table. Regenerate until
모든 checks pass.

- [ ] **단계 5: Write the 영문 report**

For each family include workload/semantic boundary, exact command/raw link, metric direction,
table, chart 함께 descriptive alt text, measured evidence, use-case selection guidance,
caveats/아님-proven, 및 priority/instrumentation fol낮음-up decision. Explicitly record:

- local rows are 낮음er-bound/API baselines, 아님 distributed winners;
- active-holder/renewal leader probes are 아님 ranked;
- cache batch put is `N/A: no public bulk mutation contract`;
- graph-store construction is `N/A: no shared construction API`;
- RESP3 tracking remains a spike, 아님 the production near-cache row;
- one local Docker snapshot does 아님 establish production SLO, cloud cost, WAN behavior, 또는 a
  universal winner.

If evidence suggests a new priority 또는 instrumentation gap, link an 기존 issue 또는 include a
fol낮음-up issue draft 만. 다음을 하지 않는다: create the GitHub issue without separate approval.

- [ ] **단계 6: 추가 synchronized root README links**

`README.md` 및 `README.ko.md` must both link the report 및 show:

```bash
scripts/capture-provider-benchmark.sh leader-local
```

The accepted family argument is exactly one of `leader-local`, `leader-containers`,
`ratelimit-local`, `ratelimit-containers`, `cache-local`, `cache-redis`, `graphio`, `graphdb`,
또는 `leader-probes`.

Explain in each locale that results are short local snapshots 및 should 아님 be copied as
production rankings. 다음을 하지 않는다: duplicate result numbers in README.

- [ ] **단계 7: Validate 및 commit docs/charts**

```bash
node docs/images/readme-charts/generate-provider-benchmark-summaries.mjs --self-test
node docs/images/readme-charts/generate-provider-benchmark-summaries.mjs
git diff --check
rg -n 'provider benchmark|provider 벤치마크' README.md README.ko.md
```

예상: parser self-테스트 pass, regeneration is deterministic, both README files contain the
same report/capture surface, 및 diff check passes.

커밋:

```bash
git add docs/research/2026-07-20-issue-560-provider-benchmark-matrix.md docs/images/readme-charts README.md README.ko.md
git commit -m "Explain provider choices from bounded evidence" \
  -m "Constraint: tables and charts may compare only equivalent scenario semantics" \
  -m "Rejected: universal provider ranking | one local snapshot cannot prove production fitness" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: strict parser self-tests, deterministic chart generation, visual QA, README parity, and diff check"
```

### 작업 10: 기록 the Type A Lesson 및 실행 Targeted Gates

**파일:**
- 생성: `docs/lessons/2026-07-20-issue-560-provider-benchmark-matrix.md`

- [ ] **단계 1: Write the reusable lesson**

기록 the reusable decisions: semantic equivalence 전에 timing, deadline/sleep probes outside
rankings, disposable fixture provenance, deterministic concurrency join, parser/materialization
boundary, atomic raw capture, 및 selection guidance instead of universal winners. Include concrete
file/command evidence 및 explain when the pattern is `N/A`.

- [ ] **단계 2: 실행 targeted 패키지 및 script validation**

```bash
go test -p 1 -count=1 ./testcontainers/redis ./testcontainers/postgres ./testcontainers/mongodb
go test -p 1 -count=1 ./leader ./ratelimit ./cache ./graph ./graph/graphio ./graph/neo4j
go test -p 1 -race -count=1 ./leader ./ratelimit ./cache ./graph ./graph/graphio
bash -n scripts/capture-provider-benchmark.sh scripts/capture-provider-benchmark_test.sh
bash scripts/capture-provider-benchmark_test.sh
node docs/images/readme-charts/generate-provider-benchmark-summaries.mjs --self-test
```

예상: 모든 PASS; container benchmarks remain skipped unless explicitly opted in.

- [ ] **단계 3: 커밋 lesson**

```bash
git add docs/lessons/2026-07-20-issue-560-provider-benchmark-matrix.md
git commit -m "Preserve provider benchmark lessons" \
  -m "Constraint: future provider additions need the same semantic and evidence boundaries" \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Tested: targeted package, race, capture-script, and chart-parser checks"
```

### 작업 11: 실행 Full 검증, Reviews, 및 준비 the PR

**파일:**
- Modify 만 files required by verified P0/P1 review repairs.

- [ ] **단계 1: 실행 full repository gates**

```bash
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
git diff --check
```

예상: every command exits 0. Testcontainers-backed work stays serial. If the known
`leader/sql TestLeaseStatements/concurrent-single-winner` symptom recurs, capture the failure,
rerun the exact subtest repeatedly, 및 distinguish valid lease expiry from a new regression
전에 changing production code.

- [ ] **단계 2: Re-run artifact reproducibility 및 보안 checks**

```bash
node docs/images/readme-charts/generate-provider-benchmark-summaries.mjs
git diff --exit-code -- docs/images/readme-charts
if rg -n '(://[^/@[:space:]]+:[^/@[:space:]]+@|(?i)(password|passwd|token|secret|authorization)[=:][^[:space:]]+)' docs/research/outputs/issue-560; then exit 1; fi
git status --short
```

예상: deterministic charts, 없음 secret matches, 및 clean worktree.

- [ ] **단계 3: 실행 단계 6-R 및 단계 7-R reviews**

실행 six independent 성능, 안정성, 보안, Operator/Ops, Developer/API, 및
User/호출자 lanes plus main integration. 리뷰 the implemented diff 및 then exact PR head.
Resolve every P0/P1; fix, justify, 또는 file every P2/P3. Heavy container commands remain in the
main session 및 sequential.

- [ ] **단계 4: Render the DoD**

Report exact counts/evidence for scope, targeted/full 테스트, race, lint/vet/tidy/fmt, 모든 nine
capture commands, five family tables/charts, environment/redaction, README parity, lesson, 및
7-Tier results. Required final state: `Blocked=0`, `P0=0`, `P1=0`.

- [ ] **단계 5: Push 및 create the approved PR**

Push `perf/issue-560-provider-benchmark-matrix` 및 create:

```text
bluetape4k/bluetape-go
develop <- perf/issue-560-provider-benchmark-matrix
```

The PR body must include `Closes #560`, benchmark snapshot caveats, exact validation evidence,
review provenance, 및 central `## DoD Status`. PR creation is already in the approved delivery
scope. 다음을 하지 않는다: enable auto-merge 및 do 아님 merge.

- [ ] **단계 6: Stop at merge-ready**

After CI, current reviews/threads, exact-head verification, 및 applicable human-review artifacts
pass, report the exact PR/head as merge-ready 및 wait for a fresh explicit 사용자 approval.
