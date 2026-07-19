# Issue #537 etcd Leader Election Step 6-R Code Review

Issue: #537 `feat: Add etcd leader election backend`

Date: 2026-07-19

Base and merge base: `41663dea0a2a34cd459df24802f59882cff834aa`

Reviewed implementation SHA: `f5d24a83b08777cced3ede65c755af061417556b`

Branch: `feat/issue-537-etcd-leader`

Gate: six independent perspectives plus main-session integration.

## Live Metadata

| Item | Live result |
|---|---|
| Issue state | OPEN |
| Milestone | `0.19.0` |
| Assignee | `debop` |
| Labels | `type: task`, `area: leader`, `area: testing`, `priority: p2` |
| Pull request | Not created at this gate |
| Remote CI/reviews/threads | N/A until the authorized PR is created |

## Convergence History

The review started from the completed etcd provider and repeatedly refreshed
all affected perspectives after each correction. The exact repair commits were:

| Commit | Decision |
|---|---|
| `c024e2d` | Contained campaign, cleanup, timeout, and diagnostic failure boundaries. |
| `bb2da02` | Aligned cadence, documentation, rollback, and review evidence. |
| `73f7f25` | Added explicit fleet contender, lease, watcher, and Proclaim capacity gates. |
| `6f60d8a` | Pinned attached-lease authorization evidence and server-version rerun requirements. |
| `c5c3c1d` | Closed shared-client shutdown hazards, bounded campaign join, and preserved cleanup-attempt history. |
| `ac168e1` | Made per-unit leadership guards executable and separated exact cleanup proof from unresolved state. |
| `ae85445` | Raised `x/crypto` and `x/net` to the active and rollback security floor and pinned the vulnerability scan. |
| `c948495` | Moved unresolved inventory and provider restore behind the final healthy-client exact-absence proof. |
| `f5d24a8` | Locked the zero-contender failure branch with a direct persist-and-no-restore regression test. |

The first security hypothesis that cross-principal `KeepAliveOnce` bypassed
authorization was disproved against pinned etcd `v3.6.13` and the server's
`checkLeaseRenew` path. The final test proves that both `Revoke` and
`KeepAliveOnce` are denied for an attached lease outside the principal's key
range. Mutually untrusted tenants still require separate clusters.

The initial Developer/API observer concern was normalized as an approved
non-goal: the public contract intentionally uses bounded `IsLeader` sampling
and does not add an asynchronous event API.

No final lane timed out; main-session fallback was not required.

## Terminal Exact-Head Results

| Tier | Perspective | Verdict | P0 | P1 | P2 | P3 |
|---|---|---:|---:|---:|---:|---:|
| 1 | Performance | PASS | 0 | 0 | 0 | 0 |
| 2 | Stability | PASS | 0 | 0 | 0 | 0 |
| 3 | Security | PASS | 0 | 0 | 0 | 1 |
| 4 | Operator/Ops | PASS | 0 | 0 | 0 | 0 |
| 5 | Developer/API | PASS | 0 | 0 | 0 | 0 |
| 6 | User/Caller | PASS | 0 | 0 | 0 | 0 |
| Main | Integration | PASS | 0 | 0 | 0 | 1 |

Every terminal lane reviewed implementation SHA
`f5d24a83b08777cced3ede65c755af061417556b` against
`41663dea0a2a34cd459df24802f59882cff834aa`.

## Accepted Non-Blocking Finding

Security P3: `GO-2026-5932` marks `golang.org/x/crypto/openpgp` as
unsafe-by-design and unmaintained. The repository does not import or call that
package, the advisory has no fixed module version, and the pinned repository
scan reports zero reachable or imported-package vulnerabilities. Do not add an
`openpgp` import; retain the pinned scan in promotion and rollback gates.

## Verification Evidence

Fresh evidence on the reviewed implementation SHA:

- `make ci` — PASS in 578 seconds, including tidy, formatting, vet, lint,
  repository normal tests, repository race tests, and Testcontainers packages.
- `make lint` within CI — PASS, `0 issues`.
- `go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...` — PASS with zero
  reachable and zero imported-package vulnerabilities.
- `go mod verify` and `go mod tidy -diff` — PASS.
- Full `leader/leadertest` plus `leader/etcd` normal and race suites — PASS.
- Supervisor proof/rollback tests — PASS at `-count=50`; focused race runs —
  PASS at up to `-race -count=20`.
- Cadence and single-flight tests — PASS at `-count=10`; focused race — PASS.
- Real etcd `v3.6.13` conformance, authorization, 32-contender resource return,
  hard-stop, exact-key watch, and cleanup proofs — PASS.
- English/Korean README and release-runbook contract tests — PASS.
- `git diff --check` and clean-worktree verification — PASS.

An earlier ten-run Docker-backed etcd soak completed in 281 seconds during
convergence. Final exact-head confidence comes from the fresh full CI plus the
terminal lane-specific normal, race, and real-server evidence above.

## Main Integration Verdict

PASS for local PR readiness.

- P0 = 0
- P1 = 0
- P2 = 0
- P3 = 1, accepted as a non-reachable module-only advisory
- The caller-owned client, official Session/Election primitives, exact-key
  ownership monitoring, proof-gated cleanup, hard-stop coordination, capacity
  gates, security floor, bilingual operations guidance, and Type A lesson are
  aligned with the approved design and plan.
- The implementation is ready for the already authorized push and PR creation.
- It is not merge-ready until the PR exact head passes remote CI and Step 7-R.

## DoD

| Item | Status |
|---|---|
| Live issue and milestone state refreshed | Done. |
| Six independent perspectives covered | Done. |
| Same exact implementation SHA reviewed | Done: `f5d24a83b08777cced3ede65c755af061417556b`. |
| Main integration review completed | Done. |
| P0/P1/P2 normalized | Done: `P0=0 P1=0 P2=0`. |
| Accepted P3 recorded | Done: one non-reachable module-only advisory. |
| Targeted, race, real-server, static, vulnerability, and full CI evidence | Done. |
| Type A reusable lesson | Done: `docs/lessons/2026-07-19-issue-537-etcd-leader.md`. |
| Remote CI/reviews/threads | N/A; PR not yet created. |
| Merge side effect | Not authorized; fresh merge approval remains required. |
