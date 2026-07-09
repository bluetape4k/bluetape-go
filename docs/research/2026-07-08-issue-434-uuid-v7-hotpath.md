# Issue #434 UUID v7 parallel hot-path evaluation

Issue: #434
Date: 2026-07-08
Work type: Type B performance evaluation

## Question

Should `id.NewUUIDV7Generator` replace the generator-wide UUID v7 mutex with a
narrower hot-path strategy such as atomic logical tick reservation?

## Evidence

- Environment: `outputs/issue-434/environment.md`
- Baseline benchmark: `outputs/issue-434/uuid-v7-baseline-count10.txt`
- Baseline profile bench: `outputs/issue-434/uuid-v7-reuse-parallel-profile-bench.txt`
- Baseline CPU top: `outputs/issue-434/uuid-v7-reuse-parallel-cpu-top.txt`
- Baseline mutex top: `outputs/issue-434/uuid-v7-reuse-parallel-mutex-top.txt`
- Atomic candidate benchmark: `outputs/issue-434/uuid-v7-atomic-count10.txt`
- Baseline vs atomic candidate: `outputs/issue-434/benchstat-atomic-candidate.txt`

## Baseline

The current implementation still shows the same shape recorded after issue
#192: single-thread UUID v7 generation is stable, while a shared generator used
from `b.RunParallel` spends visible time around the UUID v7 ordering mutex.

The fresh baseline on `Apple M5` and Go `1.26.5` measured:

| Benchmark | Baseline result |
|---|---:|
| `UUIDV7NewString` | `82.02 ns/op +/- 4%`, `112 B/op`, `3 allocs/op` |
| `UUIDV7NewStringParallel` | `200.1 ns/op +/- 1%`, `112 B/op`, `3 allocs/op` |
| `UUIDV7ReuseGenerator` | `68.00 ns/op +/- 0%`, `64 B/op`, `2 allocs/op` |
| `UUIDV7ReuseGeneratorParallel` | `192.6 ns/op +/- 1%`, `64 B/op`, `2 allocs/op` |

CPU and mutex profiles confirm that shared-generator parallel UUID v7 remains
contention-shaped. `io.ReadFull` accounted for only a small cumulative slice in
the sampled CPU profile, so the issue #192 entropy-buffering fix is still
holding.

## Candidate

A local candidate replaced the UUID v7 generator mutex with atomic logical tick
reservation. It preserved the same benchmark/test surface but was rejected
because it did not materially improve the target row.

`benchstat` reported:

| Benchmark | Candidate result | Decision |
|---|---:|---|
| `UUIDV7NewString` | `79.52 ns/op +/- 6%` | noise, not material |
| `UUIDV7NewStringParallel` | `200.5 ns/op +/- 1%` | no improvement |
| `UUIDV7ReuseGenerator` | `68.04 ns/op +/- 0%` | no improvement |
| `UUIDV7ReuseGeneratorParallel` | `193.2 ns/op +/- 1%` | no improvement |

The candidate geomean changed by `-0.62%`, and allocations were unchanged.

## Decision

Do not change production UUID v7 generation in this issue.

The measured target row did not improve, so replacing the mutex with atomic
tick reservation would add concurrency complexity without a proven performance
benefit. Keep the existing mutex-based implementation because it is simpler,
preserves deterministic reader/clock behavior, and already passes the existing
ordering, rollback, overflow, stress, and race checks.

## Follow-up

Further UUID v7 work should start from a different hypothesis:

- reduce string allocation/encoding cost;
- benchmark caller-owned generator pools instead of a single shared generator;
- evaluate sharded generators only with a clear API contract for per-shard
  ordering tradeoffs.

Do not re-open the atomic logical tick reservation approach unless a new Go
runtime or benchmark profile shows a materially different result.
