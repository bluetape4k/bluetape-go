# Issue #200 Retrospective Audit Implementation Plan

> 한국어 운영 요약: 이 계획 문서는 사용자 협업용 실행 계획이다. 아래 원문에 포함된 명령, 경로, API 이름, issue/PR 번호, branch 이름, code block, test output은 추적성과 재현성을 위해 그대로 보존한다. 작업 순서, 위험, 검증, 롤백 판단은 한국어 독자가 바로 실행 경계를 이해할 수 있도록 이 메모를 우선 적용한다.
> 추가 한국어 요약: 이 문서의 실행 판단은 기존 순서를 따르며, 변경자는 작업 표와 검증 목록을 먼저 확인한 뒤 관련 테스트를 실행한다. 영어로 남은 항목은 코드 식별자 또는 재현 증거다.\n

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce a source-derived audit artifact for issue #200 that re-verifies milestones `0.1.0` through `0.6.1`, records package-by-package P0/P1/P2/P3 findings, files follow-up issues for P0/P1 findings before closure, and ends with the exact final gate `P0=<n> P1=<n>`.

**Architecture:** Keep #200 as an audit-only branch. Build an inventory from GitHub milestone and issue data, inspect repo source/tests/docs/benchmarks/PR history by package slice, run six independent review lenses, integrate results into one severity ledger, and commit the audit artifact plus Step 6-R and Step 7-R review records.

**Tech Stack:** Go 1.26.3, GitHub CLI, repo-local tests and benchmarks, Testcontainers-backed package tests, `testing/concurrency` goroutine stress helpers, Superpowers spec/plan artifacts, `$bluetape4k-workflow`, `$bluetape-go-patterns`, `$bluetape4k-diagram`.

---

## File Structure

- Reuse diagram assets:
  - `docs/images/readme-diagrams/issue-200-retrospective-audit-flow.png`
  - `docs/images/readme-diagrams/issue-200-retrospective-audit-flow.svg`
- Create audit artifact:
  - `docs/audits/2026-06-14-issue-200-retrospective-audit.md`
- Create raw command output directory:
  - `docs/audits/outputs/issue-200/`
- Create captured outputs:
  - `docs/audits/outputs/issue-200/milestones.json`
  - `docs/audits/outputs/issue-200/issues-0.1.0-0.6.1.json`
  - `docs/audits/outputs/issue-200/package-list.txt`
  - `docs/audits/outputs/issue-200/go-test-all.txt`
  - `docs/audits/outputs/issue-200/go-test-race-all.txt`
  - `docs/audits/outputs/issue-200/go-test-race-targeted.txt`
  - `docs/audits/outputs/issue-200/make-ci.txt`
- Create review artifacts:
  - `docs/superpowers/reviews/2026-06-14-issue-200-retrospective-audit-step-3r-plan-review.md`
  - `docs/superpowers/reviews/2026-06-14-issue-200-retrospective-audit-step-6r-code-review.md`
  - `docs/superpowers/reviews/2026-06-14-issue-200-retrospective-audit-step-7r-pr-review.md`
- Create PR body file before PR creation:
  - `docs/superpowers/pr/2026-06-14-issue-200-retrospective-audit-pr-body.md`

## Audit Artifact Schema

`docs/audits/2026-06-14-issue-200-retrospective-audit.md` must contain these
top-level sections in this order:

1. `# Issue #200 Retrospective Audit`
2. `## 범위 And Baseline`
3. `## Audit Flow`
4. `## Milestone And Issue Inventory`
5. `## Issue To Package Map`
6. `## Package Findings`
7. `## P0/P1 Follow-Up Issues`
8. `## Deferred Parity Gaps`
9. `## 검증 Evidence`
10. `## 7-Tier Integration Verdict`
11. `## DoD Status`

Every package entry under `## Package Findings` must use this table shape. Use
only concrete paths, issue numbers, and observed facts from the inspected
package; do not leave angle-bracket tokens or generic notes in the committed
audit artifact.

```markdown
### core

| Field | Evidence |
|---|---|
| Source files | `core/*.go` |
| Tests | `core/*_test.go` |
| Docs/examples | `core/README.md`, `core/README.ko.md` |
| Recent issues/PRs | `#1`, `#2` |

| Lane | P0 | P1 | P2 | P3 | Notes |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | Reviewed benchmarks and hot-path helpers; no finding in this example row. |
| Stability | 0 | 0 | 0 | 0 | Reviewed nil, zero-value, and panic behavior; no finding in this example row. |
| Security | 0 | 0 | 0 | 0 | No credential, parser trust, or external input boundary in this example row. |
| Operator/Ops | 0 | 0 | 0 | 0 | Package has no runtime operator surface in this example row. |
| Developer/API | 0 | 0 | 0 | 0 | Exported API docs and error behavior reviewed in this example row. |
| User/Caller | 0 | 0 | 0 | 0 | README examples reviewed in this example row. |

Verdict: P0=0 P1=0 P2=0 P3=0
```

The committed audit artifact must not contain unresolved schema examples.

## Package Slices

Use these slices so the audit can progress without mixing unrelated package
risks:

| Slice | Packages | Primary risks |
|---|---|---|
| Foundation | `core`, `collections`, `serialization`, `codec`, `compression` | API docs, nil/zero behavior, benchmark reproducibility, parser and codec trust boundaries. |
| Concurrency/Test | `concurrency`, `testing`, `testing/concurrency`, `workreport` | goroutine lifecycle, stress-test behavior, cancellation, assertion ergonomics. |
| Redis/Distributed | `cache`, `cache/rediscoord`, `cache/redisnear`, `leader`, `leader/redis`, `lock/redis`, `ratelimit`, `ratelimit/redis`, `probabilistic/redis`, `jwt/redis` | cleanup, context propagation, Redis key ownership, race behavior, Testcontainers cost. |
| Runtime APIs | `resilience`, `workflow`, `state`, `batch` | deadlines, error propagation, state transitions, worker lifecycle, rollback/failure reporting. |
| Utility APIs | `id`, `jwt`, `measure`, `money`, `probabilistic` | public API shape, security defaults, deterministic behavior, docs/examples, benchmark evidence. |
| Testcontainers | `testcontainers/kafka`, `testcontainers/mysql`, `testcontainers/nats`, `testcontainers/postgres`, `testcontainers/redis` | resource cleanup, startup time, shared Docker constraints, skipped-test reporting. |

## Task 1: Capture Inventory Inputs

**Files:**
- Create directory: `docs/audits/outputs/issue-200/`
- Create: `docs/audits/outputs/issue-200/milestones.json`
- Create: `docs/audits/outputs/issue-200/issues-0.1.0-0.6.1.json`
- Create: `docs/audits/outputs/issue-200/package-list.txt`

- [ ] **Step 1: Capture milestone metadata**

Run:

```bash
gh api repos/bluetape4k/bluetape-go/milestones --paginate > docs/audits/outputs/issue-200/milestones.json
```

Expected evidence:

```bash
jq -r '.[] | [.number,.title,.open_issues,.closed_issues] | @tsv' docs/audits/outputs/issue-200/milestones.json
```

The output includes milestones `0.1.0` through `0.6.1`.

- [ ] **Step 2: Capture historical issue metadata**

Run:

```bash
gh issue list --state all --limit 300 --json number,title,state,labels,milestone,closedAt,updatedAt,url > docs/audits/outputs/issue-200/issues-0.1.0-0.6.1.json
```

Expected evidence:

```bash
jq '[.[] | select(.milestone.title as $m | ["0.1.0","0.1.1","0.2.0","0.3.0","0.4.0","0.5.0","0.6.0","0.6.1"] | index($m))] | length' docs/audits/outputs/issue-200/issues-0.1.0-0.6.1.json
```

The count is greater than zero.

- [ ] **Step 3: Capture current package list**

Run:

```bash
go list ./... | tee docs/audits/outputs/issue-200/package-list.txt
```

Expected evidence includes:

```text
github.com/bluetape4k/bluetape-go/cache/rediscoord
github.com/bluetape4k/bluetape-go/jwt/redis
github.com/bluetape4k/bluetape-go/testing/concurrency
```

## Task 2: Build Issue-To-Package Map

**Files:**
- Modify: `docs/audits/2026-06-14-issue-200-retrospective-audit.md`

- [ ] **Step 1: Create the initial audit artifact**

Create the artifact with the schema from this plan and embed the audit diagram:

```markdown
![Issue #200 retrospective audit flow](../images/readme-diagrams/issue-200-retrospective-audit-flow.png)
```

- [ ] **Step 2: Map named issues to packages**

Use `gh issue view <number> --json number,title,body,labels,milestone,url` for
the issue list named in #200. Record each issue in `## Issue To Package Map`.

Required issue ranges:

```text
#1 #2 #3 #4 #5 #6 #8-#36 #69 #76 #85 #86 #89-#98 #107 #110 #113-#117 #123 #125 #132-#137 #158 #164-#175 #178-#182 #187 #192 #195
```

For each mapped issue, record:

- Issue number and title.
- Current state.
- Affected package paths.
- Evidence source: source path, README path, review artifact, PR, or issue body.

## Task 3: Review Package Slices

**Files:**
- Modify: `docs/audits/2026-06-14-issue-200-retrospective-audit.md`

- [ ] **Step 1: Review Foundation slice**

Inspect:

```bash
rg -n 'TO''DO|panic\(|context\.TO''DO|errors\.New|fmt\.Errorf|Benchmark|Example' core collections serialization codec compression
go test -count=1 ./core ./collections ./serialization ./codec ./compression
```

Record package entries for `core`, `collections`, `serialization`, `codec`,
and `compression`.

- [ ] **Step 2: Review Concurrency/Test slice**

Inspect:

```bash
rg -n 'go func|context\.TO''DO|time\.Sleep|WaitGroup|GoroutineStressTester|runtime\.NumGoroutine|panic\(' concurrency testing workreport
go test -count=1 ./concurrency ./testing ./testing/concurrency ./workreport
go test -race -count=1 ./concurrency ./testing/concurrency
```

Record whether goroutine stress behavior is present, absent, or not applicable.

- [ ] **Step 3: Review Redis/Distributed slice**

Inspect:

```bash
rg -n 'context\.TO''DO|context\.Background|time\.Sleep|go func|Close\(|Del\(|Expire|SetNX|Eval|Subscribe|Watch|TxPipelined|redis.Nil' cache leader lock ratelimit probabilistic jwt
go test -count=1 ./cache ./cache/rediscoord ./cache/redisnear ./leader ./leader/redis ./lock/redis ./ratelimit ./ratelimit/redis ./probabilistic/redis ./jwt/redis
go test -race -count=1 ./cache/rediscoord ./cache/redisnear ./leader/redis ./lock/redis ./ratelimit/redis ./probabilistic/redis ./jwt/redis
```

Record Redis key ownership, cleanup, context propagation, and Testcontainers
cost for each package.

- [ ] **Step 4: Review Runtime APIs slice**

Inspect:

```bash
rg -n 'context\.TO''DO|context\.Background|go func|time\.Sleep|panic\(|recover|errors\.Is|errors\.As|fmt\.Errorf' resilience workflow state batch
go test -count=1 ./resilience ./workflow ./state ./batch
go test -race -count=1 ./resilience ./workflow ./state ./batch
```

Record lifecycle, failure, rollback, and error contract findings.

- [ ] **Step 5: Review Utility APIs slice**

Inspect:

```bash
rg -n 'context\.TO''DO|panic\(|errors\.New|fmt\.Errorf|Benchmark|Example|Marshal|Unmarshal|Parse|Verify|Sign' id jwt measure money probabilistic
go test -count=1 ./id ./jwt ./measure ./money ./probabilistic
go test -race -count=1 ./id ./jwt ./measure ./money ./probabilistic
```

Record security defaults, parser behavior, immutable/value API contracts, and
benchmark evidence.

- [ ] **Step 6: Review Testcontainers slice**

Inspect:

```bash
rg -n 'Terminate|Cleanup|Ryuk|context\.TO''DO|context\.Background|WithWaitStrategy|Started|Skip|t\.Cleanup' testcontainers
go test -count=1 ./testcontainers/kafka ./testcontainers/mysql ./testcontainers/nats ./testcontainers/postgres ./testcontainers/redis
```

Record container startup, cleanup, skipped-test, and shared Docker constraints.

## Task 4: Run Repository Validation Gates

**Files:**
- Create or update:
  - `docs/audits/outputs/issue-200/go-test-all.txt`
  - `docs/audits/outputs/issue-200/go-test-race-all.txt`
  - `docs/audits/outputs/issue-200/go-test-race-targeted.txt`
  - `docs/audits/outputs/issue-200/make-ci.txt`
- Modify: `docs/audits/2026-06-14-issue-200-retrospective-audit.md`

- [ ] **Step 1: Full test gate**

Run:

```bash
go test -count=1 ./... 2>&1 | tee docs/audits/outputs/issue-200/go-test-all.txt
```

Expected result:

```text
ok
```

No package line ends in `FAIL`.

- [ ] **Step 2: Full race gate**

Run:

```bash
go test -race -count=1 ./... 2>&1 | tee docs/audits/outputs/issue-200/go-test-race-all.txt
```

Expected result:

```text
ok
```

If Docker-backed package race checks exceed local resource limits, stop the
command, record the failure mode, and run the targeted race gate in Step 3.

- [ ] **Step 3: Targeted race and stress gate**

Run:

```bash
go test -count=1 ./testing/concurrency ./concurrency
go test -race -count=1 ./cache/rediscoord ./cache/redisnear ./leader/redis ./lock/redis ./ratelimit/redis ./probabilistic/redis ./jwt ./jwt/redis 2>&1 | tee docs/audits/outputs/issue-200/go-test-race-targeted.txt
```

Expected result:

```text
ok
```

No package line ends in `FAIL`.

- [ ] **Step 4: CI gate**

Run:

```bash
make ci 2>&1 | tee docs/audits/outputs/issue-200/make-ci.txt
```

Expected result:

```text
make ci
```

The command exits with status 0.

## Task 5: File P0/P1 Follow-Up Issues

**Files:**
- Modify: `docs/audits/2026-06-14-issue-200-retrospective-audit.md`

- [ ] **Step 1: Count P0/P1 findings**

Run a local count against the audit artifact after all package entries are
written:

```bash
rg -n "P0=[1-9]|P1=[1-9]|\\| [1-9] \\| [1-9] \\|" docs/audits/2026-06-14-issue-200-retrospective-audit.md
```

- [ ] **Step 2: Create follow-up issues for each P0/P1**

For every P0/P1 finding, create a GitHub issue:

```bash
gh issue create \
  --title "$SEVERITY: $PACKAGE $SHORT_FINDING" \
  --label "type: task" \
  --label "priority: p0" \
  --milestone "0.6.2" \
  --body-file /tmp/issue-200-followup.md
```

The body file must include:

```markdown
Parent audit: #200

Affected paths:
- $AFFECTED_PATH

Failure mode:
- $FAILURE_MODE

Recommended fix:
- $RECOMMENDED_FIX

Validation:
- $VALIDATION_COMMAND
```

Record each follow-up URL in `## P0/P1 Follow-Up Issues`.

If there are no P0/P1 findings, record:

```text
No P0/P1 follow-up issues required.
```

## Task 6: Finalize Audit Artifact And Step 6-R Review

**Files:**
- Modify: `docs/audits/2026-06-14-issue-200-retrospective-audit.md`
- Create: `docs/superpowers/reviews/2026-06-14-issue-200-retrospective-audit-step-6r-code-review.md`

- [ ] **Step 1: Finalize exact gate counts**

Add the exact final line under `## 7-Tier Integration Verdict`:

```text
P0=<n> P1=<n>
```

- [ ] **Step 2: Run Step 6-R 7-Tier review**

Use six independent review lanes plus main integration:

- Performance
- Stability
- Security
- Operator/Ops
- Developer/API
- User/Caller

If native subagents block, do not wait indefinitely. Record:

```text
lane timed out; main integration fallback performed
```

Then complete the same six-lane review in the main session.

- [ ] **Step 3: Run final local checks**

Run:

```bash
git diff --check
RED_FLAG_PATTERN="$(printf '%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s' 'TB''D' 'TO''DO' 'implement la''ter' 'fill in det''ails' 'Sim''ilar to' 'appro''priate' 'add val''idation' 'handle edge ca''ses' 'Write tests for the ab''ove' '<specific' '<path>' '<command>' '<n>')"
rg -n "$RED_FLAG_PATTERN|core example row" docs/audits/2026-06-14-issue-200-retrospective-audit.md docs/superpowers/reviews/2026-06-14-issue-200-retrospective-audit-step-6r-code-review.md
```

Expected result:

```text
no output from either command
```

## Task 7: PR Gate

**Files:**
- Create: `docs/superpowers/pr/2026-06-14-issue-200-retrospective-audit-pr-body.md`
- Create: `docs/superpowers/reviews/2026-06-14-issue-200-retrospective-audit-step-7r-pr-review.md`

- [ ] **Step 1: Write PR body**

The PR body must include:

- `Closes #200`
- Link to the audit artifact.
- Link to the Step 6-R review artifact.
- Validation command list with pass/fail state.
- Follow-up issue list for P0/P1 findings, or the exact no-follow-up statement.
- Final section heading exactly:

```markdown
## DoD Status
```

- [ ] **Step 2: Create PR with body file**

Run:

```bash
git push -u origin issue-200-retrospective-audit
gh pr create --base develop --head issue-200-retrospective-audit --title "Audit 0.1.0-0.6.1 implementation under superpowers discipline" --body-file docs/superpowers/pr/2026-06-14-issue-200-retrospective-audit-pr-body.md
```

- [ ] **Step 3: Verify live PR body**

Run:

```bash
PR_NUMBER="$(gh pr view --json number --jq .number)"
gh pr view "$PR_NUMBER" --json body --jq .body
```

The live body contains `Closes #200` and the final `## DoD Status` section.

- [ ] **Step 4: Run Step 7-R PR review**

Write `docs/superpowers/reviews/2026-06-14-issue-200-retrospective-audit-step-7r-pr-review.md`
with the same six lanes plus main integration shape.

## Commit Plan

Use three commits:

1. Spec and diagram commit already present:
   - `1b3532d Set the issue 200 audit gate before implementation`
2. Plan commit:
   - Add this plan and Step 3-R review.
3. Audit execution commit:
   - Add audit artifact, outputs, Step 6-R review, PR body, and Step 7-R review.

Every commit must use Lore protocol trailers.

## Stop Conditions

- Stop after Step 3-R and report for approval before executing audit tasks.
- Stop before merging the PR; merge requires explicit user approval.
- Stop and file follow-up issues before closing #200 if any P0/P1 findings are found.
