# Issue 27 Workflow Runners Verifier

Issue: #27
Spec: `docs/superpowers/specs/2026-06-06-issue-27-workflow-runners-spec.md`
Plan: `docs/superpowers/plans/2026-06-06-issue-27-workflow-runners-plan.md`
Gate: Step 5
Status: VERIFIED

## Verified Scope

- Added first-party `workflow` package.
- Implemented `Work`, `Predicate`, `Runner`, `Sequential`, `Conditional`, and
  `Parallel`.
- Added checkable errors for nil work, nil predicate, too many false branches,
  and unknown report status.
- Added English/Korean package READMEs, compile-checked examples, release notes,
  WIP update, and lesson artifact.

## Acceptance Evidence

| Requirement | Status | Evidence |
|---|---|---|
| Sequential runner stops or continues according to policy. | PASS | `workflow/workflow.go:71`; tests at `workflow/sequential_test.go:11`, `:40`, `:62`, `:87`. |
| Parallel runner propagates cancellation and aggregates errors. | PASS | `workflow/workflow.go:126`; tests at `workflow/parallel_test.go:12`, `:37`, `:69`, `:117`, `:135`. |
| Conditional runner is covered with branch tests. | PASS | `workflow/workflow.go:91`; tests at `workflow/conditional_test.go:11`, `:36`, `:61`, `:76`, `:99`, `:122`. |
| Context-driven and no mutable `WorkContext`. | PASS | `workflow/workflow.go:13`; README non-goal at `workflow/README.md`; examples use closures only. |
| Stress/cancellation helpers used. | PASS | `GoroutineStressTester` at `workflow/workflow_concurrency_test.go:13`; `AsyncJobTester` at `workflow/workflow_concurrency_test.go:38`. |
| README pair and examples exist. | PASS | `workflow/README.md`, `workflow/README.ko.md`, `workflow/workflow_example_test.go`. |
| No new dependencies. | PASS | Implementation imports stdlib plus `workreport`; no `go.mod` change. |

## Validation Commands

| Command | Result |
|---|---|
| `gofmt -w workflow` | PASS |
| `go test -count=1 ./workflow ./workreport` | PASS: `workflow 0.669s`, `workreport 1.102s` |
| `go test -count=1 ./workflow -run Example` | PASS: `workflow 0.379s` |
| `go test -race -count=1 ./workflow ./workreport` | PASS: `workflow 1.441s`, `workreport 1.825s` |
| `go test -count=1 ./...` | PASS: all packages, including Testcontainers packages |
| `go vet ./workflow ./workreport` | PASS |
| `rg -n "^func Example|GoroutineStressTester|AsyncJobTester" workflow` | PASS |
| `rg -n "workflow|#27|0.4.0" CHANGELOG.md WIP.md docs/lessons/2026-06-06-workflow-runners.md` | PASS |
| `git diff --check` | PASS |

## Residual Risks

| Risk | Status | Notes |
|---|---|---|
| Parallel stop cause could be hidden by earlier sibling cancellation. | Mitigated | `parentFrom` uses the actual stop cause while preserving child input order. Covered by `TestParallelStopOnFailureCancelsSiblingsAndKeepsCause`. |
| Work that ignores context may still return completed after cancellation. | Accepted | Runner propagates context and cancels siblings; a work function owns honoring its context. README documents context-driven work. |
| Any-success parallel semantics not implemented. | Accepted | Spec and plan explicitly defer optional any-success semantics. |

## Verdict

VERIFIED. Issue #27 implementation satisfies the spec and plan with current
local evidence.
