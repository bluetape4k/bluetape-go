# Add async await and polling test helpers

Closes #211.

## Summary

This PR adds context-aware await/polling helpers to the root `testing` package
so tests can wait for bounded eventual consistency while preserving caller-owned
context cancellation and deadline behavior.

The API stays intentionally small: one generic `CheckAwait`/`RequireAwait`
pair, value/error convenience helpers, and an `AwaitResult` diagnostic payload
that records the final observed value, error, attempts, and elapsed time.

## Background

Milestone 0.6.4 is aligning bluetape-go's test helpers with JUnit5/Awaitility
style testing while keeping the Go API idiomatic. Existing
`Eventually`/`Consistently` helpers remain useful for boolean assertions. The
new await helpers cover richer probes where a caller needs context propagation,
polling intervals, immediate failure, and final observation diagnostics.

## Work Done

- Added `AwaitStatus`, `AwaitProbe`, `AwaitCheck`, `AwaitErrorProbe`, and
  `AwaitResult`.
- Added `CheckAwait` and `RequireAwait`.
- Added `CheckAwaitValue` and `RequireAwaitValue`.
- Added `CheckAwaitError` and `RequireAwaitError`.
- Added tests for immediate success, eventual success, timeout diagnostics,
  immediate failure diagnostics, invalid inputs, caller cancellation, and probe
  cancellation.
- Added examples for cache invalidation, lock acquisition, Testcontainers
  readiness, and workflow status checks.
- Updated `testing/README.md` and `testing/README.ko.md` in sync.
- Added review and lesson evidence for the workflow gates.

## Validation

- `go test -count=1 ./testing/...`: PASS baseline before implementation
- TDD RED: `go test -count=1 ./testing` failed on undefined `CheckAwait*` and
  `AwaitStatus` before implementation
- `go test -count=1 ./testing`: PASS
- `go test -race -count=1 ./testing`: PASS
- `go test -count=1 ./testing/...`: PASS
- `git diff --check`: PASS
- `golangci-lint cache clean`: PASS
- `make fmt-check && make vet && make lint`: PASS, `0 issues.`
- `make test`: PASS
- `make race`: PASS

## Review Notes

- Step 6-R tracked review: `docs/review/2026-06-23-issue-211-async-await-polling-review.md`
- Main-session 7-tier fallback used because stale native subagent cleanup had
  previously blocked for an hour-scale wait.
- P0=0, P1=0.

## Metadata

- Issue: #211
- Milestone: `0.6.4`
- Assignee: `debop`
- Labels: `type: task`, `area: testing`, `priority: p1`, `area: concurrency`

## DoD Status

| Step | Status | Evidence |
| --- | --- | --- |
| Issue metadata | PASS | #211 assignee `debop`, milestone `0.6.4`, labels verified live |
| Worktree | PASS | `.worktrees/issue-211-async-await-polling`, branch `issue-211-async-await-polling` |
| TDD RED | PASS | `go test -count=1 ./testing` failed before implementation on undefined APIs |
| Implementation | PASS | `testing/await.go`, tests, examples, README pair |
| Step 6-R review | PASS | `docs/review/2026-06-23-issue-211-async-await-polling-review.md`, P0=0 P1=0 |
| Lessons | PASS | `docs/lessons/2026-06-23-async-await-polling.md` |
| Local validation | PASS | `make fmt-check && make vet && make lint`, `make test`, `make race` |
| PR body verification | PENDING | Verify with `gh pr view <number> --json body` after PR creation |
| Step 7-R PR review | PENDING | Run after PR creation |
| CI | PENDING | Check `statusCheckRollup` after GitHub Actions start |
