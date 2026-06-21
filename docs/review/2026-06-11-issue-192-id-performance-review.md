# Issue #192 7-Tier Review: ID generator performance optimization

## Scope

- Issue: #192 `perf: Improve UUID, ULID, and KSUID generation performance`
- Branch: `issue/192-id-performance`
- Base: `develop` at `ce4e5ca5d39bff8c5542c093bc67585b34f148a0`
- Preserved baseline: Issue #168 benchmark report and chart remain unchanged.

## Summary

The change keeps the Issue #168 Kotlin-vs-Go chart as the pre-optimization
baseline and adds a Go-only Issue #192 optimization pass. UUID, ULID, KSUID
seconds, and KSUID millis now use a package-local locked buffered entropy reader
over `crypto/rand` by default. KSUID millis also avoids the previous temporary
encoder allocation by writing the required 27-character prefix through a fixed
local buffer.

The generated IDs remain identifiers, not authentication or authorization
secrets. Custom entropy readers are still caller-provided and must be safe for
concurrent use when a generator is shared.

## 7-Tier Findings

### Tier 1: Acceptance and Scope

- Finding: none.
- Evidence: Issue #192 scope is covered by raw baseline/final benchmarks,
  profiles, benchstat output, before/after chart, and research note.
- Gate: P0=0, P1=0.

### Tier 2: API and Compatibility

- Finding: none.
- Evidence: Public APIs remain string-returning generators. Custom entropy
  options continue to override the default reader. README wording documents the
  buffered `crypto/rand` default and custom-reader concurrency boundary.
- Gate: P0=0, P1=0.

### Tier 3: Go Code Quality

- Finding: none after P2 polish.
- Evidence: `lockedBufferedEntropy` is a small package-local primitive with a
  direct unit test. UUID reader GoDoc now matches the concurrency warning used
  by KSUID options. ULID `MarshalTextTo` error is checked even though the buffer
  size is fixed by `oklog/ulid`.
- Gate: P0=0, P1=0.

### Tier 4: Test and Race Evidence

- Finding: none.
- Evidence: ID package normal/race tests, entropy concurrent stress, goroutine
  uniqueness stress, and full `make ci` passed after the final P2 polish.
  Entropy stress uses `testing/concurrency.GoroutineStressTester`.
  AsyncJobTester N/A: entropy `Read` has no caller-observable context, async,
  IO cancellation, or deadline boundary.
- Gate: P0=0, P1=0.

### Tier 5: Performance Challenge

- Finding: none after scope labeling repair.
- Evidence: Review challenged overbroad chart/report language because Snowflake
  is excluded. The chart title/subtitle now say `UUID / ULID / KSUID` and
  `excludes Snowflake`. A second post-optimization Kotlin-vs-Go chart now keeps
  Snowflake as an unchanged synthetic-clock row and updates UUID, ULID, and
  KSUID with the optimized Go values. The report records rejected/deferred
  experiments and commands.
- Gate: P0=0, P1=0.

### Tier 6: Completion Verification

- Finding: none.
- Evidence: Verifier confirmed live issue acceptance criteria, raw artifacts,
  benchstat, profiles, chart assets, `git diff --check`, targeted tests, race
  tests, and `make ci`.
- Gate: P0=0, P1=0.

### Tier 7: Documentation and Visual Evidence

- Finding: none.
- Evidence: Vision review of the before/after chart passed with no clipping,
  overlap, readability, legend, title-gap, purpose, color, or baseline
  preservation blockers.
- Gate: P0=0, P1=0.

## Performance Result

- Geomean latency: `-58.34%`.
- UUID v4 reused: `224.10 ns/op -> 45.57 ns/op`.
- UUID v7 reused: `255.60 ns/op -> 73.95 ns/op`.
- ULID random: `104.50 ns/op -> 63.12 ns/op`.
- KSUID seconds: `393.10 ns/op -> 217.90 ns/op`.
- KSUID millis: `316.80 ns/op -> 122.80 ns/op`.
- KSUID millis allocation: `104 B/op, 3 allocs/op -> 56 B/op, 2 allocs/op`.

## Validation

- PASS: `git diff --check`
- PASS: `go test -count=1 ./id`
- PASS: `go test -race -count=1 ./id`
- PASS: `go test -count=50 ./id`
- PASS: `go test -run TestLockedBufferedEntropyConcurrentStress -count=50 ./id`
- PASS: `go test -race -run TestLockedBufferedEntropyConcurrentStress -count=10 ./id`
- PASS: `rg -n "GoroutineStressTester|AsyncJobTester N/A" id/entropy_test.go docs/review/2026-06-11-issue-192-id-performance-review.md`
- PASS: `make ci`
- PASS: `go test -run '^$' -bench 'Benchmark(UUID|ULID|KSUID)' -benchmem -count=10 ./id`
- PASS: `benchstat` baseline vs final output saved under `docs/research/outputs/issue-192/`
- PASS: baseline and final CPU/memory profiles saved under `docs/research/outputs/issue-192/`
- PASS: `xmllint --noout docs/images/readme-charts/id-generator-optimization-before-after.svg`
- PASS: rendered `docs/images/readme-charts/id-generator-optimization-before-after.png` with `rsvg-convert`
- PASS: `xmllint --noout docs/images/readme-charts/id-generator-kotlin-go-optimized-comparison.svg`
- PASS: rendered `docs/images/readme-charts/id-generator-kotlin-go-optimized-comparison.png` with `rsvg-convert`
- PASS: visually inspected `docs/images/readme-charts/id-generator-kotlin-go-optimized-comparison.png`
- PASS: native vision subagent for optimized Kotlin-vs-Go chart: P0=0 P1=0
- PASS: native code-reviewer subagent: P0=0 P1=0
- PASS: native test-engineer subagent: P0=0 P1=0
- PASS: native critic subagent: P0=0 P1=0
- PASS: native verifier subagent: P0=0 P1=0
- PASS: native vision subagent: P0=0 P1=0

## Subagent Gate Summary

- Tier 1 main integration: P0=0 P1=0.
- Tier 2 code reviewer subagent: P0=0 P1=0.
- Tier 3 test/concurrency subagent: P0=0 P1=0.
- Tier 4 critic/performance challenge subagent: P0=0 P1=0.
- Tier 5 verifier subagent: P0=0 P1=0.
- Tier 6 vision subagent: P0=0 P1=0.
- Tier 7 final integration after P2 polish: P0=0 P1=0.

## Blog Seed

This issue preserves a useful narrative: the initial chart made Go look weaker
than expected; generator reuse was not the main lever; profiling showed entropy
reads dominated; buffered crypto entropy moved the bottleneck to string
encoding, time access, and UUID v7 ordering. Keep Issue #168 as the baseline
chart and use Issue #192's before/after chart for the optimization chapter.

## Gate Verdict

P0=0 P1=0

Gate passes for PR creation. Merge still requires explicit user approval after
the PR is created and GitHub CI is verified.
