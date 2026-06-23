# Issue #216 Testcontainers contracts review

Date: 2026-06-23
Scope: `internal/testcleanup`, `testcontainers/{postgres,mysql,redis,kafka,nats}`,
and the matching README locale sets.
Baseline: `daa4aea`
Review mode: main-session 7-tier fallback after native subagent cleanup was
interrupted by the user for latency.

## Verdict

P0=0 P1=0

## 7-Tier Review

1. Performance: PASS
   - Docker-backed package execution is serial. The affected package tests no
     longer call `t.Parallel()`, and `make test` / `make race` run with `-p 1`.
   - No runtime hot path is changed; helpers are test-only.

2. Stability: PASS
   - Cleanup remains bounded through `testcleanup.Terminate`, and new tests cover
     cleanup after skipped subtests plus repeated termination.
   - Start failures now include explicit categories for Docker unavailable,
     image pull failure, readiness timeout, context cancellation, and wrapper
     failure.

3. Security: PASS
   - No production credential or network trust boundary is introduced.
   - Test credentials remain fixed and documented as test-only.

4. Operator/Ops: PASS
   - README files document Docker runtime requirements, serial execution, CI skip
     guidance for non-Docker jobs, and `-p 1` for Testcontainers coverage jobs.
   - Failure messages distinguish environment failure from wrapper failure.

5. Developer/API: PASS
   - Existing `Start` call shape remains source-compatible for callers using
     `*testing.T`; accepting `testing.TB` broadens compatibility for subtests and
     benchmarks.
   - Connection detail key constants are documented and tested:
     `postgres.connection-string`, `mysql.dsn`, `redis.address`,
     `kafka.brokers`, and `nats.url`.

6. User/Caller: PASS
   - README examples show timeout contexts, cleanup, and key-based connection
     detail maps.
   - Returned connection values are unchanged.

7. Evidence/Release: PASS
   - Validation passed locally:
     - `git diff --check`
     - `go test -p 1 -count=1 ./internal/testcleanup ./testcontainers/postgres ./testcontainers/mysql ./testcontainers/redis ./testcontainers/kafka ./testcontainers/nats`
     - `go test -race -p 1 -count=1 ./internal/testcleanup ./testcontainers/postgres ./testcontainers/mysql ./testcontainers/redis ./testcontainers/kafka ./testcontainers/nats`
     - `make fmt-check`
     - `make tidy-check`
     - `make vet`
     - `golangci-lint cache clean && make lint`
     - `make test`
     - `make race`

## Findings

No P0/P1 findings.

P2/P3 follow-up: none.

## Notes

`make lint` initially reported stale findings from a removed sibling worktree
path (`../issue-212-temp-env-output/...`). `golangci-lint cache clean` removed
that stale cache state, and the rerun reported `0 issues`.
