# Issue #25 Token-Bucket Rate Limiter Plan

Issue: #25
Milestone: 0.3.0
Spec: `docs/superpowers/specs/2026-06-05-issue-25-token-bucket-rate-limiter-spec.md`
Research: `docs/research/2026-06-05-issue-25-token-bucket-rate-limiter.md`

## Implementation Strategy

Add a first-party `ratelimit` package with a keyed local token bucket and
standard-library HTTP middleware. Add `ratelimit/redis` as the distributed
backend using one Redis Lua script per `Allow` call. Keep rejection as normal
`Result` state and reserve errors for invalid input, context cancellation, and
backend failures.

## Tasks

| Task | Scope | Work | Validation |
|---|---|---|---|
| T1 Shared API scaffold | `ratelimit/doc.go`, `ratelimit/result.go`, `ratelimit/errors.go` | Define `Limiter`, `Result`, validation helpers, context normalization, and package docs. | `go test -count=1 ./ratelimit -run 'Test.*Validation|Test.*Interface'` |
| T2 Local options/state | `ratelimit/options.go`, `ratelimit/token_bucket.go` | Normalize `RatePerSecond`, `Burst`, `IdleTTL`; store per-key state with mutex and idle cleanup; add unexported test clock constructor. | option/default tests |
| T3 Local algorithm | `ratelimit/token_bucket.go` | Implement refill, consume, retry-after, reset-after, and context checks. | burst/refill/rejection tests |
| T4 HTTP middleware | `ratelimit/http.go`, examples | Add `NewHandler`, default remote-IP keying without proxy-header trust, 429/503 behavior, `Retry-After`, custom handler. | HTTP tests and compile-checked examples |
| T5 Local stress/benchmarks | `ratelimit/*_test.go`, benchmark files | Use `GoroutineStressTester`, `AsyncJobTester`, and add focused local benchmarks. | `go test -race -count=1 ./ratelimit`; benchmark smoke |
| T6 Redis options/script | `ratelimit/redis/options.go`, `ratelimit/redis/limiter.go` | Normalize Redis options, key builder, microtoken conversion with overflow checks, Lua script with `TIME`, `HSET`, `PEXPIRE`. | Redis option tests and Testcontainers burst tests |
| T7 Redis integration tests | `ratelimit/redis/*_test.go` | Cover concurrent clients, refill, namespace isolation, key TTL, cancellation, and interface conformance. | `go test -count=1 ./ratelimit/redis`; race run |
| T8 Docs and repo metadata | README pair, package READMEs, `CHANGELOG.md`, `WIP.md`, research index, `bluetape4k-wiki` research note | Document package boundaries, usage, rejection semantics, Redis consistency, benchmark boundary, and preserved external official-doc evidence. | docs grep, `git diff --check`, `gno update`, `gno embed --collection bluetape4k-wiki` |
| T9 Reviews/verifier/lessons | `docs/superpowers/reviews`, `docs/lessons` | Write verifier artifact, 7-Tier code review, and lessons. | review gate `P0 = 0`, `P1 = 0` |

## Detailed Local Algorithm

For each key, the local limiter stores:

```text
tokens      float64
updatedAt   time.Time
lastSeenAt  time.Time
```

`Allow`:

1. normalize context and fail fast on cancellation;
2. validate key and requested token count;
3. lock limiter mutex;
4. prune idle entries opportunistically when the current key is touched;
5. initialize a missing key with `Burst` tokens;
6. refill tokens by elapsed seconds multiplied by `RatePerSecond`, capped at
   `Burst`;
7. if enough tokens exist, subtract and return `Allowed=true`;
8. otherwise compute retry-after and return `Allowed=false`.

The local implementation may keep fractional tokens internally but returns
whole-token diagnostics.

Tests use an unexported constructor with a manual clock so refill and idle
cleanup assertions do not sleep.

## Detailed Redis Algorithm

`Allow`:

1. normalize context and fail fast on cancellation;
2. validate key and requested token count;
3. build one Redis key from prefix, namespace, and logical key;
4. call `Eval` with:
   - requested microtokens;
   - burst microtokens;
   - refill microtokens per second;
   - idle TTL milliseconds;
5. convert the script array result into `ratelimit.Result`.

Lua script:

1. `TIME` -> now milliseconds;
2. `HMGET tokens updated_ms`;
3. missing state initializes to full burst and current time;
4. elapsed milliseconds refills microtokens;
5. allowed branch subtracts requested microtokens;
6. rejected branch leaves tokens unchanged after refill;
7. `HSET` updated state;
8. `PEXPIRE` idle TTL;
9. return `[allowed, remaining_tokens, retry_after_ms, reset_after_ms]`.

## Error Handling Decisions

- Invalid limiter options fail constructor calls.
- Invalid per-call key or token count returns validation errors.
- Context cancellation returns `ctx.Err()` before backend work.
- Redis `Eval` errors are wrapped and returned as backend errors.
- Rejection is not an error.
- HTTP default handler maps backend errors to 503 and normal rejection to 429.

## Test Matrix

| Behavior | Local unit | Redis/Testcontainers | HTTP | Stress | Race |
|---|---:|---:|---:|---:|---:|
| Option validation/defaults | Yes | Yes | N/A | No | No |
| Burst and rejection | Yes | Yes | Yes | Yes | Yes |
| Refill timing | Yes with fake clock | Yes with short wait | No | No | Yes |
| Context cancellation | Yes | Yes | Yes through request context | Yes via `AsyncJobTester` | Yes |
| Concurrent no over-admission | Yes | Yes | No | Yes via `GoroutineStressTester` | Yes |
| Namespace isolation | N/A | Yes | N/A | No | Yes |
| Key expiration/idle cleanup | Yes | Yes | N/A | No | Yes |
| Custom HTTP key/error handling | N/A | N/A | Yes | No | No |
| Benchmarks | Yes | Optional/follow-up | Yes | N/A | N/A |

## Documentation Notes

README wording must distinguish:

- `ratelimit`: process-local keyed limiter and HTTP middleware;
- `ratelimit/redis`: distributed Redis-backed limiter for multiple processes;
- local rejection is normal control flow;
- Redis backend failures are operational errors;
- default remote-IP keying is a development convenience, not a production
  tenant/auth boundary;
- default remote-IP keying does not trust proxy headers; applications behind a
  trusted proxy must provide `KeyFunc`.

## Validation Sequence

1. `go test -count=1 ./ratelimit`
2. `go test -race -count=1 ./ratelimit`
3. `go test -count=1 ./ratelimit/redis`
4. `go test -race -count=1 ./ratelimit/redis`
5. `go test -count=1 ./ratelimit ./ratelimit/redis ./lock/redis`
6. `go test -run '^$' -bench '^Benchmark' -benchmem ./ratelimit`
7. `make ci`
8. `git diff --check`
9. `gno update`
10. `gno embed --collection bluetape4k-docs`
11. `gno embed --collection bluetape4k-wiki`
12. `gno search "Issue #25 Token-Bucket Rate Limiter" -c bluetape4k-docs -n 10 --files`
13. `gno search "token bucket Redis Lua rate limiter" -c bluetape4k-wiki -n 10 --files`

## Risks And Mitigations

| Risk | Mitigation |
|---|---|
| Local key map grows without bound. | `IdleTTL` default and opportunistic pruning. |
| Redis clients with clock skew over-admit. | Use Redis `TIME` inside the Lua script. |
| Fractional refill creates surprising diagnostics. | Store microtokens in Redis and return whole-token diagnostics. |
| HTTP middleware encourages IP-only production limits. | README documents custom authenticated tenant keying. |
| Redis benchmark expands scope. | Add only local benchmarks now; create follow-up for Redis benchmark if needed. |
| Very large rate/burst overflows microtokens. | Constructor rejects unsafe values. |

## Definition Of Done

- T1-T9 complete.
- Required validation sequence passes or blockers are documented with exact
  command output.
- Step 2-R, Step 3-R, and Step 6-R reports show `P0 = 0` and `P1 = 0`.
- PR is opened with issue link, validation evidence, and merge approval request.
