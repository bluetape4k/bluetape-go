# Lessons Learned - Provider Benchmark Matrix (#560)

**Related issue:** #560

**Affected packages:** `leader`, `ratelimit`, `cache`, `graph`, `graph/graphio`,
`testcontainers/*`

## L1: Semantic equivalence comes before timing

### Problem

Rows with the same metric can still measure different contracts. A local reference-object cache
hit, a serialized Redis hit, a lease-expiry wait, and a graph traversal all report `ns/op`, but a
single ranking would erase the boundary that gives each number meaning.

### Decision

Each benchmark name encodes provider, scenario, concurrency or shape, and payload where applicable.
The report and charts compare only equivalent scenarios. Deadline-driven leader takeover is
isolated, and local leader/rate-limit rows are lower-bound API baselines rather than distributed
competitors.

### Evidence

`leader/provider_benchmark_test.go`, `ratelimit/provider_benchmark_test.go`,
`cache/provider_benchmark_test.go`, `graph/provider_benchmark_test.go`, and
`graph/graphio/provider_benchmark_test.go` define the scenario boundaries. The strict chart parser
in `docs/images/readme-charts/generate-provider-benchmark-summaries.mjs` rejects missing, unknown,
duplicate, non-finite, or failed rows before reporting them.

### Future Guard

If two providers cannot expose the same public operation and verification contract, mark the row
`N/A` or use separate panels. Do not normalize different semantics into one benchmark name.

## L2: Deadline and sleep behavior is probe evidence, not ordinary latency

### Problem

Lease expiry and active-holder behavior intentionally include waiting. Mixing them with ordinary
campaign or lookup rows compresses the fast operations and encourages a false provider ranking.

### Decision

Expiry takeover has its own command section and chart panel. Active-holder and renewal/deadline
behavior remains in `leader-probes.txt` as correctness evidence and is not ranked.

### Evidence

`scripts/capture-provider-benchmark.sh leader-containers` runs ordinary and expiry sections
independently. The chart parser requires both sections and never merges their samples.

### Future Guard

Use an ordinary benchmark only when the timer surrounds repeatable provider work. Put explicit
deadline, sleep, renewal, and failure-transition assertions in bounded tests or probes.

## L3: Disposable fixtures need immutable and observed provenance

### Problem

A mutable tag records intent, not the service that actually ran. Deriving `7.4` from a Redis image
tag hid the provider-reported `7.4.9` and made the same pinned image appear inconsistent across
families.

### Decision

Pin every container image by reviewed digest, query the service version during untimed setup, and
verify that the reported version matches the pinned image authority. Record both values in every
successful container artifact and in `environment.md`.

### Evidence

`docs/research/outputs/issue-560/environment.md` records six immutable fixtures and observed
versions. The leader, rate-limit, cache, and graph benchmark fixtures fail closed when the version
is absent or incompatible with the configured authority.

### Future Guard

Never substitute a tag or dependency version for provider-reported runtime evidence. A new provider
is `N/A` for publication until both immutable image identity and service version are available.

## L4: Concurrent benchmarks must join every worker deterministically

### Problem

Returning after the first winner or error can leave workers running into cleanup, contaminate the
next iteration, and report latency for incomplete work.

### Decision

Concurrent scenarios start workers behind a common gate, cancel peers on the first terminal result,
and join every worker before verification and cleanup. Setup, seed, and cleanup stay outside the
timer; the provider operation and required completion remain inside it.

### Evidence

`TestRunLeaderRoundJoinsAllWorkers`, `TestRunLeaderRoundStartsAllWorkersBeforeWinnerCancellation`,
and the rate-limit parallel benchmark checks preserve deterministic completion. Race tests support
the invariants but do not replace the named join assertions.

### Future Guard

Any new parallel row must prove start coordination, first-error propagation, complete join, exact
side-effect counts, and cleanup ordering before it is used for performance claims.

## L5: Parser work and graph materialization are separate costs

### Problem

Graph I/O read/round-trip rows can silently conflate parsing with graph-store construction. There is
no shared construction API across the current formats/providers, so a construction ranking would
not be semantically stable.

### Decision

Graph I/O includes a `RecordConstructionBaseline` beside write, read, and round-trip rows, while the
report charts only equivalent format round trips. Graph-store construction is explicitly
`N/A: no shared construction API`.

### Evidence

`graph/graphio/provider_benchmark_test.go` produces all four operations for each format and shape;
`graphio.txt` preserves 180 raw samples. The generated graph I/O chart selects only the medium
round-trip rows.

### Future Guard

Do not infer store-ingestion throughput from parser benchmarks. Add a shared construction contract
and separately verified fixture before introducing that comparison.

## L6: Raw capture must be atomic, fail closed, and preserve failures

### Problem

Writing directly to a canonical artifact can overwrite good evidence with a failed or interrupted
run. Broad redaction can also hide benchmark output while giving the appearance of success.

### Decision

Capture raw output in a private directory outside the repository, sanitize and rescan the complete
artifact, then atomically publish only a zero-exit result. Keep non-zero or blocked captures under a
timestamped `-failed-` name without replacing the previous canonical file. Signal cleanup retains
private state until shell unwind completes.

### Evidence

`scripts/capture-provider-benchmark_test.sh` exercises clean publication, dirty-source rejection,
output confinement, secret blocking, publication failure, and signal behavior. The issue output
directory retains four development failure artifacts alongside nine successful canonical files.

### Future Guard

A benchmark capture helper is not complete until interruption, filesystem failure, redaction
failure, and stale-canonical preservation are tested. Never publish a partial stream as canonical.

## L7: Selection guidance is more durable than a universal winner

### Problem

One Apple Silicon host and one local Docker runtime cannot prove production SLOs, cloud cost,
failure recovery, WAN behavior, or operational fit.

### Decision

The report presents min/median/max evidence, operation boundaries, provider selection conditions,
and explicit `not proven` lists. It retains raw output, environment, tables, Vega-Lite sources,
SVG, and PNG so a future run can be compared without treating this snapshot as timeless.

### Evidence

`docs/research/2026-07-20-issue-560-provider-benchmark-matrix.md` contains five family decisions and
links every canonical artifact. `README.md` and `README.ko.md` expose the same capture surface and
warn against copying the numbers as production rankings.

### Future Guard

Re-run on deployment-relevant architecture before changing an operational recommendation. When a
snapshot exposes an instrumentation gap, draft a focused follow-up with telemetry and workload
requirements instead of changing provider semantics in the benchmark issue.
