# Issue 86 Strategic Leader Elector Plan

Spec: `docs/superpowers/specs/2026-06-05-issue-86-strategic-leader-elector-spec.md`
Issue: #86
Milestone: 0.3.0

## Tasks

1. Core API and unit tests.
   - Add `leader/strategy.go`.
   - Add `leader/strategy_test.go`.
   - Cover candidate metrics, FIFO/random/scored strategy, and scorers.

2. Redis strategic elector.
   - Add `leader/redis/strategic.go`.
   - Store candidates as JSON values plus live ZSET index.
   - Use Lua and Redis server `TIME` for register/list bookkeeping.

3. Redis tests.
   - Add `leader/redis/strategic_test.go`.
   - Use Redis Testcontainers.
   - Include stress/cancellation helper tests.

4. Examples and docs.
   - Add compile-checked example for scored idle-time election.
   - Update package README files and root README locale pair.
   - Link #86 research from `docs/research` and the 0.3.0 research note.

5. Review artifacts.
   - Add lesson note.
   - Add local 7-tier review note with `P0=0 P1=0`.

6. Validation.
   - `gofmt`.
   - `go test -count=1 ./leader`.
   - `go test -count=1 ./leader/redis -run 'Strategic|Candidate|Scored|FIFO|Random|Async|Stress'`.
   - `go test -race -count=1 ./leader ./leader/redis -run 'Strategic|Candidate|Scored|FIFO|Random|Async|Stress'`.
   - `make ci`.
   - `git diff --check`.

7. Commit and PR.
   - Commit with Lore trailers.
   - Create PR assigned to `debop`, milestone `0.3.0`, closing #86.
   - Verify PR body and metadata.
   - Stop at PR DoD; do not merge automatically.

## Acceptance Matrix

| Behavior | Unit | Redis/Testcontainers | Stress | Race |
|---|---:|---:|---:|---:|
| Strategy determinism | Yes | No | No | Yes |
| Candidate register/list/unregister | No | Yes | Yes | Yes |
| TTL pruning | No | Yes | No | Yes |
| Result update | No | Yes | No | Yes |
| RunIfLeader action gating | No | Yes | Yes | Yes |
| Context cancellation | No | Yes | AsyncJobTester | Yes |

## Stop Condition

#86 PR exists with full metadata, local validation evidence, local 7-tier review
shows `P0=0 P1=0`, and the feature worktree has no unrelated changes.
