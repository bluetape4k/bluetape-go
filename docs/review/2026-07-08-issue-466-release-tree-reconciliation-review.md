# Issue #466 Release Tree Reconciliation Review

## Scope

- Issue: #466 `release: reconcile develop with v0.15.0 main release tree`
- Branch: `chore/issue-466-reconcile-main-tree`
- Base: `origin/develop`
- Release tree source: `origin/main` at `c1e6db575beee41d19ce6b3ca5de5a89ef0b0da8`

## Changes Reviewed

- Merged the current `origin/main` release tree into the `origin/develop` line.
- Resolved release-tree conflicts in favor of `origin/main` because `main` is the
  published `v0.15.0` source of truth and already includes the intended 0.15.0
  release payload.
- Preserved main-only release assets that would otherwise be deleted by a future
  direct `develop -> main` release promotion, including MongoDB Testcontainers
  docs/code and 0.13.0/0.14.0 release evidence.
- Added this review artifact and the paired lesson so the reconciliation reason
  survives after the sync.

## Acceptance Mapping

| Requirement | Status | Evidence |
|---|---|---|
| Remove main-only release assets from the future delete set | PASS | Before sync, `git diff --name-status origin/develop..origin/main` listed `testcontainers/mongodb/*` and release/review docs as additions on `main`. |
| Keep the published release tree as source of truth | PASS | Conflict resolution used `origin/main`; `git diff --cached --name-status origin/main` was empty before adding issue #466 evidence docs. |
| Preserve current 0.15.0 release payload | PASS | Source commit is `v0.15.0` target `c1e6db575beee41d19ce6b3ca5de5a89ef0b0da8`. |
| Keep validation explicit | PASS | `git diff --cached --check` passed after conflict resolution; full `make ci` remains the merge gate. |

## 7-Tier Review

| Lane | Verdict | Notes |
|---|---|---|
| Performance | PASS | No new runtime behavior beyond already released code; this PR reconciles branch topology. P0=0 P1=0. |
| Stability | PASS | Future release promotion no longer risks deleting already released assets. P0=0 P1=0. |
| Security | PASS | No new security boundary or secret-bearing code is introduced by this reconciliation. P0=0 P1=0. |
| Operator/Ops | PASS | Release-guide and release evidence from `main` are preserved on `develop`. P0=0 P1=0. |
| Developer/API | PASS | Public package files come from the already published `v0.15.0` tree. P0=0 P1=0. |
| User/Caller | PASS | README/package docs remain aligned with the released tree. P0=0 P1=0. |
| Integration | PASS | The sync branch makes future `develop -> main` promotion a normal additive/reviewable path instead of a deletion-prone projection. P0=0 P1=0. |

## Validation

- `git diff --cached --check`: PASS after merge conflict resolution.
- `git diff --cached --name-status origin/main`: PASS before adding this issue
  evidence, proving the resolved tree matched the released `main` tree.
- `make ci`: PASS, including normal tests and race tests across Docker-backed
  Testcontainers packages.

P0=0 P1=0.
