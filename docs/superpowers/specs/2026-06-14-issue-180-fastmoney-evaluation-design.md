# Issue #180 FastMoney Evaluation Design

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.


## 목표

`money` package에 별도 long-backed `FastMoney` public type이 필요한지
벤치마크와 사용 사례 근거로 판정한다. 이 작업의 기본 방향은 새 public type을
먼저 만들지 않고, #35에서 추가된 decimal-backed `Money`의 minor-unit 경로가
실제 hot path 요구를 충분히 만족하는지 측정하는 것이다.

## Source Evidence

- GitHub issue: #180 `Evaluate long-backed FastMoney for money`
- Parent issue: #35 `Port money and decimal helpers`
- Current package:
  - `money/money.go`
  - `money/money_test.go`
  - `money/money_concurrency_test.go`
  - `money/README.md`
  - `money/README.ko.md`
- Existing #35 artifacts:
  - `docs/superpowers/specs/2026-06-09-issue-35-money-decimal-spec.md`
  - `docs/superpowers/plans/2026-06-09-issue-35-money-decimal-plan.md`
  - `docs/superpowers/reviews/2026-06-09-issue-35-money-code-review.md`
- JVM reference:
  - `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/money/src/main/kotlin/io/bluetape4k/money/FastMoneySupport.kt`
  - `/Users/debop/work/bluetape4k/bluetape4k-projects/utils/money/README.md`
- Go dependency evidence:
  - `go doc github.com/govalues/money Amount` confirms `Amount` is immutable and safe for concurrent goroutine use.
  - `go doc github.com/govalues/money NewAmountFromMinorUnits` confirms upstream minor-unit conversion support.
  - `gh repo view Rhymond/go-money --json ...` confirms `Rhymond/go-money` remains an active MIT Fowler-style integer minor-unit package.

## Decision Diagram

![money FastMoney evaluation decision flow](../../images/readme-diagrams/money-fastmoney-evaluation-decision-flow.png)

Diagram assets:

- Generator: `scripts/generate-money-fastmoney-evaluation-diagram.mjs`
- Final assets:
  - `docs/images/readme-diagrams/money-fastmoney-evaluation-decision-flow.svg`
  - `docs/images/readme-diagrams/money-fastmoney-evaluation-decision-flow.png`
- Graphviz evidence:
  - `docs/images/readme-diagrams/money-fastmoney-evaluation-decision-flow.dot`
  - `docs/images/readme-diagrams/money-fastmoney-evaluation-decision-flow.plain`
  - `docs/images/readme-diagrams/money-fastmoney-evaluation-decision-flow-graphviz.svg`
  - `docs/images/readme-diagrams/money-fastmoney-evaluation-decision-flow-graphviz.png`
- Geometry gate:
  - `nodes=7 routes=7 segments=19`
  - `badEndpointAngle=0 badBends=0 interiorCrossings=0 nodeOverlaps=0 laneClearance=0`
  - `margins=L48/R48/T48/B48 titleGap=74`
- Visual gate: rendered PNG inspected. Title/subtitle, decision cards, footer evidence band, and route lanes have no visible overlap.

## Chosen Approach

Approach 1: benchmark-first decision.

Implement a focused benchmark suite for current `Money` paths and write a
research conclusion that either rejects a public `FastMoney` type for now or
opens a narrower follow-up if measured evidence proves the need.

This is the default because:

- `Money` already supports `NewMinor` and `MinorUnits`, which covers the most important `FastMoney` input/output use case.
- `govalues/money.Amount` already provides decimal-backed arithmetic, currency metadata, serialization-friendly values, and goroutine-safe immutable values.
- A public `FastMoney` type would duplicate arithmetic, parsing, serialization, exchange-rate, README, examples, and error contracts.
- #180 is labeled `type: research`; the acceptance criteria ask for benchmark evidence before adding a new public type.

## Rejected Approaches

### Approach 2: Internal FastMoney Prototype

Add an unexported long-backed prototype only for benchmarks and compare it with
current `Money`.

Rejected for the first pass because it risks spending implementation effort on
a throwaway type before measuring whether current `Money` is insufficient. If
the baseline benchmark shows a real latency/allocation blocker, this approach
can become a follow-up spike.

### Approach 3: Public FastMoney Type Now

Add `type FastMoney` with constructors, arithmetic, serialization, parsing,
README guidance, examples, and exchange-rate behavior.

Rejected because it creates a parallel API surface without measured need. The
Go package should not mechanically copy the JVM Moneta surface. Public API
duplication is only justified if the benchmark and caller-use evidence show a
clear gap that cannot be solved by documenting `Money` minor-unit paths.

## Benchmark Scope

Add `money/money_benchmark_test.go` with table-backed benchmarks for the current
public API:

- `BenchmarkMoneyNewMinorUSD`
- `BenchmarkMoneyNewMinorJPY`
- `BenchmarkMoneyMinorUnitsUSD`
- `BenchmarkMoneyAddUSD`
- `BenchmarkMoneySumUSD10`
- `BenchmarkMoneyParseUSD`
- `BenchmarkMoneyMarshalJSON`
- `BenchmarkMoneyDirectGovaluesNewAmountFromMinorUnits`

The direct `govalues` benchmark is a reference row for wrapper overhead only.
It is not a public API recommendation.

Benchmark command:

```bash
go test -run '^$' -bench '^BenchmarkMoney' -benchmem ./money
```

Raw output path:

```text
docs/research/outputs/issue-180/money-fastmoney-evaluation-bench.txt
```

Benchmark interpretation rules:

- Treat short local runs as comparable snapshots, not production rankings.
- Report `go version`, `GOOS`, `GOARCH`, CPU, dirty-tree state, command, benchtime, and package commit.
- Preserve raw benchmark output as the numeric source of truth.
- The README/research table must say `lower ns/op is better`, `lower B/op is better`, and `lower allocs/op is better`.
- A public `FastMoney` type is rejected for now unless the benchmark shows a meaningful hot-path gap and the research note identifies a caller use case that needs a distinct long-backed public contract.

Meaningful hot-path gap for this issue:

- `NewMinor`, `MinorUnits`, `Add`, or `Sum` is at least 3x slower than the direct `govalues` reference row for the same operation family, or
- the simplest minor-unit path allocates more than 5 objects/op while the reference path stays near zero, or
- a documented caller workflow needs long-backed minor-unit storage as a public type boundary rather than only faster construction/extraction.

If none of those conditions is met, #180 should explicitly reject `FastMoney`
for now and document the rejection in the README pair.

## Chart Requirement

Benchmark evidence must include a real chart, not just a Markdown table.

Required chart assets:

- `docs/images/readme-charts/money-fastmoney-evaluation-benchmark.svg`
- `docs/images/readme-charts/money-fastmoney-evaluation-benchmark.png`

Chart requirements:

- Use horizontal bars for benchmark rows.
- Show separate panels or split scales for `ns/op`, `B/op`, and `allocs/op`.
- Label each metric with `lower is better`.
- Keep the raw benchmark output as the numeric source of truth.
- Include a bottom interpretation band with run conditions and caveat.
- Use `Architects Daughter` for titles/row labels and `Comic Mono` for values, axes, caveats, and footer text.
- Render and inspect PNG before PR. A table-like heatmap or numeric grid is not accepted as the chart.

## Documentation Scope

Update `money/README.md` and `money/README.ko.md`:

- Replace the current `Long-backed FastMoney | Deferred | Benchmark-driven evaluation is tracked in #180` row with the measured decision.
- Add a short `Money vs FastMoney` decision note:
  - Use `Money` for deterministic decimal-backed amounts and minor-unit input/output.
  - A separate `FastMoney` remains unnecessary unless a future issue records measured hot-path need and public API semantics.
- Include or link the benchmark table and chart near the decision note.
- Keep README surrounding prose localized; shared diagram/chart assets remain English.

Add research and lesson artifacts:

- `docs/research/2026-06-14-issue-180-fastmoney-evaluation.md`
- `docs/lessons/2026-06-14-issue-180-fastmoney-evaluation.md`

## Testing And Validation

Baseline and implementation validation:

```bash
go test -count=1 ./money ./testing/concurrency
go test -count=1 ./money -run 'TestMoneyOperationsUseGoroutineStressTester'
go test -race -count=1 ./money ./testing/concurrency
go test -run '^$' -bench '^BenchmarkMoney' -benchmem ./money
node scripts/generate-money-fastmoney-evaluation-diagram.mjs
node docs/images/readme-charts/generate-money-fastmoney-evaluation-benchmark.mjs
xmllint --noout docs/images/readme-diagrams/money-fastmoney-evaluation-decision-flow.svg
xmllint --noout docs/images/readme-charts/money-fastmoney-evaluation-benchmark.svg
git diff --check
make fmt-check
make tidy-check
make vet
make lint
make ci
```

Goroutine stress gate:

- Existing `TestMoneyOperationsUseGoroutineStressTester` remains mandatory evidence because #180 touches the public `Money` performance story and immutable value contract.
- If benchmark or documentation changes introduce a new goroutine-safe claim, add or extend `GoroutineStressTester` coverage before PR.
- `AsyncJobTester` is not applicable unless this issue adds a context-aware provider, IO, or asynchronous API. It should not add those APIs.

## Review Gates

Use main-session 7-Tier fallback for this session because native subagents have
been unreliable in the current Codex session. The review shape is still fixed:
six independent lenses plus main integration.

Step 2-R spec review lenses:

- Performance: benchmark scope, chart shape, decision threshold.
- Stability: goroutine stress/race validation and no new shared-state risk.
- Security: no untrusted parsing expansion or hidden external IO.
- Operator/Ops: benchmark run conditions, reproducibility, raw output preservation.
- Developer/API: no premature public `FastMoney` type, Go-shaped API boundary.
- User/caller: README decision clarity and Money-vs-FastMoney guidance.
- Main integration: deduplicate findings, enforce P0/P1 = 0.

Step 3-R plan review and Step 6-R implementation review use the same six
lenses plus main integration. Step 7-R PR review repeats the same gate on the
PR body, docs, chart, benchmark evidence, and CI state.

## Acceptance Criteria

- `money` has benchmark coverage for current minor-unit and representative hot paths.
- Raw benchmark output is stored under `docs/research/outputs/issue-180/`.
- A real benchmark chart PNG/SVG is generated and visually inspected.
- Research note compares JVM FastMoney intent, current Go `Money`, direct `govalues` reference, and Go integer-minor-unit alternatives.
- README pair explains whether `FastMoney` remains unnecessary or why a follow-up public type is justified.
- Goroutine stress and race evidence are reported.
- 7-Tier review evidence records P0=0 and P1=0 before PR.

## 위험 And Mitigations

| Risk | Severity | Mitigation |
|---|---:|---|
| Benchmark noise is overinterpreted as production ranking. | P1 | Label the run as a local snapshot and preserve raw output, run conditions, and caveats. |
| A table is mistaken for a chart. | P1 | Require horizontal bar chart PNG/SVG with metric direction and rendered PNG inspection. |
| Public `FastMoney` duplicates `Money` semantics without need. | P1 | Block public type unless benchmark gap plus caller use case are both documented. |
| Wrapper-overhead benchmark encourages upstream type leakage. | P2 | Mark direct `govalues` row as reference only, not a public API recommendation. |
| README drift between English and Korean docs. | P1 | Update `money/README.md` and `money/README.ko.md` together. |
| Existing goroutine-safe claim is not revalidated. | P1 | Run `TestMoneyOperationsUseGoroutineStressTester` and `go test -race`. |

## Spec Self-Review

- Placeholder scan: PASS. No unresolved placeholder remains.
- Internal consistency: PASS. Approach, benchmark scope, chart requirement, docs, and review gates all use benchmark-first decision.
- Scope check: PASS. This is one focused `money` package research/benchmark/docs change, not a new public type implementation.
- Ambiguity check: PASS. Public `FastMoney` is explicitly rejected unless measured need plus caller-use evidence justify it.
