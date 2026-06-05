# Lessons Learned - Cache Benchmark Suite (2026-06-04)

Issue: #107
Milestone: 0.3.0

## L1: Benchmark PRs need measured docs, not only commands

### Problem

#107 asks for benchmark commands, environment notes, and sample results. A PR
body alone would satisfy the short-term review but would not be searchable by
GNO after the branch is merged.

### Lesson

Benchmark work should write the durable interpretation under `docs/research`
with sample output, environment notes, and explicit limits such as "local
snapshot, not production ranking".

### Evidence

- `docs/research/2026-06-04-issue-107-cache-benchmark-suite.md`
- `go test -run '^$' -bench '^BenchmarkMemory' -benchtime=100ms -benchmem ./cache`
- `go test -run '^$' -bench '^BenchmarkNearCache' -benchtime=100ms -benchmem ./cache/redisnear`

## L2: Worktree boundaries must be checked before patching

### Problem

The first patch application used the session default directory and wrote #107
files into the main `develop` worktree instead of the feature worktree.

### Lesson

For multi-worktree bluetape-go work, use absolute paths or verify `git status`
in the target worktree before every `apply_patch` edit. If a patch lands in the
wrong worktree, move the patch to the feature worktree and restore `develop`
clean before continuing.

### Evidence

- Main `develop` worktree was restored to clean before tests continued.
- All #107 changes now live in `.worktrees/bench-issue-107-cache-suite`.
