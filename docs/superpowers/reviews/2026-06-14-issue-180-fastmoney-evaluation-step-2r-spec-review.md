# Issue #180 FastMoney Evaluation Step 2-R Spec Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #180
Spec: `docs/superpowers/specs/2026-06-14-issue-180-fastmoney-evaluation-design.md`
게이트: Step 2-R, 7-Tier spec/design review
Method: main-session role switching. Native subagents were not used in this session because prior lane waits have been unreliable; the main integration review performed the required six independent lenses plus synthesis.

## 검토 범위

- `docs/superpowers/specs/2026-06-14-issue-180-fastmoney-evaluation-design.md`
- `scripts/generate-money-fastmoney-evaluation-diagram.mjs`
- `docs/images/readme-diagrams/money-fastmoney-evaluation-decision-flow.{dot,plain,svg,png}`
- `docs/images/readme-diagrams/money-fastmoney-evaluation-decision-flow-graphviz.{svg,png}`

## 증거

| Check | Evidence | Status |
|---|---|---|
| Live issue scope | #180 is `type: research`, asks for benchmark evidence before public `FastMoney`. | PASS |
| Parent issue continuity | #35 spec deferred `FastMoney`; current `Money` already has `NewMinor` and `MinorUnits`. | PASS |
| Diagram gate | `node scripts/generate-money-fastmoney-evaluation-diagram.mjs` printed `nodes=7 routes=7 segments=19 badEndpointAngle=0 badBends=0 interiorCrossings=0 nodeOverlaps=0 laneClearance=0 margins=L48/R48/T48/B48 titleGap=74`. | PASS |
| Diagram XML/PNG | `xmllint --noout ...decision-flow.svg ...decision-flow-graphviz.svg`; `file ...decision-flow.png` reports `PNG image data, 1880 x 1080`. | PASS |
| Visual inspection | Rendered PNG inspected; footer text overlap was found, fixed, rerendered, and reinspected. | PASS |
| Placeholder scan | `rg -n 'TBD|TODO|implement later|fill in details|Similar to|appropriate|add validation|handle edge cases|Write tests for the above' ...` returned no hits. | PASS |
| Whitespace | `git diff --check` passed. | PASS |

## 6개 검토 관점

| Lane | P0 | P1 | P2 | P3 | Verdict | Evidence |
|---|---:|---:|---:|---:|---|---|
| Performance | 0 | 0 | 0 | 0 | PASS | Spec defines `BenchmarkMoney*` scope, direct `govalues` reference row, `ns/op`, `B/op`, `allocs/op`, raw output path, chart requirement, and a concrete 3x / 5 allocs/op / caller-use threshold. |
| Stability | 0 | 0 | 0 | 0 | PASS | Existing `TestMoneyOperationsUseGoroutineStressTester` and `go test -race -count=1 ./money ./testing/concurrency` are mandatory; no new shared-state or goroutine lifecycle behavior is introduced. |
| Security | 0 | 0 | 0 | 0 | PASS | Scope is benchmark/docs only. Spec does not add hidden IO, provider behavior, untrusted parser expansion, or public deserialization behavior. |
| Operator/Ops | 0 | 0 | 0 | 0 | PASS | Spec requires run conditions, dirty-tree state, commit, raw output preservation, and caveats that local benchmark snapshots are not production rankings. |
| Developer/API | 0 | 0 | 0 | 0 | PASS | Public `FastMoney` is rejected unless measured need plus caller use case justify the duplicate API surface; direct upstream benchmark row is explicitly not a public API recommendation. |
| User/Caller | 0 | 0 | 0 | 0 | PASS | README pair must replace the deferred row with a measured decision and explain when `Money` is enough. Chart must be a real horizontal bar chart, not a heatmap/table. |

## Initial Findings And Fixes

| Severity | Finding | Fix | Status |
|---|---|---|---|
| P2 | "Meaningful hot-path gap" was subjective and could let future work argue either way. | Added concrete decision thresholds: 3x slower operation family, >5 allocs/op on simplest minor path, or documented caller workflow needing long-backed type boundary. | FIXED |
| P3 | First rendered diagram footer text overlapped. | Removed duplicate footer node rendering, rerendered, reinspected final PNG. | FIXED |

## 메인 통합 검토

- P0 findings: 0
- P1 findings: 0
- P2 findings: 0 remaining
- P3 findings: 0 remaining

The spec is coherent with #180: measure current `Money` first, preserve benchmark evidence, include real chart assets, and keep public `FastMoney` out of scope unless evidence crosses the documented threshold.

## 판정

P0=0 P1=0

Step 2-R verdict: PASS.
