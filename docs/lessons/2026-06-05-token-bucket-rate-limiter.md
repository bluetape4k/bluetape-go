# Lessons Learned - Token-Bucket Rate Limiter (2026-06-05)

Related issue: #25
Milestone: 0.3.0
Affected packages: `ratelimit`, `ratelimit/redis`

## L1: Apply patches must target the feature worktree explicitly

### Problem

The first research/spec/plan patch was applied from the main repo cwd instead
of the feature worktree. The mistake was detected by comparing `git status` in
both worktrees before implementation began.

### Lesson

For Type A work in linked worktrees, always verify the `apply_patch` path and
run `git status --short --branch` in both the main worktree and feature
worktree after the first write. If a patch lands in the wrong worktree, remove
only the agent-created files and reapply them under the feature worktree path
before continuing.

### Evidence

- Main `develop` worktree returned to clean state.
- Feature branch retained the research/spec/plan/review artifacts.

## L2: External official-doc evidence needs wiki preservation before PR

### Problem

The first plan draft used official Redis and Go docs as design evidence but did
not include the `bluetape4k-wiki` preservation step required by the workflow
SOP.

### Lesson

When a bluetape4k feature uses external official docs, include wiki note
preservation and `gno embed --collection bluetape4k-wiki` in the plan before
implementation. Treat missing preservation as a review finding, not a
postscript.

### Evidence

- `bluetape4k-wiki` note:
  `research/2026-06-05-token-bucket-rate-limiter-redis-go.md`
- Wiki commit: `2ac234d`
- Validation: `gno update`, `gno embed --collection bluetape4k-wiki`, and
  representative `gno search`.

## L3: Per-call token requests must be bounded by burst

### Problem

The initial spec validated positive token requests but did not explicitly state
what happens when a caller asks for more tokens than the bucket capacity.

### Lesson

For token-bucket APIs, `tokens > Burst` should be a validation error because no
amount of waiting can make the request satisfiable. Cover this in both local and
Redis tests.

### Evidence

- `TestTokenBucketRejectsOverBurstRequest`
- `TestLimiterRejectsInvalidAllowInput`
