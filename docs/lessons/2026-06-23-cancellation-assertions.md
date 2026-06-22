# Lessons: Cancellation Assertions

## What changed

Issue #213 added test-only helpers that assert Go context cancellation
contracts:

- direct `context.Canceled` propagation
- direct `context.DeadlineExceeded` propagation
- blocked waiter release after cancellation
- resource cleanup observation after cancellation

## What surprised us

`golangci-lint` reused stale cache entries from a deleted sibling worktree and
reported diagnostics for paths that no longer existed. Cleaning the cache and
rerunning lint separated stale cache noise from real current-worktree
`errorlint` findings.

Native subagent cleanup also blocked for an hour-scale wait. For small Type B
Go work, keep the 7-tier gate bounded: if native lanes are stale or unresponsive,
record the fallback and complete the six review lenses in the main session.

## What to repeat

- Start with RED tests for new public assertion helpers so diagnostics are
  locked before implementation.
- Preserve wrapped errors in diagnostic helpers when a probe returns a concrete
  error; use explicit `<nil>` diagnostics when no error is returned.
- Document that cancellation helpers are cooperative. Go cannot safely stop a
  goroutine that ignores `ctx.Done()` forever.
- Run `golangci-lint cache clean` before treating deleted-worktree lint paths
  as current findings.

## Verification

- `go test -count=1 ./testing`: PASS
- `go test -race -count=1 ./testing`: PASS
- `go test -count=1 ./testing/...`: PASS
- `make fmt-check && make vet && make lint`: PASS
- `make test`: PASS
- `make race`: PASS
