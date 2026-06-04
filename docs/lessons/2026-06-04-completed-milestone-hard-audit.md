# Lessons Learned — Completed Milestone Hard Audit (2026-06-04)

**Related issue**: #113
**Affected milestones**: `0.1.0`, `0.1.1`, `0.2.0`, completed `0.3.0`

## L1: Completed issues still need evidence-depth audits

### Problem

The completed milestone set passed current CI and race checks, but some public
edge cases had weaker direct evidence than the newer cache and near-cache work.

### Lesson

After a milestone-quality miss, audit completed issues with the same 7-tier
frame used for new Type A work. Keep P0/P1 as blockers, but convert weaker
P2 evidence into a tracked follow-up issue instead of leaving it only in chat.

### Evidence

- `make ci`: PASS
- `go test -count=1 -coverprofile=/tmp/bluetape-go-issue-113.cover ./...`: PASS
- `go test -race -count=1 ./cache ./cache/redisnear ./leader/redis ./resilience`: PASS
- Follow-up #114 tracks non-blocking edge-case evidence gaps.
