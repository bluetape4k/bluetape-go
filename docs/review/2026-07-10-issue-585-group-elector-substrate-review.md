# Issue #585 Redis GroupElector Substrate Review

## Scope

- Baseline: `develop` at `abb22b5`
- Implementation: `leader/redis/{elector.go,group.go,group_test.go,README.md,README.ko.md}`
- Parent: #570
- Review mode: local six-perspective equivalent. Native review-lane spawning
  is not exposed in this session; the main session performed the integration
  review.

## Evidence

- `go test -p 1 -count=1 ./leader/redis`
- `go test -p 1 -race -count=1 ./leader/redis`
- `make tidy-check`, `make fmt-check`, `make vet`, `make lint` (`0 issues`)
- `make test` and `make race` after a serial Testcontainers rerun
- `git diff --check`
- CodeGraph affected-file query and current diff review

The first `make ci` read stale lint paths from the removed #581 worktree;
`golangci-lint cache clean` removed that environment-only cache state. The
first full normal test run also hit two 5-second Redis Streams observation
timeouts outside this diff. Its isolated rerun and a clean full `make test`
both passed, so no source change was made for that unrelated flaky result.

## Six-Perspective Findings

| Perspective | P0 | P1 | P2 | P3 | Evidence and verdict |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | ZSET scripts, Redis command count, poll cadence, and server-time expiry behavior are unchanged. The suffix expands to the existing canonical owner-token representation, with no throughput claim. Benchmark is N/A; #560 owns provider benchmark measurement, table, chart, and analysis. |
| Stability | 0 | 0 | 0 | 0 | `Campaign` continues to preserve caller cancellation, while `renewLoop` retains its existing ticker, `done` channel, and ownership cleanup. Existing expiry, owner drift, cancellation, and bounded contention coverage passes with `go test -race`. |
| Security | 0 | 0 | 0 | 0 | Canonical token parsing is asserted from the persisted suffix. Public provider boundaries now return sanitized `OpError` values that retain `redis.ErrClosed`; the regression test verifies the raw key is absent and the outer operation is `campaign`. |
| Operator/Ops | 0 | 0 | 0 | 0 | Redis key layout, ZSET member prefix, and Lua server-time semantics remain unchanged. English and Korean package README files describe the GroupElector compatibility and diagnostic contract. |
| Developer/API | 0 | 0 | 0 | 0 | No exported signature changes. Public methods preserve causal matching through `errors.Is` and typed inspection through `errors.As`. Shared Lease helpers remain intentionally excluded because their whole-value owner-token contract conflicts with the composite ZSET member value. |
| User/Caller | 0 | 0 | 0 | 0 | Callers observe the same member-qualified storage value and leader-slot lifecycle. Redis provider diagnostics no longer reveal raw keys or token values. |

## Integration Notes

- `newElectorToken` owns only canonical suffix creation; GroupElector continues
  to store `memberID:<random>` in the ZSET.
- `groupAcquireScript`, `groupReleaseScript`, and `groupRenewScript` remain
  package-local because they manage a composite ZSET member and Redis server
  time, not a whole-value lease.
- Context cancellation is a caller control-flow result, not a provider
  failure; its existing `%w` behavior is retained.
- Kotlin JUnit concurrency helpers do not apply to this Go repository. Existing
  bounded Testcontainers lifecycle/contention tests and `go test -race` are the
  relevant concurrency evidence.

P0=0 P1=0
