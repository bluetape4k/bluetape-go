# Issue #560 Provider Benchmark Matrix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add reproducible, semantically bounded benchmark matrices and retained evidence for every currently implemented multi-provider family in issue #560.

**Architecture:** Keep each benchmark harness in its family root as external test code, reuse existing Testcontainers helpers, and add no production API or runtime dependency. A fail-safe capture script records exact commands and sanitized raw output; one English report, bilingual README links, and reproducible chart sources aggregate only equivalent scenarios.

**Tech Stack:** Go 1.26.3 benchmarks/tests, existing Redis/MongoDB/PostgreSQL/etcd/Neo4j/Testcontainers dependencies, POSIX shell, Node.js chart generators, Vega-Lite JSON, SVG/PNG, Markdown.

---

## File Map

Create:

- `leader/provider_benchmark_test.go` — leader latency rows, deterministic concurrent rounds, and correctness probes.
- `ratelimit/provider_benchmark_test.go` — Redis/PostgreSQL rate-limiter rows and local baseline.
- `cache/provider_benchmark_test.go` — memory, Redis L2, tiered, near-cache, and serializer sections.
- `graph/graphio/provider_benchmark_test.go` — CSV/NDJSON/GraphML shapes and construction baseline.
- `graph/provider_benchmark_test.go` — Neo4j/Memgraph traversal and PostgreSQL recursive-CTE baseline.
- `scripts/capture-provider-benchmark.sh` — allowlisted family execution and atomic raw-output capture.
- `scripts/capture-provider-benchmark_test.sh` — success, failure, redaction, and atomic-replacement contract.
- `docs/research/outputs/issue-560/environment.md` — sanitized environment manifest.
- `docs/research/outputs/issue-560/*.txt` — exact successful benchmark/probe outputs.
- `docs/research/2026-07-20-issue-560-provider-benchmark-matrix.md` — final report.
- `docs/images/readme-charts/generate-provider-benchmark-summaries.mjs` — raw-output parser and chart renderer.
- `docs/images/readme-charts/provider-benchmark-*-summary.vl.json` — chart data/spec sources.
- `docs/images/readme-charts/provider-benchmark-*-summary.svg` — reviewable vector charts.
- `docs/images/readme-charts/provider-benchmark-*-summary.png` — Markdown-compatible raster charts.
- `docs/lessons/2026-07-20-issue-560-provider-benchmark-matrix.md` — Type A reusable lesson.

Modify:

- `testcontainers/redis/redis.go`, `redis_test.go` — immutable Redis image authority.
- `testcontainers/postgres/postgres.go`, `postgres_test.go` — immutable PostgreSQL image authority.
- `testcontainers/mongodb/mongodb.go`, `mongodb_test.go` — immutable MongoDB image authority.
- `graph/neo4j/benchmark_test.go` — checked bounded cleanup and immutable Neo4j/Memgraph images.
- `README.md`, `README.ko.md` — matching report link, snapshot caveat, and capture command.

Do not modify exported production contracts in `leader`, `ratelimit`, `cache`,
`graph`, or `graph/graphio`. Do not add dependencies or benchmark live/cloud/public
services.

## Spec Coverage Matrix

| Spec requirement | Plan proof |
|---|---|
| Five implemented multi-provider families | Tasks 2-6 |
| Equivalent scenarios and local/network separation | Tasks 2-6 family-specific sub-benchmark grammar |
| Fixture reuse and immutable provenance | Task 1 and Tasks 2-6 fixture construction |
| Bounded contexts, worker joins, checked cleanup | Tasks 2-6 focused contract tests |
| Leader acquire/resign/contention/expiry/cancellation/stale-owner coverage | Task 2 latency rows and probes |
| Cache L1/L2/tiered/near-cache plus serialization | Task 4 |
| Graph I/O parser/materialization and construction boundary | Task 5 |
| Path-shaped GraphDB plus PostgreSQL baseline | Task 6 |
| Exact commands, redaction, failure-safe raw output | Task 7 |
| Fresh current-HEAD evidence and environment | Task 8 |
| Tables, charts, selection analysis, caveats | Task 9 |
| Bilingual discoverability | Task 9 |
| No public API/dependency/default change | Tasks 1-10 diff and module checks |
| Full verification and P0/P1=0 | Tasks 10-11 |

### Task 1: Pin Container Provenance Without Changing Helper APIs

**Files:**
- Modify: `testcontainers/redis/redis.go`
- Modify: `testcontainers/redis/redis_test.go`
- Modify: `testcontainers/postgres/postgres.go`
- Modify: `testcontainers/postgres/postgres_test.go`
- Modify: `testcontainers/mongodb/mongodb.go`
- Modify: `testcontainers/mongodb/mongodb_test.go`
- Modify: `graph/neo4j/benchmark_test.go`

- [ ] **Step 1: Resolve reviewed multi-architecture image digests**

Use the reviewed multi-architecture index digests below. Before editing code, inspect each index
and verify that it still contains both `linux/amd64` and `linux/arm64` descriptors:

```bash
docker manifest inspect --verbose redis:7.4-alpine
docker manifest inspect --verbose postgres:16-alpine
docker manifest inspect --verbose mongo:7.0
docker manifest inspect --verbose neo4j:5.26.0
docker manifest inspect --verbose memgraph/memgraph:3.5.0
```

| Image tag | Reviewed index digest |
|---|---|
| `redis:7.4-alpine` | `sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99` |
| `postgres:16-alpine` | `sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777` |
| `mongo:7.0` | `sha256:340c1c56fb10e95cf79ff547f8664b96bc6ead9909bc355238cbf865a9695a6f` |
| `neo4j:5.26.0` | `sha256:5a015e53de1895e7eee1574ae0325cf8c4b89587222778108c594bdd45a474b5` |
| `memgraph/memgraph:3.5.0` | `sha256:b411deeb2341698f4f7a0d69535c8937c341e924f66962aa3e70acb63c7a5bd1` |

Expected: every command reports an index/manifest-list whose Linux descriptors include both
architectures. Do not proceed if the reviewed digest no longer resolves or lacks either target.

- [ ] **Step 2: Write failing image-authority tests**

Add same-package tests for the three shared helpers:

```go
func TestDefaultImageIsImmutable(t *testing.T) {
	if !regexp.MustCompile(`^[^[:space:]@]+@sha256:[0-9a-f]{64}$`).MatchString(defaultImage) {
		t.Fatalf("defaultImage is not digest pinned: %q", defaultImage)
	}
}
```

In `graph/neo4j/benchmark_test.go`, introduce constants
`neo4jBenchmarkImage` and `memgraphBenchmarkImage`; add
`TestGraphBenchmarkImagesAreImmutable` with the same pattern.

- [ ] **Step 3: Run tests and verify RED**

```bash
go test -count=1 ./testcontainers/redis ./testcontainers/postgres ./testcontainers/mongodb ./graph/neo4j -run 'Test(DefaultImageIsImmutable|GraphBenchmarkImagesAreImmutable)'
```

Expected: FAIL because current helper and GraphDB image strings are mutable tags.

- [ ] **Step 4: Pin tag-plus-digest constants and keep constructor signatures unchanged**

Use these exact tag-plus-index-digest constants:

```go
const defaultImage = "redis:7.4-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99"
```

Apply the corresponding exact table entry to PostgreSQL, MongoDB, Neo4j, and Memgraph. Do not add
environment overrides or exported image options. Update GraphDB sub-benchmark display names to
retain the human-readable tag rather than rendering the full digest.

- [ ] **Step 5: Run focused and shared-helper tests**

```bash
gofmt -w testcontainers/redis/*.go testcontainers/postgres/*.go testcontainers/mongodb/*.go graph/neo4j/benchmark_test.go
go test -count=1 ./testcontainers/redis ./testcontainers/postgres ./testcontainers/mongodb ./graph/neo4j
```

Expected: PASS; no exported API diff.

- [ ] **Step 6: Commit**

```bash
git add testcontainers/redis testcontainers/postgres testcontainers/mongodb graph/neo4j/benchmark_test.go
git commit -m "Pin provider fixtures to reviewed images" \
  -m "Constraint: benchmark evidence must resolve the same service images on amd64 and arm64" \
  -m "Rejected: ambient image overrides | they make destructive fixture provenance ambiguous" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: focused testcontainer helper and graph benchmark tests"
```

### Task 2: Add the Leader Provider Matrix and Correctness Probes

**Files:**
- Create: `leader/provider_benchmark_test.go`

- [ ] **Step 1: Write failing deterministic concurrency-helper tests**

Define test-only result types before provider construction:

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
	_, err := runLeaderRound(context.Background(),
		leaderRoundOptions{workers: 8, attemptLimit: 100 * time.Millisecond, roundLimit: time.Second},
		func(context.Context, string) (bool, error) { return false, errors.New("injected") })
	if err == nil { t.Fatal("expected error") }
}
```

The helper must use a start barrier, buffered result channel, first-error cancellation, bounded
wait, and main-goroutine assertion. Worker goroutines must never call `b.Fatal`.

- [ ] **Step 2: Run tests and verify RED**

```bash
go test -count=1 ./leader -run '^TestRunLeaderRound'
```

Expected: FAIL because the helper is undefined.

- [ ] **Step 3: Implement the concurrency helper and fixture interface**

```go
type leaderBenchFixture struct {
	name      string
	newElector func(member, group string) (leader.Elector, error)
	observe   func(context.Context, string) (string, error)
	cleanup   func(context.Context, string) error
	close     func(context.Context) error
}
```

Redis/MongoDB/PostgreSQL fixtures must call existing `testcontainers/*` helpers. The etcd
fixture copies only the current `leader/etcd` platform-digest selection, readiness, and
client-close order because its fixture is package-private. Every fixture uses a 90-second startup
context, 10-second operation contexts, internally generated 32-character lowercase-hex prefixes,
and checked namespace/client cleanup.

- [ ] **Step 4: Add exact latency benchmark rows**

Add:

```go
func BenchmarkProviderLeaderLocal(b *testing.B)
func BenchmarkProviderLeaderContainers(b *testing.B)
```

The container function gates on `BLUETAPE_LEADER_PROVIDER_BENCH=1` and emits:

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

Use a 30-second lease for non-expiry rounds, 5-second attempt bounds, 10-second round bounds,
unique groups per round, `b.ReportAllocs()`, explicit `b.ResetTimer()`, and timer-paused
cleanup. `ExpiryTakeover` uses the same intentionally shorter lease for every provider and stays
in its own report/chart section.

- [ ] **Step 5: Add non-ranked correctness probes**

```go
func TestProviderLeaderBenchmarkProbes(t *testing.T)
```

Under the same opt-in gate, run `ActiveHolderCancellation`, `RenewalPersistence`,
`CancellationCleanup`, and `StaleOwnerRejected` once per provider. Assert exact owner
preservation/replacement, bounded goroutine drain, bounded resign, and backend absence or
replacement proof. Do not emit benchmark timing rows for these probes.

- [ ] **Step 6: Run RED/GREEN smoke and race checks**

```bash
go test -count=1 ./leader -run '^TestRunLeaderRound'
go test -run '^$' -bench '^BenchmarkProviderLeaderLocal$' -benchtime=1x ./leader
go test -race -count=1 ./leader -run '^TestRunLeaderRound'
```

Expected: PASS. Do not start containers in this step.

- [ ] **Step 7: Commit**

```bash
git add leader/provider_benchmark_test.go
git commit -m "Measure equivalent leader lifecycle paths" \
  -m "Constraint: blocking and renewal-window probes cannot be ranked as ordinary latency rows" \
  -m "Rejected: one-second contention leases | valid expiry takeover can appear as duplicate ownership" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: leader helper tests, local smoke benchmark, and focused race test" \
  -m "Not-tested: opt-in provider containers run during evidence collection"
```

### Task 3: Add the Rate-Limiter Provider Matrix

**Files:**
- Create: `ratelimit/provider_benchmark_test.go`

- [ ] **Step 1: Write failing round-contract tests**

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
	_, err := runRateLimitRound(context.Background(), 8, time.Second,
		func(context.Context, string) (ratelimit.Result, error) { return ratelimit.Result{}, errors.New("injected") })
	if err == nil { t.Fatal("expected error") }
}
```

- [ ] **Step 2: Verify RED, implement helper, verify GREEN**

```bash
go test -count=1 ./ratelimit -run '^TestRunRateLimitRound'
```

Expected before implementation: FAIL. Implement barrier start, buffered results, first-error
cancellation, bounded join, and main-goroutine assertions; rerun for PASS.

- [ ] **Step 3: Add local and container benchmark functions**

```go
func BenchmarkProviderRateLimitLocal(b *testing.B)
func BenchmarkProviderRateLimitContainers(b *testing.B)
```

`Local/TokenBucket/{AllowAvailable,AllowRejected}` is a separate algorithm baseline.
Redis and PostgreSQL emit `AllowAvailable`, `AllowRejected`, `AllowParallel/N=8`, and
`AllowDistinctKeys/N=8` with identical capacity/refill inputs. Each iteration uses an internal
hex namespace, timer-paused seed/reset, fresh 10-second contexts, and checks that same-key allowed
count never exceeds capacity. Gate containers on
`BLUETAPE_RATELIMIT_PROVIDER_BENCH=1`.

- [ ] **Step 4: Run focused tests and local smoke**

```bash
gofmt -w ratelimit/provider_benchmark_test.go
go test -count=1 ./ratelimit -run '^TestRunRateLimitRound'
go test -run '^$' -bench '^BenchmarkProviderRateLimitLocal$' -benchtime=1x ./ratelimit
go test -race -count=1 ./ratelimit -run '^TestRunRateLimitRound'
```

Expected: PASS without Docker.

- [ ] **Step 5: Commit**

```bash
git add ratelimit/provider_benchmark_test.go
git commit -m "Compare distributed rate-limit paths" \
  -m "Constraint: provider rounding differs, so invariants outrank exact remaining-token equality" \
  -m "Rejected: local token bucket in the provider ranking | it has no network or shared-state semantics" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: round helper tests, local smoke benchmark, and focused race test"
```

### Task 4: Add the Cache Path Matrix

**Files:**
- Create: `cache/provider_benchmark_test.go`

- [ ] **Step 1: Write failing payload and invalidation-observation tests**

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

- [ ] **Step 2: Verify RED and implement deterministic helpers**

```bash
go test -count=1 ./cache -run '^Test(BenchmarkPayloadSizes|ObservePeerInvalidation)'
```

Expected before implementation: FAIL. Implement deterministic payload bytes, internally generated
hex keys, a bounded observation loop with no leaked ticker/goroutine, and a serializer wrapper
that counts marshal/unmarshal calls.

- [ ] **Step 3: Add exact local and Redis benchmark sections**

```go
func BenchmarkProviderCacheLocal(b *testing.B)
func BenchmarkProviderCacheRedis(b *testing.B)
```

Emit payload sub-benchmarks for `128B` and `4KiB`:

```text
Memory/{GetHit,GetMiss,Set,GetOrLoadHot}/{128B,4KiB}
SerializationBaseline/{Marshal,Unmarshal}/{128B,4KiB}
RedisL2/{GetHit,GetMiss,Set,Delete}/{128B,4KiB}
Tiered/{L1Hit,L2Hit,LoadMiss,WriteThrough}/{128B,4KiB}
NearCachePubSub/{LocalHit,LocalMiss,PublishSet,PublishDelete,PeerInvalidation}/{128B,4KiB}
```

Use the approved `redisvalue.DefaultConfig()` copied per cache, decoded values in L1, JSON
serialization only at L2, and no batch-put row. Assert the report later records
`N/A: no public bulk mutation contract`. For `PeerInvalidation`, complete subscription
readiness before timing, measure publish-to-peer-eviction observation, use the two-second bound,
surface subscriber errors, and check every `Close`.

- [ ] **Step 4: Run focused tests, local smoke, and race**

```bash
gofmt -w cache/provider_benchmark_test.go
go test -count=1 ./cache -run '^Test(BenchmarkPayloadSizes|ObservePeerInvalidation)'
go test -run '^$' -bench '^BenchmarkProviderCacheLocal$' -benchtime=1x ./cache
go test -race -count=1 ./cache -run '^Test(BenchmarkPayloadSizes|ObservePeerInvalidation)'
```

Expected: PASS without Docker.

- [ ] **Step 5: Commit**

```bash
git add cache/provider_benchmark_test.go
git commit -m "Separate local and Redis cache costs" \
  -m "Constraint: L1 stores decoded values while serialization belongs only to the L2 boundary" \
  -m "Rejected: one cache winner table | hit, miss, invalidation, and serialization have different semantics" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: cache helper tests, local smoke benchmark, and focused race test"
```

### Task 5: Add the Graph I/O Format Matrix

**Files:**
- Create: `graph/graphio/provider_benchmark_test.go`

- [ ] **Step 1: Write failing deterministic-shape and round-trip tests**

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
		if diff := cmp.Diff(records, decoded); diff != "" { t.Fatalf("%s (-want +got):\n%s", format.name, diff) }
	}
}
```

Use only deterministic safe scalar values so CSV formula escaping does not change semantics.

- [ ] **Step 2: Verify RED, implement format adapters, verify GREEN**

```bash
go test -count=1 ./graph/graphio -run '^TestBenchmark(GraphShape|Formats)'
```

Expected before implementation: FAIL. Implement test-local adapters for paired CSV, NDJSON, and
GraphML using public APIs and identical logical records.

- [ ] **Step 3: Add shape/operation benchmarks**

```go
func BenchmarkGraphIOFormats(b *testing.B)
```

Emit `Small/100V-200E-3P`, `Medium/10000V-20000E-5P`, and
`WideProperties/1000V-2000E-20P` under each format with `Write`, `Read`,
`RoundTrip`, and `RecordConstructionBaseline`. Pause timing for fixture byte generation,
call `b.SetBytes(totalEncodedBytes)`, report allocations, and assert record counts plus
representative IDs. Never subtract the construction baseline to invent parser-only numbers.

- [ ] **Step 4: Run focused tests and smoke**

```bash
gofmt -w graph/graphio/provider_benchmark_test.go
go test -count=1 ./graph/graphio -run '^TestBenchmark(GraphShape|Formats)'
go test -run '^$' -bench '^BenchmarkGraphIOFormats$' -benchtime=1x ./graph/graphio
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add graph/graphio/provider_benchmark_test.go
git commit -m "Compare graph formats at equivalent boundaries" \
  -m "Constraint: graphio exposes parser plus record materialization, not a shared graph-store builder" \
  -m "Rejected: subtracting construction baselines | benchmark subtraction does not isolate parser cost" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: deterministic format round trips and graphio smoke benchmark"
```

### Task 6: Add Path-Shaped GraphDB and PostgreSQL Baselines

**Files:**
- Create: `graph/provider_benchmark_test.go`
- Modify: `graph/neo4j/benchmark_test.go`

- [ ] **Step 1: Write failing shape and result-equivalence tests**

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

- [ ] **Step 2: Verify RED and implement pure shape helpers**

```bash
go test -count=1 ./graph -run '^Test(TraversalShapes|NormalizeTraversalIDs)'
```

Expected before implementation: FAIL. Implement deterministic vertex/edge seeds, expected ordered
ID sets, duplicate/missing detection, and internal lowercase-hex run IDs.

- [ ] **Step 3: Implement disposable runtime fixtures**

Define:

```go
type traversalRuntime struct {
	name    string
	seed    func(context.Context, traversalShape) error
	query   func(context.Context, traversalShape) ([]string, error)
	cleanup func(context.Context) error
	close   func(context.Context) error
}
```

Neo4j and Memgraph use the same parameterized Cypher subset and official Testcontainers
modules/request. PostgreSQL uses `testcontainers/postgres`, a benchmark-owned schema, indexed
`vertices(id)` and `edges(from_id,to_id)`, and a parameterized recursive CTE. No ambient DSN
or endpoint is accepted. All runtimes verify expected IDs before `b.ResetTimer()`, use fresh
10-second query contexts, then checked cleanup/client/container shutdown.

- [ ] **Step 4: Add exact traversal benchmark**

```go
func BenchmarkGraphProviderTraversalContainers(b *testing.B)
```

Gate on `BLUETAPE_GRAPH_PROVIDER_BENCH=1`. Emit:

```text
Neo4j/{LongChain/Depth16,LongChain/Depth64,DeepWide/Depth4Fanout4}
Memgraph/{LongChain/Depth16,LongChain/Depth64,DeepWide/Depth4Fanout4}
PostgreSQLRecursiveCTE/{LongChain/Depth16,LongChain/Depth64,DeepWide/Depth4Fanout4}
```

The timed loop contains only parameterized traversal and result materialization. Seed/index/schema
and correctness checks remain outside timing. Add `b.ReportAllocs()` and explicit
`b.ResetTimer()`.

- [ ] **Step 5: Harden existing GraphDB cleanup**

Replace ignored cleanup errors in `graph/neo4j/benchmark_test.go` with a bounded
`context.WithoutCancel` cleanup that reports deletion, driver close, and container termination
failures. Do not move existing CRUD rows into the provider selection chart.

- [ ] **Step 6: Run pure tests and compile/smoke gate**

```bash
gofmt -w graph/provider_benchmark_test.go graph/neo4j/benchmark_test.go
go test -count=1 ./graph -run '^Test(TraversalShapes|NormalizeTraversalIDs)'
go test -run '^$' -bench '^BenchmarkGraphProviderTraversalContainers$' -benchtime=1x ./graph
go test -count=1 ./graph/neo4j
```

Expected: PASS; container benchmark reports SKIP without its environment variable.

- [ ] **Step 7: Commit**

```bash
git add graph/provider_benchmark_test.go graph/neo4j/benchmark_test.go
git commit -m "Measure graph databases on path-shaped work" \
  -m "Constraint: GraphDB adoption evidence requires equivalent traversal results and a relational baseline" \
  -m "Rejected: CRUD-only provider ranking | it does not represent long-chain or deep-wide access" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: traversal shape tests, opt-in skip smoke, and graph adapter tests"
```

### Task 7: Add Fail-Safe Benchmark Capture

**Files:**
- Create: `scripts/capture-provider-benchmark.sh`
- Create: `scripts/capture-provider-benchmark_test.sh`

- [ ] **Step 1: Write the failing shell contract test**

The test creates a temporary fake `go` executable and output directory, then verifies:

```sh
assert_success_writes_atomic_canonical_output
assert_failure_preserves_previous_success
assert_failure_writes_timestamped_failure_output
assert_unknown_family_fails_before_command
assert_secret_pattern_blocks_canonical_output
assert_command_timestamp_sha_and_exit_status_headers_exist
```

Invoke:

```bash
bash scripts/capture-provider-benchmark_test.sh
```

Expected: FAIL because the capture script does not exist.

- [ ] **Step 2: Implement an allowlisted POSIX-shell entry point**

Accept exactly:

```text
leader-local leader-containers leader-probes
ratelimit-local ratelimit-containers
cache-local cache-redis
graphio graphdb
```

The script uses `set -eu`, a cleanup trap, an output-directory override only for its own tracked
artifact destination, and a `case` that stores commands as positional arguments rather than
`eval`. It writes command, UTC timestamp, Git SHA, pre-run clean state, combined stdout/stderr,
and exit status to a temporary file. It rejects a dirty pre-run tree except the documented
artifact-output directory. On success it runs the allowlisted secret-pattern scanner and atomically
renames the file. On failure it retains a timestamped `*-failed-*.txt`, preserves the previous
canonical file, and exits non-zero.

- [ ] **Step 3: Verify GREEN and shell syntax**

```bash
bash -n scripts/capture-provider-benchmark.sh scripts/capture-provider-benchmark_test.sh
bash scripts/capture-provider-benchmark_test.sh
```

Expected: PASS with all six named assertions.

- [ ] **Step 4: Commit code before measurements**

```bash
git add scripts/capture-provider-benchmark.sh scripts/capture-provider-benchmark_test.sh
git commit -m "Preserve benchmark evidence without overwriting failures" \
  -m "Constraint: raw output must retain exact commands while excluding credentials and host-local identifiers" \
  -m "Rejected: direct shell redirection | it loses exit provenance and can overwrite the last valid run" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: shell syntax and fake-command capture contract"
```

### Task 8: Execute the Matrix Sequentially and Preserve Raw Evidence

**Files:**
- Create: `docs/research/outputs/issue-560/environment.md`
- Create: `docs/research/outputs/issue-560/*.txt`

- [ ] **Step 1: Verify a clean measurement HEAD**

```bash
git status --porcelain
git rev-parse HEAD
go version
docker version
docker info
```

Expected: empty Git status; record only allowlisted environment fields. Do not paste full
`docker info` into the artifact.

- [ ] **Step 2: Write the sanitized environment manifest**

Record UTC/local timestamp and timezone, OS/kernel/arch, CPU model, logical CPUs, RAM, Go version,
Git SHA and clean pre-run state, Docker client/server/platform, each configured tag/resolved digest,
and provider-reported version. Never record DSNs, endpoints, container IDs, host paths, registry
credentials, proxy configuration, or the complete environment.

- [ ] **Step 3: Run local families**

```bash
scripts/capture-provider-benchmark.sh leader-local
scripts/capture-provider-benchmark.sh ratelimit-local
scripts/capture-provider-benchmark.sh cache-local
scripts/capture-provider-benchmark.sh graphio
```

Expected: four canonical outputs with exit status 0 and five samples per benchmark row.

- [ ] **Step 4: Run container families one at a time**

```bash
scripts/capture-provider-benchmark.sh leader-containers
scripts/capture-provider-benchmark.sh leader-probes
scripts/capture-provider-benchmark.sh ratelimit-containers
scripts/capture-provider-benchmark.sh cache-redis
scripts/capture-provider-benchmark.sh graphdb
```

Expected: each command exits 0 before the next begins. If any command fails, preserve its failure
artifact, stop downstream reporting for that family, repair the benchmark/fixture in its owning
task, commit, return to a clean HEAD, and rerun every affected family.

- [ ] **Step 5: Validate evidence integrity**

```bash
rg -n 'exit_status: 0' docs/research/outputs/issue-560
rg -n '(://[^/@[:space:]]+:[^/@[:space:]]+@|(?i)(password|passwd|token|secret|authorization)[=:][^[:space:]]+)' docs/research/outputs/issue-560
```

Expected: every canonical output has exit status 0; the secret scan has no matches. Manually
confirm row counts and min/median/max availability for all container scenarios.

- [ ] **Step 6: Commit raw evidence**

```bash
git add docs/research/outputs/issue-560
git commit -m "Retain current-head provider measurements" \
  -m "Constraint: all container families ran serially from one clean committed HEAD" \
  -m "Rejected: partial successful-family publication | issue 560 requires every implemented multi-provider family" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: nine capture commands, leader probes, artifact exit headers, and secret scan"
```

### Task 9: Generate Charts, Report Decisions, and Link Both READMEs

**Files:**
- Create: `docs/research/2026-07-20-issue-560-provider-benchmark-matrix.md`
- Create: `docs/images/readme-charts/generate-provider-benchmark-summaries.mjs`
- Create: `docs/images/readme-charts/provider-benchmark-*-summary.vl.json`
- Create: `docs/images/readme-charts/provider-benchmark-*-summary.svg`
- Create: `docs/images/readme-charts/provider-benchmark-*-summary.png`
- Modify: `README.md`
- Modify: `README.ko.md`

- [ ] **Step 1: Load the chart workflow before visual work**

Read and follow `bluetape-diagram/SKILL.md`. Reuse the current repository chart style and retain
generator, Vega-Lite JSON, SVG, and PNG.

- [ ] **Step 2: Write a failing strict raw-output parser**

The Node generator must reject unknown, missing, duplicate, non-finite, or non-zero-exit rows.
Before implementing parsing, add a self-test mode:

```bash
node docs/images/readme-charts/generate-provider-benchmark-summaries.mjs --self-test
```

Expected before parser implementation: FAIL. The GREEN self-test covers one valid fixture and
unknown/missing/duplicate/non-finite/error-exit fixtures.

- [ ] **Step 3: Generate five family chart sets**

Produce leader, rate limiter, cache, graph I/O, and GraphDB chart sets. Chart only equivalent
latency rows, keep `ExpiryTakeover` separate, use min/median/max or error bars for container
samples, label units, provide sufficient contrast and direct provider labels/patterns, and never
depend on color alone. Keep allocation/density details in tables rather than mixing units.

- [ ] **Step 4: Visually inspect rendered assets**

Open every PNG and SVG at document scale. Verify labels, clipping, units, contrast, ordering,
pattern/label distinction, and consistency with the adjacent raw-derived table. Regenerate until
all checks pass.

- [ ] **Step 5: Write the English report**

For each family include workload/semantic boundary, exact command/raw link, metric direction,
table, chart with descriptive alt text, measured evidence, use-case selection guidance,
caveats/not-proven, and priority/instrumentation follow-up decision. Explicitly record:

- local rows are lower-bound/API baselines, not distributed winners;
- active-holder/renewal leader probes are not ranked;
- cache batch put is `N/A: no public bulk mutation contract`;
- graph-store construction is `N/A: no shared construction API`;
- RESP3 tracking remains a spike, not the production near-cache row;
- one local Docker snapshot does not establish production SLO, cloud cost, WAN behavior, or a
  universal winner.

If evidence suggests a new priority or instrumentation gap, link an existing issue or include a
follow-up issue draft only. Do not create the GitHub issue without separate approval.

- [ ] **Step 6: Add synchronized root README links**

`README.md` and `README.ko.md` must both link the report and show:

```bash
scripts/capture-provider-benchmark.sh leader-local
```

The accepted family argument is exactly one of `leader-local`, `leader-containers`,
`ratelimit-local`, `ratelimit-containers`, `cache-local`, `cache-redis`, `graphio-local`,
`graph-provider-containers`, or `leader-probes`.

Explain in each locale that results are short local snapshots and should not be copied as
production rankings. Do not duplicate result numbers in README.

- [ ] **Step 7: Validate and commit docs/charts**

```bash
node docs/images/readme-charts/generate-provider-benchmark-summaries.mjs --self-test
node docs/images/readme-charts/generate-provider-benchmark-summaries.mjs
git diff --check
rg -n 'provider benchmark|provider 벤치마크' README.md README.ko.md
```

Expected: parser self-tests pass, regeneration is deterministic, both README files contain the
same report/capture surface, and diff check passes.

Commit:

```bash
git add docs/research/2026-07-20-issue-560-provider-benchmark-matrix.md docs/images/readme-charts README.md README.ko.md
git commit -m "Explain provider choices from bounded evidence" \
  -m "Constraint: tables and charts may compare only equivalent scenario semantics" \
  -m "Rejected: universal provider ranking | one local snapshot cannot prove production fitness" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: strict parser self-tests, deterministic chart generation, visual QA, README parity, and diff check"
```

### Task 10: Record the Type A Lesson and Run Targeted Gates

**Files:**
- Create: `docs/lessons/2026-07-20-issue-560-provider-benchmark-matrix.md`

- [ ] **Step 1: Write the reusable lesson**

Record the reusable decisions: semantic equivalence before timing, deadline/sleep probes outside
rankings, disposable fixture provenance, deterministic concurrency join, parser/materialization
boundary, atomic raw capture, and selection guidance instead of universal winners. Include concrete
file/command evidence and explain when the pattern is `N/A`.

- [ ] **Step 2: Run targeted package and script validation**

```bash
go test -count=1 ./testcontainers/redis ./testcontainers/postgres ./testcontainers/mongodb
go test -count=1 ./leader ./ratelimit ./cache ./graph ./graph/graphio ./graph/neo4j
go test -race -count=1 ./leader ./ratelimit ./cache ./graph ./graph/graphio
bash -n scripts/capture-provider-benchmark.sh scripts/capture-provider-benchmark_test.sh
bash scripts/capture-provider-benchmark_test.sh
node docs/images/readme-charts/generate-provider-benchmark-summaries.mjs --self-test
```

Expected: all PASS; container benchmarks remain skipped unless explicitly opted in.

- [ ] **Step 3: Commit lesson**

```bash
git add docs/lessons/2026-07-20-issue-560-provider-benchmark-matrix.md
git commit -m "Preserve provider benchmark lessons" \
  -m "Constraint: future provider additions need the same semantic and evidence boundaries" \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Tested: targeted package, race, capture-script, and chart-parser checks"
```

### Task 11: Run Full Verification, Reviews, and Prepare the PR

**Files:**
- Modify only files required by verified P0/P1 review repairs.

- [ ] **Step 1: Run full repository gates**

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

Expected: every command exits 0. Testcontainers-backed work stays serial. If the known
`leader/sql TestLeaseStatements/concurrent-single-winner` symptom recurs, capture the failure,
rerun the exact subtest repeatedly, and distinguish valid lease expiry from a new regression
before changing production code.

- [ ] **Step 2: Re-run artifact reproducibility and security checks**

```bash
node docs/images/readme-charts/generate-provider-benchmark-summaries.mjs
git diff --exit-code -- docs/images/readme-charts
rg -n '(://[^/@[:space:]]+:[^/@[:space:]]+@|(?i)(password|passwd|token|secret|authorization)[=:][^[:space:]]+)' docs/research/outputs/issue-560
git status --short
```

Expected: deterministic charts, no secret matches, and clean worktree.

- [ ] **Step 3: Run Step 6-R and Step 7-R reviews**

Run six independent performance, stability, security, Operator/Ops, Developer/API, and
User/caller lanes plus main integration. Review the implemented diff and then exact PR head.
Resolve every P0/P1; fix, justify, or file every P2/P3. Heavy container commands remain in the
main session and sequential.

- [ ] **Step 4: Render the DoD**

Report exact counts/evidence for scope, targeted/full tests, race, lint/vet/tidy/fmt, all nine
capture commands, five family tables/charts, environment/redaction, README parity, lesson, and
7-Tier results. Required final state: `Blocked=0`, `P0=0`, `P1=0`.

- [ ] **Step 5: Push and create the approved PR**

Push `perf/issue-560-provider-benchmark-matrix` and create:

```text
bluetape4k/bluetape-go
develop <- perf/issue-560-provider-benchmark-matrix
```

The PR body must include `Closes #560`, benchmark snapshot caveats, exact validation evidence,
review provenance, and central `## DoD Status`. PR creation is already in the approved delivery
scope. Do not enable auto-merge and do not merge.

- [ ] **Step 6: Stop at merge-ready**

After CI, current reviews/threads, exact-head verification, and applicable human-review artifacts
pass, report the exact PR/head as merge-ready and wait for a fresh explicit user approval.
