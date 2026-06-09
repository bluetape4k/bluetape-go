# Issue #36 Spec Review

Spec: `docs/superpowers/specs/2026-06-09-issue-36-probabilistic-bloom-filter-spec.md`
Review date: 2026-06-09
Scope: Step 2-R local 7-Tier review plus in-flight subagent review lanes.

## Integrated Findings

P0=0 P1=0

| Tier | P0 | P1 | P2 | P3 | Evidence |
|---|---:|---:|---:|---:|---|
| Security | 0 | 0 | 0 | 0 | No serialization, reflection, Redis, network, file, or credential boundary in #36. |
| Ops/SRE | 0 | 0 | 0 | 0 | Spec bounds memory through config, has no goroutine ownership, and records Redis as #182. |
| Structural | 0 | 0 | 0 | 0 | Package is `probabilistic`; Redis Bloom/Cuckoo/HLL are explicitly deferred to #182. |
| Go API quality | 0 | 0 | 0 | 0 | Concrete type, explicit `Hasher[T]`, sentinel errors, and no Kotlin-style DSL hierarchy. |
| Tests/types | 0 | 0 | 0 | 0 | Spec requires config, merge, false-positive, nil/sentinel, stress, and race tests. |
| Performance/stability | 0 | 0 | 0 | 0 | Spec fixes SHA-256 double hashing, bounded hash count, and mutex-backed goroutine safety. |
| Docs/release | 0 | 0 | 0 | 0 | Spec requires bilingual README, root README, CHANGELOG, WIP, #182 deferral, PR metadata. |

## Review Notes

- Accepted deliberate Go deviation from Kotlin source: Kotlin states writes are
  not thread-safe; Go package will be goroutine-safe and prove it with
  `GoroutineStressTester` plus race detector.
- Rejected adding Redis Bloom/Cuckoo/HLL to #36 because #182 tracks that parity
  under `0.6.1` and #36 is the `0.6.0` in-memory closure.
- `AsyncJobTester` is N/A because the public API has no context-aware async,
  I/O, timer, or cancellation boundary.

## Subagent Findings And Repair

Initial subagent source-parity review reported P0=0 P1=3:

| Priority | Finding | Repair |
|---|---|---|
| P1 | Go `func` hasher identity cannot support merge compatibility. | Spec now uses `Hasher[T]` with explicit comparable compatibility key. |
| P1 | Exported concrete `BloomFilter[T]` made zero-value behavior unsafe/underspecified. | Spec now exposes `BloomFilter[T]` interface backed by unexported implementation. |
| P1 | Concurrency contract overclaimed relative to stress plan. | Spec now defines `PutAll` snapshot/lock behavior, self-merge no-op, and stress/race coverage for `PutAll`/`Clear`. |

Repair review: P0=0 P1=0. Implementation may proceed.

## Step DoD

| Step | Status | Evidence |
|---|---|---|
| Step 2-R spec review | PASS | P0=0 P1=0 in this artifact |
| Next step unblocked | PASS | Plan review may proceed |
