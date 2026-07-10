# Issue #590 Redis Rate Limiter Diagnostic Substrate Migration Spec

## Context

Issue #570 moves Redis-backed packages to the shared `redis` substrate only
where its input contract is compatible. `ratelimit/redis` has one direct
provider boundary: `Limiter.Allow` dispatches its package-owned token-bucket
Lua script through `redis.Cmdable.Eval` and currently formats a provider
failure with `fmt.Errorf`.

## Current Contract Evidence

- `Allow` uses one Lua script with Redis `TIME`, `HMGET`, `HSET`, and
  `PEXPIRE`; the script owns refill arithmetic, token consumption, and retry
  calculation atomically.
- Bucket keys are exactly `bluetape:ratelimit:<trimmed-namespace>:bucket:<key>`.
  A nonblank caller key is retained byte-for-byte, including leading/trailing
  spaces, colons, and braces. Existing tests prove spaced and unspaced keys are
  distinct buckets.
- A zero `IdleTTL` derives a refill-aware package default. A positive explicit
  value must cover at least one complete refill duration.
- Pre-canceled contexts return before Redis dispatch. Successful/rejected
  limiter results and script-result parse failures are package behavior, not
  provider-operation failures.

## Decision

Keep key construction, validation, TTL derivation, and the Lua script local.
Replace only the error returned by `Eval` with a redacted `redis.OpError` using
family `rate limiter`, operation `consume`, and the already-computed exact
bucket key as its correlation input. Preserve the provider cause. If the
context becomes canceled after dispatch, join `ctx.Err()` with the provider
cause before constructing the operation error.

## Alternatives Considered

| Approach | Decision | Reason |
|---|---|---|
| Adopt `redis.KeyBuilder` | Reject | Its structural validation would reject established caller namespace/key values and would change byte-level bucket-key compatibility. |
| Adopt generic TTL validation | Reject | This package's refill-aware default and full-refill lower bound are its public operational contract. |
| Replace the Lua script with shared script helpers | Reject | The shared helpers implement ownership compare/delete/extend, not token-bucket result semantics. |
| Migrate only `Eval` diagnostics | Accept | It standardizes typed, redacted provider failures without changing algorithm, Redis command count, or behavior. |

## Invariants And Acceptance Criteria

1. `Allow` executes the same script once with the same key and arguments.
2. Namespace trimming and caller-key byte preservation remain unchanged.
3. Existing burst, refill, retry, idle-expiration, and concurrent-client
   behavior remain unchanged.
4. Preflight cancellation still returns `ctx.Err()` before Redis I/O.
5. An `Eval` provider failure exposes `*redis.OpError` with family `rate limiter`,
   operation `consume`, and a deterministic redacted bucket-key ID.
6. `errors.Is` retains both the provider cause and a post-dispatch context error.
7. Formatted provider diagnostics reveal no raw namespace, caller key, bucket
   key, script argument, or provider text.
8. No public API, benchmark result, chart, or performance claim changes.

## Risks And Failure Modes

| Risk | Mitigation |
|---|---|
| A wrapper hides `redis.ErrClosed` or context cancellation | Assert both `errors.As` and `errors.Is` in deterministic closed-client and late-context tests. |
| A shared key helper narrows established input support | Keep `bucketKey` local and retain exact-key tests. |
| A script or TTL refactor changes distributed admission | Explicitly exclude script/TTL changes and rerun existing Testcontainers concurrency coverage. |
| Error text leaks operational identifiers | Use the shared redacted `OpError` and marker-based leak assertions. |

## Test Plan

- Add a closed-client `Allow` test that asserts typed error, provider cause,
  family/operation/key-ID, and marker redaction.
- Add a deterministic unit test for the post-dispatch cancellation error
  boundary so both causes remain discoverable without a timing-dependent Redis
  race.
- Retain exact-key, namespace, TTL, burst/rejection, refill, cancellation, and
  `GoroutineStressTester` concurrent-client coverage.
- Run serial Testcontainers normal/race tests, then repository CI with reuse
  disabled and Ryuk enabled.

## Benchmark Decision

No benchmark is run for this diagnostics-only migration. Issue #560 owns any
cross-provider benchmark matrix and its mandatory result table, chart, and
written analysis. A future measurement for this package must update those
three artifacts together.

## Non-Goals

- No exported API, key format, namespace normalization, caller-key behavior,
  Lua algorithm, Redis `TIME`, `IdleTTL`, retry, fairness, or Cluster change.
- No new limiter algorithm, waiting/reservation behavior, Redis primitive, or
  provider benchmark conclusion.
