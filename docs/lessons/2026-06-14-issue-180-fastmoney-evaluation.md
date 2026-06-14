# Lesson: Benchmark Before Duplicating Money APIs

## Context

Issue #180 evaluated whether bluetape-go should add a public long-backed
`FastMoney` type similar in spirit to the JVM money helper surface.

The existing Go package already had a decimal-backed immutable `Money` type plus
minor-unit input and output helpers:

- `NewMinor`
- `MinorUnits`

## Lesson

Performance-motivated public API duplication needs two pieces of evidence:

- measured hot-path evidence showing that the existing API is not good enough;
- a caller contract showing that the existing type cannot express the required
  boundary.

For #180, the benchmark showed that current minor-unit and arithmetic paths are
already near the direct `govalues/money` reference and allocate zero objects per
operation. A separate public `FastMoney` type would duplicate construction,
arithmetic, parsing, serialization, exchange-rate, README, example, and error
contracts without enough evidence.

## Apply Next Time

- Measure current wrapper cost before adding a performance-shaped public type.
- Preserve raw benchmark output before producing tables or charts.
- Use a real chart for benchmark-heavy decisions so reviewers can see scale and
  direction without reading only numbers.
- Keep direct dependency benchmark rows as reference data, not as public API
  recommendations.
- Require a follow-up issue for public API expansion when the benchmark crosses
  threshold, instead of expanding scope inside the research issue.

## Evidence

- Raw output: `docs/research/outputs/issue-180/money-fastmoney-evaluation-bench.txt`
- Chart: `docs/images/readme-charts/money-fastmoney-evaluation-benchmark.png`
- Decision note: `docs/research/2026-06-14-issue-180-fastmoney-evaluation.md`
