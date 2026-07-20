# Provider Benchmark Matrix for Issue 560

## Decision summary

This report compares only operations with matching semantics inside each family. It does not
produce a universal provider ranking. The measurements are a short local snapshot on the machine
recorded in [environment.md](outputs/issue-560/environment.md), from Git SHA
`1f1069f4b119957d96158c969f819d6374f902c8`.

- Use local rows as API and implementation lower bounds, not as distributed-provider winners.
- Choose leader and rate-limit providers from operational requirements first; the measured
  latency shows the cost shape of this fixture, not production availability or durability.
- Preserve the cache boundary: L1 keeps reference objects, while L2 serialization and network
  work dominate remote paths.
- Choose graph I/O by interchange requirements before local round-trip latency.
- Keep PostgreSQL recursive CTE as a strong first-party traversal baseline; the snapshot does not
  justify adopting or rejecting a graph database universally.

All charts show observed min, median, and max `ns/op`. Lower is better. Allocation data and every
unselected row remain in the linked raw Go benchmark outputs.

## Reproduction and evidence

Run one family at a time:

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

Each raw artifact records the exact `go test` command, timestamp, Git SHA, benchmark output, and
exit status. Container artifacts also record the provider-reported version and immutable image
reference. Failed development captures are retained separately with `-failed-` in their names and
are not report inputs. Each command captures at most 16 MiB by default; an overflow fails closed
without replacing canonical evidence. `BLUETAPE_PROVIDER_BENCH_MAX_OUTPUT_BYTES` may set a smaller
positive bound for constrained environments.

To refresh the tracked snapshot:

1. Start from a clean worktree, record the measured Git SHA and current host/runtime details in
   `environment.md`, and ensure Docker is available for container-backed families.
2. Run all nine families above sequentially. Do not overlap container-backed captures.
3. Reconcile provider-reported versions and immutable image references from the successful raw
   artifacts into `environment.md`; stop if a provider is missing, duplicated, or inconsistent.
4. Run `node docs/images/readme-charts/generate-provider-benchmark-summaries.mjs --self-test`,
   then run the same script without arguments to regenerate all chart sources, SVGs, and PNGs.
5. Update the report tables from the new raw min/median/max values, cross-check them against the
   generated Vega-Lite data, then run the repository diff, redaction, and visual chart checks.

The generator fails closed when a canonical artifact's recorded SHA differs from the authority in
`environment.md`. A partial recapture must not be published as a complete matrix.

## Leader election

### Workload and semantic boundary

The ranked rows time uncontended campaign, owned resignation, supported contention, leader lookup,
and lease-expiry takeover against disposable Redis, MongoDB, PostgreSQL, and etcd fixtures. The
chart selects uncontended campaign and lookup as ordinary operations and isolates expiry takeover
because its intentional lease wait is a different latency class. etcd contention is not ranked
because the deterministic same-process fixture cannot establish an equivalent multi-session
contention row.

- Local lower bound: [leader-local.txt](outputs/issue-560/leader-local.txt)
- Container commands and rows: [leader-containers.txt](outputs/issue-560/leader-containers.txt)
- Correctness/deadline probes: [leader-probes.txt](outputs/issue-560/leader-probes.txt)

Metric direction: lower `ns/op` is better for the same operation. Active-holder and renewal/deadline
probes are correctness evidence, not ranking inputs.

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

- Redis is suitable when the deployment already operates Redis and low local-network coordination
  latency matters more than adding another consensus system.
- MongoDB and PostgreSQL avoid a new provider when either database is already the operational
  authority; their different operation profiles should be evaluated against the application mix.
- etcd remains appropriate when its consensus and operational model are already required. This
  snapshot's expiry behavior includes provider semantics and polling/deadline behavior, not just a
  network round trip.

Not proven: failover availability, clock-skew tolerance, quorum loss, WAN behavior, renewal under
sustained load, or production fencing guarantees. Priority decision: keep the correctness probes
outside rankings and add multi-process/load evidence before changing a provider recommendation.

## Rate limiting

### Workload and semantic boundary

The provider rows compare the same single-key allow/reject decisions and the same eight-worker
parallel/distinct-key shapes for Redis and PostgreSQL. The local token bucket is only an API lower
bound and does not participate in the distributed ranking.

- Local lower bound: [ratelimit-local.txt](outputs/issue-560/ratelimit-local.txt)
- Container rows: [ratelimit-containers.txt](outputs/issue-560/ratelimit-containers.txt)

Metric direction: lower `ns/op` is better for an equivalent allow decision.

| Scenario | Redis median | PostgreSQL median |
| --- | ---: | ---: |
| Allow available | 311.7 us | 243.1 us |
| Allow rejected | 210.4 us | 223.6 us |
| Parallel, one key, N=8 | 505.3 us | 10.65 ms |
| Parallel, distinct keys, N=8 | 482.9 us | 11.75 ms |

![Rate-limit provider latency chart comparing Redis and PostgreSQL min, median, and max for equivalent allow decisions on a logarithmic axis](../images/readme-charts/provider-benchmark-ratelimit-summary.png)

### Selection guidance

PostgreSQL had the lower median for the single-key available path, while Redis was lower for the
rejected path and both eight-worker rows. PostgreSQL remains a valid choice when transactional
colocation and one operational datastore are more important than the concurrency shape observed
here. The result is a profiling signal, not evidence that either contract is universally better.

Not proven: fairness under sustained arrival rates, multi-host lock contention, database pool
saturation, failover, cloud cost, or rate-limit accuracy during partitions. Follow-up issue draft:
profile the PostgreSQL parallel path with connection-pool telemetry and a controlled multi-client
arrival curve before making a high-contention recommendation.

## Cache

### Workload and semantic boundary

Local memory, Redis L2, tiered cache, and Pub/Sub near-cache rows preserve different boundaries.
L1 rows return reference objects without serialization. L2 and tiered-L2 rows include remote access
and serialization. Pub/Sub peer invalidation includes publication plus observed peer eviction.
RESP3 client tracking remains an experimental spike and is not the production near-cache row.

- Local memory and serialization baselines: [cache-local.txt](outputs/issue-560/cache-local.txt)
- Redis, tiered, and Pub/Sub near-cache rows: [cache-redis.txt](outputs/issue-560/cache-redis.txt)

Metric direction: lower `ns/op` is better only within the same cache-path semantics.

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

- Use the memory cache for process-local reference semantics and the lowest hot-read overhead.
- Use tiered cache when a local reference-object L1 plus serialized Redis L2 matches the consistency
  model. Treat the two layers as different semantic and performance boundaries.
- Use the Pub/Sub near cache when peer invalidation is required and eventual invalidation semantics
  are acceptable. The production row is Pub/Sub, not the RESP3 spike.

Batch put is `N/A: no public bulk mutation contract`. Not proven: eviction under memory pressure,
subscriber loss/recovery, cluster failover, WAN invalidation lag, hot-key amplification, or a
production SLO.

## Graph I/O

### Workload and semantic boundary

CSV, NDJSON, and GraphML use the same generated graph shapes and separately time write, read,
round-trip, and record-construction baselines. The chart compares only the medium
`10000V-20000E-5P` round trip. It does not mix throughput, allocations, or construction baselines
into the latency axis.

- Exact command and all rows: [graphio.txt](outputs/issue-560/graphio.txt)

Metric direction: lower `ns/op` is better for the same graph shape and round-trip operation.

| Format | Median | Observed min-max |
| --- | ---: | ---: |
| CSV | 120.59 ms | 119.66-120.62 ms |
| NDJSON | 87.35 ms | 86.68-87.67 ms |
| GraphML | 215.27 ms | 213.41-221.33 ms |

![Graph I/O chart comparing CSV, NDJSON, and GraphML medium round-trip min, median, and max latency](../images/readme-charts/provider-benchmark-graphio-summary.png)

### Selection guidance

NDJSON had the lowest medium round-trip median in this snapshot. CSV remains useful for simple
tabular interchange, while GraphML preserves a graph-oriented interchange contract. Choose the
format from interoperability and fidelity requirements before using this local latency ordering.

Graph-store construction is `N/A: no shared construction API`. Not proven: streaming backpressure,
compressed transport, malformed-input behavior at scale, disk I/O, cross-language compatibility,
or production dataset skew.

## Graph traversal providers

### Workload and semantic boundary

Neo4j, Memgraph, and PostgreSQL recursive CTE fixtures receive the same generated long-chain and
deep-wide shapes. Setup, seeding, version queries, and verification stay outside the timer. The
ranked operation is traversal only.

- Exact command and all rows: [graphdb.txt](outputs/issue-560/graphdb.txt)

Metric direction: lower `ns/op` is better for an equivalent seeded traversal.

| Shape | Neo4j median | Memgraph median | PostgreSQL CTE median |
| --- | ---: | ---: | ---: |
| Long chain, depth 16 | 2.16 ms | 537.5 us | 476.9 us |
| Long chain, depth 64 | 1.67 ms | 608.7 us | 1.06 ms |
| Deep-wide, depth 4 fanout 4 | 6.69 ms | 1.51 ms | 493.4 us |

![Graph traversal chart comparing Neo4j, Memgraph, and PostgreSQL recursive CTE min, median, and max latency for equivalent seeded shapes](../images/readme-charts/provider-benchmark-graphdb-summary.png)

### Selection guidance

PostgreSQL recursive CTE had the lowest median for depth-16 and deep-wide rows; Memgraph had the
lowest median for depth 64. Neo4j was slower in these three local shapes. These results justify
keeping the relational baseline and measuring the application's real query mix; they do not prove
that a provider is universally better.

Not proven: Cypher feature breadth, transactional graph updates, index strategy, concurrent mixed
read/write load, cluster behavior, operational tooling, or production-scale datasets. Priority
decision: do not add a new graph provider from these rows alone; require workload-specific
capability and operational evidence.

## Global limitations

One local Docker snapshot does not establish production SLOs, cloud cost, WAN behavior, failure
recovery, or a universal winner. The immutable images and provider-reported versions make this
snapshot auditable, not timeless. Re-run the same commands on the deployment-relevant architecture
and preserve a new environment manifest before changing an operational recommendation.
