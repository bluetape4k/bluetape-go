# Issue #25 Verifier

Issue: #25
Milestone: 0.3.0
Date: 2026-06-05
Spec: `docs/superpowers/specs/2026-06-05-issue-25-token-bucket-rate-limiter-spec.md`
Plan: `docs/superpowers/plans/2026-06-05-issue-25-token-bucket-rate-limiter-plan.md`

## Scope Verified

- `ratelimit` local keyed token-bucket limiter.
- `ratelimit` standard-library HTTP middleware.
- `ratelimit/redis` Redis-backed distributed token-bucket limiter.
- Tests, stress coverage, cancellation coverage, benchmarks, README pair,
  package READMEs, `CHANGELOG.md`, `WIP.md`, and external research preservation.

## Spec DoD Mapping

| Spec requirement | Evidence | Status |
|---|---|---|
| Root `ratelimit` package exists. | `ratelimit/doc.go`, `ratelimit/token_bucket.go`, `ratelimit/http.go` | PASS |
| Redis-backed package exists. | `ratelimit/redis/doc.go`, `ratelimit/redis/limiter.go` | PASS |
| Local refill rate and burst capacity. | `TestTokenBucketAllowsBurstAndReportsRejection`, `TestTokenBucketRefillsWithClock` | PASS |
| Redis limiter safe for concurrent clients. | `TestLimiterConcurrentClientsDoNotOverAdmitBurst`; Redis `Eval` script in `ratelimit/redis/limiter.go` | PASS |
| HTTP middleware example. | `ExampleNewHandler`, `ratelimit/README.md` | PASS |
| Stress tests use `GoroutineStressTester`. | `TestTokenBucketStressDoesNotOverAdmitBurst`, `TestLimiterConcurrentClientsDoNotOverAdmitBurst` | PASS |
| Cancellation tests use `AsyncJobTester`. | `TestTokenBucketAsyncJobTesterCoversCancellation`, `TestLimiterAsyncJobTesterCoversCancellation` | PASS |
| Local benchmarks are present. | `BenchmarkTokenBucketAllowAllowed`, `BenchmarkTokenBucketAllowRejected`, `BenchmarkHandlerAllowed`; `make bench-ratelimit` | PASS |
| README English/Korean and CHANGELOG updated. | `README.md`, `README.ko.md`, `CHANGELOG.md` | PASS |
| No new dependency is added. | `go.mod`/`go.sum` unchanged | PASS |

## Plan Task Mapping

| Task | Evidence | Status |
|---|---|---|
| T1 Shared API scaffold | `Limiter`, `Result`, package docs | PASS |
| T2 Local options/state | `Options.normalize`, `TokenBucket`, unexported `newWithClock` | PASS |
| T3 Local algorithm | refill/consume/retry/reset/context validation tests | PASS |
| T4 HTTP middleware | handler tests for allow, reject, backend error, custom handler, remote IP | PASS |
| T5 Local stress/benchmarks | stress/cancel tests and `make bench-ratelimit` output | PASS |
| T6 Redis options/script | Redis options validation, overflow checks, Lua script | PASS |
| T7 Redis integration tests | Testcontainers tests for burst, refill, namespace, TTL, concurrency, cancellation | PASS |
| T8 Docs/repo metadata | README pair, package READMEs, CHANGELOG, WIP, wiki note pushed | PASS |
| T9 Reviews/verifier/lessons | This verifier plus Step 6-R artifact; lessons pending Step 7 | IN PROGRESS |

## Validation Evidence

| Command | Result |
|---|---|
| `go test -count=1 ./ratelimit ./ratelimit/redis ./lock/redis` | PASS, 58 tests before final local over-burst addition |
| `go test -count=1 ./ratelimit ./ratelimit/redis` | PASS, 44 tests |
| `go test -race -count=1 ./ratelimit ./ratelimit/redis` | PASS, 44 tests |
| `make bench-ratelimit` | PASS; local allowed/rejected and HTTP allowed benchmarks ran |
| `make ci` | PASS after final local over-burst test addition |
| `git diff --check` | PASS |
| `gno update` + `gno embed --collection bluetape4k-wiki` | PASS in `bluetape4k-wiki`; research note pushed at `2ac234d` |

## Known Evidence Gap

`gno search "Issue #25 Token-Bucket Rate Limiter" -c bluetape4k-docs` did not
return the feature-worktree documents because the `bluetape4k-docs` collection
indexes the main repo path, not this linked worktree. This is not a code
blocker. Re-run docs GNO update/search after merge/local sync.

## Verdict

VERIFIED with one non-blocking evidence timing gap:

- P0: 0
- P1: 0
- P2: 1 docs GNO worktree indexing gap, deferred to post-merge Step 10
