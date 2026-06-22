# Add cancellation assertion helpers for async Go APIs

Closes #213.

## Summary

This PR adds test-only cancellation assertion helpers to the root `testing`
package so async Go APIs can prove that they preserve caller-owned context
cancellation instead of hiding or retrying it.

The helpers cover direct cancellation, deadline expiration, blocked waiter
release, and cleanup observation after cancellation. The matching `Require*`
wrappers fail a supplied `testing.TB`, while `Check*` functions return
diagnostic errors that are easy to assert in tests.

## Background

Milestone 0.6.4 is aligning the Go testing helpers with the cancellation and
concurrency contracts used across the bluetape ecosystem. Go cannot safely stop
a goroutine that ignores `ctx.Done()` forever, so these helpers intentionally
assert cooperative cancellation behavior rather than pretending to force-kill
work.

## Work Done

- Added `ContextOperation`, `WaiterProbe`, and `CleanupProbe` contracts.
- Added `CheckContextCanceled` and `RequireContextCanceled`.
- Added `CheckDeadlineExceeded` and `RequireDeadlineExceeded`.
- Added `CheckWaiterReleased` and `RequireWaiterReleased`.
- Added `CheckCleanupOnCancel` and `RequireCleanupOnCancel`.
- Added success, diagnostic, timeout, cleanup, and example tests.
- Updated `testing/README.md` and `testing/README.ko.md` with synchronized
  cancellation helper usage and cooperative cancellation guidance.
- Added spec, review, and lesson evidence for the workflow gates.

## Validation

- `go mod download`: PASS
- `go test -count=1 ./testing/...`: PASS baseline
- TDD RED: `go test -count=1 ./testing` failed on undefined cancellation APIs
  before implementation
- `go test -count=1 ./testing`: PASS
- `go test -race -count=1 ./testing`: PASS
- `go test -count=1 ./testing/...`: PASS
- `golangci-lint cache clean`: PASS
- `make fmt-check && make vet && make lint`: PASS, `0 issues.`
- `make test`: PASS
- `make race`: PASS
- `git diff --check`: PASS

## Review Notes

- Step 6-R tracked review: `docs/review/2026-06-23-issue-213-cancellation-assertions-review.md`
- Main-session 7-tier fallback used because stale native subagent cleanup
  blocked and was aborted.
- P0=0, P1=0.

## Metadata

- Issue: #213
- Milestone: `0.6.4`
- Assignee: `debop`
- Labels: `type: task`, `area: testing`, `priority: p0`, `area: concurrency`

## DoD Status

| Step | Status | Evidence |
| --- | --- | --- |
| Issue metadata | PASS | #213 assignee `debop`, milestone `0.6.4`, labels verified live |
| Worktree | PASS | `.worktrees/issue-213-cancellation-assertions`, branch `issue-213-cancellation-assertions` |
| TDD RED | PASS | `go test -count=1 ./testing` failed before implementation on undefined APIs |
| Implementation | PASS | `testing/cancellation.go`, tests, examples, README pair |
| Step 6-R review | PASS | `docs/review/2026-06-23-issue-213-cancellation-assertions-review.md`, P0=0 P1=0 |
| Lessons | PASS | `docs/lessons/2026-06-23-cancellation-assertions.md` |
| Local validation | PASS | `make fmt-check && make vet && make lint`, `make test`, `make race` |
| PR body verification | PENDING | Verify with `gh pr view <number> --json body` after PR creation |
| Step 7-R PR review | PENDING | Run after PR creation |
| CI | PENDING | Check `statusCheckRollup` after GitHub Actions start |
