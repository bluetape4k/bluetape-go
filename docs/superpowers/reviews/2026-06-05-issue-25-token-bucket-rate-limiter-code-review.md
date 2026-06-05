# Issue #25 7-Tier Code Review

Issue: #25
Milestone: 0.3.0
Date: 2026-06-05
Diff base: `origin/develop`
Scope: `ratelimit`, `ratelimit/redis`, README pair, package READMEs,
`CHANGELOG.md`, `WIP.md`, research/spec/plan/review artifacts.

## Reviewed Evidence

- Local limiter API and state: `ratelimit/result.go`, `ratelimit/options.go`,
  `ratelimit/token_bucket.go`.
- HTTP middleware: `ratelimit/http.go`.
- Redis limiter: `ratelimit/redis/options.go`, `ratelimit/redis/limiter.go`.
- Tests and benchmarks: `ratelimit/*_test.go`,
  `ratelimit/redis/*_test.go`, `ratelimit/*benchmark_test.go`.
- Docs: `ratelimit/README.md`, `ratelimit/redis/README.md`, root README pair,
  `CHANGELOG.md`, `WIP.md`.
- Validation: targeted tests, race tests, `make bench-ratelimit`, `make ci`,
  wiki preservation, GNO evidence.

## Tier Findings

| Tier | P0 | P1 | P2 | P3 | Evidence |
|---|---:|---:|---:|---:|---|
| Tier 1 Security | 0 | 0 | 0 | 0 | `RemoteIPKey` uses `Request.RemoteAddr` only and `TestRemoteIPKeyDoesNotTrustProxyHeaders` locks proxy-header non-trust. Redis logical key length is bounded by `MaxKeyBytes`. |
| Tier 2 Ops/SRE reliability | 0 | 0 | 0 | 0 | Rejection is normal result state; Redis/backend errors remain errors; context cancellation is tested in local and Redis packages. Redis keys use `PEXPIRE` through the Lua script. |
| Tier 3 Structural impact | 0 | 0 | 0 | 0 | New packages only. Existing `cache`, `resilience`, `lock/redis`, and `go.mod` public contracts are unchanged. |
| Tier 4 Go/API quality | 0 | 0 | 0 | 0 | API uses `context.Context`, small interfaces, `net/http`, caller-owned `redis.Cmdable`, and package-local validation. Public comments are short and Go-native. |
| Tier 5 Tests/types/silent failure | 0 | 0 | 0 | 0 | Local, Redis, HTTP, cancellation, stress, TTL, namespace, over-burst, and benchmark paths are covered. Weak no-op tests were not found. |
| Tier 6 Performance/stability | 0 | 0 | 0 | 0 | Hot local paths benchmark at 0 allocations for direct `Allow`; Redis consume/refill is one script call per attempt; local map has `IdleTTL` pruning; Redis keys expire with `PEXPIRE`. |
| Tier 7 Docs/release/evidence | 0 | 0 | 1 | 0 | README pair, package READMEs, CHANGELOG, WIP, Makefile benchmark target, research/spec/plan/review artifacts exist. GNO docs collection cannot index feature worktree documents until merge/local sync. |

## Current-Session Integration

| ID | Severity | Area | Finding | Resolution |
|---|---|---|---|---|
| CR-1 | P2 | Evidence | `bluetape4k-docs` GNO search does not see feature-worktree docs. | Recorded in verifier; rerun after merge/local sync. |

## Validation Snapshot

| Command | Result |
|---|---|
| `go test -count=1 ./ratelimit ./ratelimit/redis` | PASS, 44 tests |
| `go test -race -count=1 ./ratelimit ./ratelimit/redis` | PASS, 44 tests |
| `go test -count=1 ./ratelimit ./ratelimit/redis ./lock/redis` | PASS before final local over-burst addition |
| `make bench-ratelimit` | PASS |
| `make ci` | PASS after final local over-burst test addition |
| `git diff --check` | PASS |

## Convergence Verdict

- P0: 0
- P1: 0
- P2: 1 accepted with explicit post-merge validation condition
- P3: 0

Step 6-R gate status: PASS.
