# Issue #433 GraphML Research Review

Issue: [#433](https://github.com/bluetape4k/bluetape-go/issues/433)
Branch: `feat/issue-433-graphml-research`
Baseline: `6f30d20`
Date: 2026-07-09

## Scope

- GraphML research decision note.
- `graph` and `graph/graphio` README capability wording.
- Research index entry.
- Lesson artifact.

No production Go code or public API behavior changes are in scope.

## Evidence

- Current `graphio` docs already defer GraphML until NDJSON/CSV adoption
  evidence exists.
- GNO/repo search found prior graph scope and boundary lessons that defer
  GraphML because XML compatibility raises maintenance cost.
- Live GitHub lookup found `bluetape-go-workshop` issue #52 still accepts CSV,
  NDJSON, or GraphML-style fixtures and does not require GraphML.
- External producer docs show incompatible or limited GraphML subsets across
  NetworkX, Gephi, Neo4j APOC, and yWorks/yFiles.
- `bluetape4k-graph` history shows GraphML needed explicit compatibility
  fixtures and typed-value error handling before compatibility claims were safe.

## Lens Check

| Lens | Verdict | Evidence |
|---|---|---|
| Performance | PASS | P0=0 P1=0. Decision avoids adding XML parse cost to the current hot path. |
| Stability | PASS | P0=0 P1=0. No production code changed; future gate requires bounded decoding and fixtures. |
| Security | PASS | P0=0 P1=0. Research calls out untrusted XML input and rejects implicit parser adoption. |
| Operator/Ops | PASS | P0=0 P1=0. No new dependency, container, CI, or runtime requirement added. |
| Developer/API | PASS | P0=0 P1=0. Existing `graphio` NDJSON/CSV API boundary is preserved. |
| User/Caller | PASS | P0=0 P1=0. README links a concrete decision instead of overpromising GraphML support. |
| Integration | PASS | P0=0 P1=0. Main-session review accepts defer decision and follow-up gates. |

## Validation

| Command | Status | Evidence |
|---|---|---|
| `rg -n "GraphML\\|graphio\\|issue-433" README.md README.ko.md graph docs/lessons docs/research` | PASS | Current and updated GraphML references reviewed. |
| `gno query "GraphML graphio adoption evidence" -c bluetape4k-docs --no-rerank` | PASS | Returned workshop graph-io planning context. |
| `gno query "GraphML graphio issue 433" -c bluetape4k-github --fast --no-rerank` | PASS | Returned bluetape4k-graph compatibility PRs/issues. |
| External docs lookup | PASS | NetworkX, Gephi, Neo4j APOC, yWorks/yFiles, and sibling repo evidence checked. |
| `go test -count=1 ./graph ./graph/graphio` | PASS | Graph and graphio tests passed after README/research update. |
| `go test -race -count=1 ./graph/graphio` | PASS | Graphio race gate passed. |
| `git diff --check` | PASS | No whitespace errors in bluetape-go diff. |
| `gno update` | PASS | Added the new wiki note and refreshed docs index entries. |
| `gno embed --collection bluetape4k-wiki` | PASS | Embedded two chunks for the new wiki research note. |
| `gno search "GraphML graphio defer" -c bluetape4k-wiki --limit 5` | PASS | Returned `research/2026-07-09-bluetape-go-graphml-graphio-evaluation.md`. |

## Findings

P0=0 P1=0

- No blocker findings.
- P3 follow-up: if a future issue implements GraphML, require a fixture corpus
  before public README wording claims producer compatibility.

## Residual Risk

This is a research decision, not an implementation proof. If a downstream
workflow later requires GraphML, the implementation issue must rerun source
checks against the exact producer versions and fixture files.
