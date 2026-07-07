# Issue #399 SerDe Fixture Matrix Review

Issue: #399
Branch: `issue-399-serde-benchmark-fixtures`
Review date: 2026-07-07
Scope:

- `docs/benchmarks/2026-07-07-issue-399-serde-fixtures.md`
- `docs/lessons/2026-07-07-serde-benchmark-fixtures.md`

## Acceptance Review

| Criterion | Evidence | Verdict |
|---|---|---|
| Fixture definitions are checked into the benchmark/research documentation path chosen for 0.14.0. | `docs/benchmarks/2026-07-07-issue-399-serde-fixtures.md` defines six versioned fixture IDs under `docs/benchmarks`. | PASS |
| Every scenario has metric direction and required raw output format. | The document defines metric direction, Go/Rust/JVM raw output requirements, and a scenario table with required metrics and raw output format per scenario. | PASS |
| Explicit exclusions are documented. | The `Exclusions` section rejects production rankings, symmetric-adapter churn, API churn before evidence, lossy conversion, unlabeled strict/lenient comparisons, and compressed-size-only recommendations. | PASS |
| Compatibility notes cover Go/Rust/JVM field naming, numeric precision, time values, and optional fields. | The `Fixture Rules` and `Compatibility Notes` sections define snake_case manifests, decimal text/minor units, UTC RFC3339 timestamps, and absent-vs-null handling. | PASS |

## P0/P1 Findings

P0=0 P1=0

No blocker findings. This is documentation-only benchmark planning evidence;
it does not change production code, public APIs, benchmark runners, or release
state.

## Validation

- `git diff --check`: PASS
- `rg -n "serde-small-object-v1|Scenario Matrix|Metric direction|Raw output requirement|Exclusions|#400|#401|#402|#403" docs/benchmarks/2026-07-07-issue-399-serde-fixtures.md`: PASS
- `rg -n "docs/benchmarks/2026-07-07-issue-399-serde-fixtures.md|compression_benchmark_test.go|serialization/README.md|codec/README.md" docs/lessons/2026-07-07-serde-benchmark-fixtures.md`: PASS

## Residual Risk

- The document intentionally defers runnable Go benchmark changes to #400 and
  raw evidence preservation to #401.
- Rust/JVM candidates are mapped from current local repository surfaces; later
  runner PRs must re-check each sibling repository before producing raw output.
