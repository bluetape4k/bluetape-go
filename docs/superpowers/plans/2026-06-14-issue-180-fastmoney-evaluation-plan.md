# Issue #180 FastMoney Evaluation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Measure whether the current decimal-backed `money.Money` API needs a separate public long-backed `FastMoney` type, then document the decision with raw benchmark output, a real chart, README guidance, and review evidence.

**Architecture:** Keep `Money` as the only public amount type during this issue. Add benchmark-only measurement code, preserve raw benchmark output, generate chart assets from the raw output, and use the measured threshold from the approved spec to either reject `FastMoney` for now or open a narrower follow-up issue.

**Tech Stack:** Go 1.26.3, `github.com/govalues/money`, repo-local `testing/concurrency.GoroutineStressTester`, Node.js chart generation, Graphviz/rsvg conversion assets, `$bluetape-go-patterns`, `$bluetape4k-diagram`.

---

## File Structure

- Create `money/money_benchmark_test.go`
  - Benchmark current `Money` minor-unit, arithmetic, parse, and JSON paths.
  - Include a direct `govalues/money` reference row for wrapper-overhead comparison only.
- Create `docs/research/outputs/issue-180/money-fastmoney-evaluation-bench.txt`
  - Store the raw `go test -bench` output.
- Create `docs/images/readme-charts/generate-money-fastmoney-evaluation-benchmark.mjs`
  - Parse raw benchmark output and render a horizontal-bar chart.
  - Write SVG, PNG, and Vega-Lite data-source JSON assets.
- Create `docs/images/readme-charts/money-fastmoney-evaluation-benchmark.vl.json`
  - Preserve chart input data in a machine-readable form.
- Create `docs/images/readme-charts/money-fastmoney-evaluation-benchmark.svg`
  - Final chart vector asset.
- Create `docs/images/readme-charts/money-fastmoney-evaluation-benchmark.png`
  - Final rendered chart asset for README embedding and visual inspection.
- Create `docs/research/2026-06-14-issue-180-fastmoney-evaluation.md`
  - Record benchmark environment, raw-output link, chart link, comparison, and decision.
- Create `docs/lessons/2026-06-14-issue-180-fastmoney-evaluation.md`
  - Record the reusable lesson: benchmark-first before duplicating money APIs.
- Modify `money/README.md`
  - Replace the deferred `FastMoney` row with measured decision text.
  - Add `Money vs FastMoney` note and chart link.
- Modify `money/README.ko.md`
  - Keep Korean README parity with the English README.
- Modify `CHANGELOG.md`
  - Add an Unreleased `money` bullet for FastMoney benchmark evaluation evidence if the file has an Unreleased section.
- Create review artifacts:
  - `docs/superpowers/reviews/2026-06-14-issue-180-fastmoney-evaluation-step-3r-plan-review.md`
  - `docs/superpowers/reviews/2026-06-14-issue-180-fastmoney-evaluation-step-6r-code-review.md`
  - `docs/superpowers/reviews/2026-06-14-issue-180-fastmoney-evaluation-step-7r-pr-review.md`

## Task 1: Add Benchmark Coverage

**Files:**
- Create: `money/money_benchmark_test.go`

- [ ] **Step 1: Add current API and direct-reference benchmark file**

Use this benchmark file:

```go
package money

import (
	"encoding/json"
	"testing"

	gmoney "github.com/govalues/money"
)

func BenchmarkMoneyNewMinorUSD(b *testing.B) {
	for i := 0; i < b.N; i++ {
		value, err := NewMinor(12345, USD)
		if err != nil {
			b.Fatal(err)
		}
		if value.IsZero() {
			b.Fatal("expected valid money")
		}
	}
}

func BenchmarkMoneyNewMinorJPY(b *testing.B) {
	for i := 0; i < b.N; i++ {
		value, err := NewMinor(12345, JPY)
		if err != nil {
			b.Fatal(err)
		}
		if value.IsZero() {
			b.Fatal("expected valid money")
		}
	}
}

func BenchmarkMoneyMinorUnitsUSD(b *testing.B) {
	value, err := New("123.45", USD)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		units, err := value.MinorUnits()
		if err != nil {
			b.Fatal(err)
		}
		if units != 12345 {
			b.Fatalf("units = %d", units)
		}
	}
}

func BenchmarkMoneyAddUSD(b *testing.B) {
	left, err := New("123.45", USD)
	if err != nil {
		b.Fatal(err)
	}
	right, err := New("67.89", USD)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		value, err := left.Add(right)
		if err != nil {
			b.Fatal(err)
		}
		if value.IsZero() {
			b.Fatal("expected valid sum")
		}
	}
}

func BenchmarkMoneySumUSD10(b *testing.B) {
	values := make([]Money, 10)
	for i := range values {
		value, err := NewMinor(int64(1000+i), USD)
		if err != nil {
			b.Fatal(err)
		}
		values[i] = value
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		value, err := Sum(USD, values...)
		if err != nil {
			b.Fatal(err)
		}
		if value.IsZero() {
			b.Fatal("expected valid total")
		}
	}
}

func BenchmarkMoneyParseUSD(b *testing.B) {
	for i := 0; i < b.N; i++ {
		value, err := Parse("USD 123.45")
		if err != nil {
			b.Fatal(err)
		}
		if value.IsZero() {
			b.Fatal("expected valid money")
		}
	}
}

func BenchmarkMoneyMarshalJSON(b *testing.B) {
	value, err := New("123.45", USD)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		payload, err := json.Marshal(value)
		if err != nil {
			b.Fatal(err)
		}
		if len(payload) == 0 {
			b.Fatal("expected payload")
		}
	}
}

func BenchmarkMoneyDirectGovaluesNewAmountFromMinorUnits(b *testing.B) {
	for i := 0; i < b.N; i++ {
		value, err := gmoney.NewAmountFromMinorUnits("USD", 12345)
		if err != nil {
			b.Fatal(err)
		}
		if value.IsZero() {
			b.Fatal("expected valid amount")
		}
	}
}
```

- [ ] **Step 2: Run the targeted benchmark compile gate**

Run:

```bash
go test -count=1 ./money -run '^$'
```

Expected result:

```text
ok  	github.com/bluetape4k/bluetape-go/money
```

## Task 2: Capture Raw Benchmark Evidence

**Files:**
- Create directory: `docs/research/outputs/issue-180/`
- Create: `docs/research/outputs/issue-180/money-fastmoney-evaluation-bench.txt`

- [ ] **Step 1: Capture environment metadata**

Run these commands and include their output at the top of the raw evidence file:

```bash
git rev-parse --short HEAD
git status --short
go version
go env GOOS GOARCH
```

If `git status --short` is not empty because the benchmark file is uncommitted, record the dirty files in the raw evidence file. Do not hide the dirty state.

- [ ] **Step 2: Run and tee the benchmark output**

Run:

```bash
go test -run '^$' -bench '^BenchmarkMoney' -benchmem ./money | tee -a docs/research/outputs/issue-180/money-fastmoney-evaluation-bench.txt
```

Expected result includes all benchmark rows:

```text
BenchmarkMoneyNewMinorUSD
BenchmarkMoneyNewMinorJPY
BenchmarkMoneyMinorUnitsUSD
BenchmarkMoneyAddUSD
BenchmarkMoneySumUSD10
BenchmarkMoneyParseUSD
BenchmarkMoneyMarshalJSON
BenchmarkMoneyDirectGovaluesNewAmountFromMinorUnits
```

- [ ] **Step 3: Interpret the threshold from the spec**

Use these rules from the approved spec:

- `NewMinor`, `MinorUnits`, `Add`, or `Sum` at least 3x slower than the direct `govalues` reference for the same operation family means a follow-up spike is justified.
- The simplest minor-unit path above 5 allocs/op while the direct reference is near zero means a follow-up spike is justified.
- A caller workflow requiring long-backed minor-unit storage as a public type boundary means a follow-up issue is justified.
- If none of these conditions is true, reject a public `FastMoney` type for now and keep `Money` as the public API.

## Task 3: Generate Benchmark Chart Assets

**Files:**
- Create: `docs/images/readme-charts/generate-money-fastmoney-evaluation-benchmark.mjs`
- Create: `docs/images/readme-charts/money-fastmoney-evaluation-benchmark.vl.json`
- Create: `docs/images/readme-charts/money-fastmoney-evaluation-benchmark.svg`
- Create: `docs/images/readme-charts/money-fastmoney-evaluation-benchmark.png`

- [ ] **Step 1: Add raw-output parser**

Use this parsing contract in the generator:

```js
const benchmarkPattern = /^BenchmarkMoney([A-Za-z0-9]+)-\d+\s+\d+\s+([\d.]+)\s+ns\/op\s+([\d.]+)\s+B\/op\s+([\d.]+)\s+allocs\/op$/;

function parseBenchmarks(text) {
  return text
    .split(/\r?\n/)
    .map((line) => benchmarkPattern.exec(line.trim()))
    .filter(Boolean)
    .map((match) => ({
      id: match[1],
      label: labelFor(match[1]),
      group: match[1].startsWith("DirectGovalues") ? "Reference" : "Money",
      nsPerOp: Number(match[2]),
      bytesPerOp: Number(match[3]),
      allocsPerOp: Number(match[4]),
    }));
}
```

Use these labels:

```js
const labels = new Map([
  ["NewMinorUSD", "NewMinor USD"],
  ["NewMinorJPY", "NewMinor JPY"],
  ["MinorUnitsUSD", "MinorUnits USD"],
  ["AddUSD", "Add USD"],
  ["SumUSD10", "Sum USD x10"],
  ["ParseUSD", "Parse USD text"],
  ["MarshalJSON", "Marshal JSON"],
  ["DirectGovaluesNewAmountFromMinorUnits", "Direct govalues minor"],
]);
```

- [ ] **Step 2: Render a real chart**

The SVG must be a horizontal-bar chart with three panels:

- `Latency (ns/op) - lower is better`
- `Heap bytes (B/op) - lower is better`
- `Allocations (allocs/op) - lower is better`

Visual requirements:

- Bars, not a table or heatmap.
- Distinct color for `Money` and `Reference`.
- Numeric values at the end of each bar.
- Bottom interpretation band with commit, Go version, GOOS/GOARCH, CPU line, and the local-snapshot caveat.
- `Architects Daughter` for title and row labels.
- `Comic Mono` for metric labels, values, caveats, and footer text.
- Chart height must leave at least 48 px bottom padding after the bottom band.
- Chart width must be at least 1600 px so labels do not overflow.

- [ ] **Step 3: Write Vega-Lite data-source JSON**

The `.vl.json` file must include the parsed benchmark rows and environment metadata:

```json
{
  "title": "Money FastMoney Evaluation Benchmark",
  "source": "docs/research/outputs/issue-180/money-fastmoney-evaluation-bench.txt",
  "rows": []
}
```

Replace `rows` with the actual parsed benchmark rows when the generator runs.

- [ ] **Step 4: Render PNG and validate assets**

Run:

```bash
node docs/images/readme-charts/generate-money-fastmoney-evaluation-benchmark.mjs
xmllint --noout docs/images/readme-charts/money-fastmoney-evaluation-benchmark.svg
file docs/images/readme-charts/money-fastmoney-evaluation-benchmark.png
```

Expected `file` result:

```text
docs/images/readme-charts/money-fastmoney-evaluation-benchmark.png: PNG image data
```

- [ ] **Step 5: Inspect rendered PNG**

Open the rendered PNG with `view_image` and confirm:

- It is a real bar chart.
- Labels do not overflow.
- Bottom interpretation band does not overlap the bars or footer.
- Numeric values are readable without relying on the README table.

## Task 4: Write Research And Lesson Artifacts

**Files:**
- Create: `docs/research/2026-06-14-issue-180-fastmoney-evaluation.md`
- Create: `docs/lessons/2026-06-14-issue-180-fastmoney-evaluation.md`

- [ ] **Step 1: Write the research note**

Use this section structure:

```md
# Issue #180 FastMoney Evaluation

## Decision

## Benchmark Environment

## Benchmark Chart

## Raw Benchmark Output

## Results

## Interpretation

## Comparison

## Follow-Up
```

The comparison section must cover:

- JVM `FastMoneySupport.kt`: long-backed Moneta helper intent.
- Current Go `Money`: decimal-backed immutable wrapper with `NewMinor` and `MinorUnits`.
- Direct `govalues/money`: benchmark reference only, not a public API recommendation.
- `Rhymond/go-money`: active MIT Fowler-style integer minor-unit alternative with a narrower decimal/exchange-rate surface.

The decision section must match the benchmark threshold:

- If the threshold is not crossed, write that `FastMoney` is rejected for now.
- If the threshold is crossed, write that #180 does not add a public type and create a follow-up issue with the specific operation family and measured gap.

- [ ] **Step 2: Write the lesson note**

Use this section structure:

```md
# Lesson: Benchmark Before Duplicating Money APIs

## Context

## Lesson

## Apply Next Time

## Evidence
```

The lesson must state that performance-motivated public API duplication needs measured hot-path evidence plus a caller contract that cannot be served by the existing type.

## Task 5: Update README Pair

**Files:**
- Modify: `money/README.md`
- Modify: `money/README.ko.md`

- [ ] **Step 1: Replace the deferred selection-guide row**

English row when the threshold is not crossed:

```md
| Long-backed FastMoney | Not added | #180 benchmark evidence keeps `Money` as the public API; use `NewMinor` and `MinorUnits` for minor-unit paths. |
```

Korean row when the threshold is not crossed:

```md
| Long-backed FastMoney | 추가하지 않음 | #180 benchmark 근거에 따라 `Money`를 public API로 유지합니다. minor-unit 경로는 `NewMinor`와 `MinorUnits`를 사용하십시오. |
```

If the threshold is crossed, replace `Not added` / `추가하지 않음` with follow-up wording that names the new GitHub issue and does not claim `FastMoney` exists in this PR.

- [ ] **Step 2: Add English decision note**

Add this section near the selection guide:

```md
## Money vs FastMoney

`Money` remains the public amount type. #180 measured the minor-unit and
representative hot paths and did not add a separate long-backed `FastMoney`
type. Use `NewMinor` for integer minor-unit input and `MinorUnits` for
integer extraction.

![money FastMoney evaluation benchmark](../docs/images/readme-charts/money-fastmoney-evaluation-benchmark.png)

The benchmark snapshot is local evidence, not a production ranking. The raw
output is stored in `docs/research/outputs/issue-180/money-fastmoney-evaluation-bench.txt`.
```

If the threshold is crossed, adjust the second sentence to say that the benchmark justifies a follow-up issue rather than saying no separate type was added.

- [ ] **Step 3: Add Korean decision note**

Add this section near the selection guide:

```md
## Money vs FastMoney

`Money`를 public 금액 타입으로 유지합니다. #180은 minor-unit 및 대표 hot path를
측정했고 별도 long-backed `FastMoney` 타입을 추가하지 않았습니다. 정수 minor-unit
입력은 `NewMinor`, 정수 추출은 `MinorUnits`를 사용하십시오.

![money FastMoney evaluation benchmark](../docs/images/readme-charts/money-fastmoney-evaluation-benchmark.png)

이 benchmark snapshot은 local 비교 근거이며 production ranking이 아닙니다. Raw output은
`docs/research/outputs/issue-180/money-fastmoney-evaluation-bench.txt`에 보관합니다.
```

If the threshold is crossed, adjust the second sentence to say that the benchmark justifies a follow-up issue rather than saying no separate type was added.

## Task 6: Re-run Diagram And Chart Gates

**Files:**
- Existing: `scripts/generate-money-fastmoney-evaluation-diagram.mjs`
- Existing: `docs/images/readme-diagrams/money-fastmoney-evaluation-decision-flow.svg`
- Existing: `docs/images/readme-diagrams/money-fastmoney-evaluation-decision-flow.png`
- New: chart assets from Task 3

- [ ] **Step 1: Re-run the approved decision diagram**

Run:

```bash
node scripts/generate-money-fastmoney-evaluation-diagram.mjs
xmllint --noout docs/images/readme-diagrams/money-fastmoney-evaluation-decision-flow.svg
file docs/images/readme-diagrams/money-fastmoney-evaluation-decision-flow.png
```

Expected generator evidence includes:

```text
badEndpointAngle=0 badBends=0 interiorCrossings=0 nodeOverlaps=0 laneClearance=0
```

- [ ] **Step 2: Re-run benchmark chart generator**

Run:

```bash
node docs/images/readme-charts/generate-money-fastmoney-evaluation-benchmark.mjs
xmllint --noout docs/images/readme-charts/money-fastmoney-evaluation-benchmark.svg
file docs/images/readme-charts/money-fastmoney-evaluation-benchmark.png
```

- [ ] **Step 3: Inspect both PNGs**

Use `view_image` for:

- `docs/images/readme-diagrams/money-fastmoney-evaluation-decision-flow.png`
- `docs/images/readme-charts/money-fastmoney-evaluation-benchmark.png`

Confirm no visible text overflow, no chart/table confusion, and no excessive bottom whitespace.

## Task 7: Run Tests, Race, Stress, Lint, And CI

**Files:**
- Read existing: `money/money_concurrency_test.go`
- Read existing: `testing/concurrency/*`

- [ ] **Step 1: Run targeted package tests**

Run:

```bash
go test -count=1 ./money ./testing/concurrency
```

Expected result:

```text
ok  	github.com/bluetape4k/bluetape-go/money
ok  	github.com/bluetape4k/bluetape-go/testing/concurrency
```

- [ ] **Step 2: Run mandatory GoroutineStressTester gate**

Run:

```bash
go test -count=1 ./money -run 'TestMoneyOperationsUseGoroutineStressTester'
```

Expected result:

```text
ok  	github.com/bluetape4k/bluetape-go/money
```

- [ ] **Step 3: Run race gate**

Run:

```bash
go test -race -count=1 ./money ./testing/concurrency
```

Expected result:

```text
ok  	github.com/bluetape4k/bluetape-go/money
ok  	github.com/bluetape4k/bluetape-go/testing/concurrency
```

- [ ] **Step 4: Run benchmark gate**

Run:

```bash
go test -run '^$' -bench '^BenchmarkMoney' -benchmem ./money
```

Expected result includes `ns/op`, `B/op`, and `allocs/op` for every benchmark row.

- [ ] **Step 5: Run repo quality gates**

Run:

```bash
git diff --check
make fmt-check
make tidy-check
make vet
make lint
make ci
```

If `make lint` or `make ci` fails because `golangci-lint` is missing or a known local toolchain issue appears, record the exact failure in the PR DoD and run the next-best `go test ./...` gate.

## Task 8: Step 6-R Main-Session 7-Tier Implementation Review

**Files:**
- Create: `docs/superpowers/reviews/2026-06-14-issue-180-fastmoney-evaluation-step-6r-code-review.md`

- [ ] **Step 1: Review six independent lenses**

Because native subagents are unstable in this session, perform main-session role switching and write that fallback explicitly in the review artifact.

Use these lanes:

- Performance: benchmark rows, threshold math, raw output, chart readability.
- Stability: no public API mutation, goroutine stress, race results.
- Security: no hidden IO, no parser/deserialization expansion beyond existing API.
- Operator/Ops: reproducible environment metadata, raw output, chart generator command.
- Developer/API: no premature `FastMoney`, direct upstream row marked reference-only.
- User/Caller: README pair, chart guidance, local-snapshot caveat.
- Main integration: deduplicate findings and require P0=0/P1=0.

- [ ] **Step 2: Gate findings**

The review must end with:

```text
P0=0 P1=0
Step 6-R verdict: PASS.
```

If P0/P1 exists, fix it before PR.

## Task 9: Commit, Push, PR, And Step 7-R PR Review

**Files:**
- Create: `docs/superpowers/reviews/2026-06-14-issue-180-fastmoney-evaluation-step-7r-pr-review.md`

- [ ] **Step 1: Commit implementation with Lore protocol**

Use a commit message with this shape:

```text
Evaluate FastMoney need with benchmark evidence

Constraint: #180 is research-first and blocks public FastMoney without measured need.
Rejected: public FastMoney in this PR | existing Money minor-unit paths must be measured first.
Confidence: high
Scope-risk: narrow
Directive: do not add a parallel money type unless a future issue records benchmark gap and caller contract.
Tested: go test -count=1 ./money ./testing/concurrency; go test -race -count=1 ./money ./testing/concurrency; go test -run '^$' -bench '^BenchmarkMoney' -benchmem ./money; make ci
Not-tested: production workload benchmark outside local machine.
```

- [ ] **Step 2: Push branch**

Run:

```bash
git push -u origin issue-180-fastmoney-evaluation
```

- [ ] **Step 3: Create PR with body file**

The PR body must include:

- Issue link: `Closes #180`
- Benchmark command and raw output path.
- Chart asset links.
- Decision summary.
- 7-Tier Step 6-R summary.
- Final `##` heading exactly named `## DoD Status`.

After creating or editing the PR, verify the live PR body:

```bash
gh pr view --json number,url,body
```

- [ ] **Step 4: Run Step 7-R PR review**

Write `docs/superpowers/reviews/2026-06-14-issue-180-fastmoney-evaluation-step-7r-pr-review.md` with the same six lanes plus main integration.

Review live PR state:

```bash
gh pr checks --watch=false
gh pr view --json number,url,body,mergeStateStatus,reviewDecision
```

The Step 7-R review must end with:

```text
P0=0 P1=0
Step 7-R verdict: PASS.
```

Stop after PR creation and review. Wait for explicit merge approval before merging.

## Execution Order

1. Task 1: benchmark file.
2. Task 2: raw benchmark evidence and threshold interpretation.
3. Task 3: chart generator and real chart assets.
4. Task 4: research and lesson artifacts.
5. Task 5: README pair.
6. Task 6: diagram/chart visual gates.
7. Task 7: tests, race, stress, lint, CI.
8. Task 8: Step 6-R implementation review.
9. Task 9: commit, PR, Step 7-R review, then stop for merge approval.

## Plan Self-Review Checklist

- The plan creates no public `FastMoney` type in this issue.
- The plan preserves raw benchmark output before interpreting it.
- The plan requires a real horizontal-bar chart and rendered PNG inspection.
- The plan updates English and Korean README files together.
- The plan requires `GoroutineStressTester` and `go test -race`.
- The plan repeats the fixed 7-Tier shape: six independent lenses plus main integration.
- The plan stops at PR review and waits for explicit merge approval.
