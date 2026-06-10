# Issue #168 7-Tier Review: ID generator benchmark comparison

## Scope

- Issue: #168 `Benchmark id generators against bluetape4k-idgenerators`
- Branch: `issue/168-id-benchmarks`
- Base: `origin/develop`
- Changed files:
  - `Makefile`
  - `README.md`
  - `README.ko.md`
  - `id/id_benchmark_test.go`
  - `id/README.md`
  - `id/README.ko.md`
  - `docs/research/README.md`
  - `docs/research/README.ko.md`
  - `docs/research/2026-06-10-issue-168-id-generator-benchmark.md`
  - `docs/research/outputs/issue-168/*`

## Summary

The change adds a reproducible `make bench-id` target, enables allocation
reporting for existing Go ID benchmarks, and preserves a local comparison
snapshot against the sibling JVM `bluetape4k-idgenerators` benchmark suite. The
JVM side is documented as `kotlinx-benchmark` with the JMH JVM backend, not as a
hand-written raw JMH harness.

The report keeps Go per-ID `ns/op` benchmark results separate from JVM batch
throughput plus uniqueness checks. It also records stress/race evidence for the
Go `id` package and concludes that no evidence-backed follow-up implementation
issue is required from this snapshot.

## 7-Tier Findings

### Tier 1: Acceptance and Scope

- Finding: none.
- Evidence: The research note covers comparable UUID v4/v7, ULID
  random/monotonic, KSUID seconds/millis, and Snowflake surfaces, with commands,
  environment, raw output paths, interpretation limits, and conclusion.
- Gate: P0=0, P1=0.

### Tier 2: Dependency and Benchmark Semantics

- Finding: none.
- Evidence: The JVM terminology was checked against sibling Gradle tasks and
  source. The docs use `kotlinx-benchmark` with the JMH JVM backend and list
  both valid short configuration tasks and generated benchmark task names.
- Gate: P0=0, P1=0.

### Tier 3: Go Code Quality

- Finding: none.
- Evidence: The Go production API is unchanged. Benchmark changes are limited to
  `b.ReportAllocs()` and the opt-in `make bench-id` target.
- Gate: P0=0, P1=0.

### Tier 4: Test and Stress Evidence

- Finding: none after repair.
- Evidence: Review initially flagged missing retained Go stress/race output. The
  raw artifacts now include `go-id-test.txt`, `go-id-race-test.txt`, and
  `revisions.txt`; the report explicitly states that cross-family goroutine
  uniqueness is covered by Go tests/race rather than per-algorithm throughput
  benchmarks.
- Gate: P0=0, P1=0.

### Tier 5: Critical Challenge

- Finding: none after repair.
- Evidence: Review flagged that #168 was indexed under `0.6.0`; both English
  and Korean research indexes now place the report under `0.6.1`.
- Gate: P0=0, P1=0.

### Tier 6: Completion Verification

- Finding: none after repair.
- Evidence: The verifier confirmed local PR-readiness, raw artifact existence,
  package tests, race tests, benchmark target, and `make ci` pass. The only
  verifier P2 was the same `0.6.1` research-index mismatch, now repaired.
- Gate: P0=0, P1=0.

### Tier 7: Documentation and Bilingual Parity

- Finding: none after repair.
- Evidence: The writer review confirmed the benchmark terminology and
  interpretation boundary, then flagged the `0.6.1` index mismatch. English and
  Korean indexes were updated together.
- Gate: P0=0, P1=0.

## Validation

- PASS: `git diff --check`
- PASS: `go test -count=1 ./id`
- PASS: `go test -race -count=1 ./id`
- PASS: `make bench-id`
- PASS: `make ci`
- PASS: sibling `./gradlew :bluetape4k-idgenerators:singleThreadBenchmark`
- PASS: sibling `./gradlew :bluetape4k-idgenerators:concurrentBenchmark`

## Subagent Gate Summary

- Tier 1 analyst: P0=0 P1=0
- Tier 2 dependency expert: P0=0 P1=0
- Tier 3 code reviewer: P0=0 P1=0
- Tier 4 test engineer: P0=0 P1=0 after repair
- Tier 5 critic: P0=0 P1=0, P2 repaired
- Tier 6 verifier: P0=0 P1=0, P2 repaired
- Tier 7 writer: P0=0 P1=0, P2 repaired

## Gate Verdict

P0=0 P1=0

Gate passes for PR creation. PR-level body and CI evidence must be verified
after `gh pr create`.
