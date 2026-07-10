# Issue #588 Redis Cache Coordinator Substrate Migration Spec

## Context

Issue #570 adopts the shared Go-native `redis` substrate in narrow,
compatibility-preserving slices. `cache/rediscoord` already delegates owner
lease acquisition and release to the migrated `lock/redis` package, but its
direct Redis `GET` and `SET` paths still format provider errors locally.

## Current Contract Evidence

- `StampedeCache` stores lock keys as `<prefix>:<namespace>:lock:<caller-key>`
  and result keys as `<prefix>:<namespace>:result:<caller-key>`.
- `Namespace` and caller keys are intentionally retained verbatim. A blank
  namespace is rejected, but leading/trailing non-blank whitespace and
  delimiter characters remain supported.
- Result envelopes compare their `Token` as an opaque string. They can contain
  values written by prior package versions and must not require canonical
  `redis.OwnerToken` parsing.
- `lock/redis` owns acquire/release behavior, generated lock tokens, and safe
  compare-and-delete. This package must not copy that behavior.
- The package README already includes local Testcontainers benchmark results,
  a latency chart, and a written analysis.

## Decision

Keep all key construction, duration normalization, result-envelope encoding,
and `lock/redis` delegation local. Migrate only direct Redis provider failures
to `redis.OpError` using the existing raw key as input for a redacted
correlation identifier.

The family is `cache coordination`; operations are `result-get`, `owner-get`,
`owner-check`, and `result-set`. A provider failure preserves its original
cause through `errors.Is` and exposes `*redis.OpError` through `errors.As`.
When a command returns after the caller context is done, the wrapped cause also
retains the context error by joining it before `redis.OpError` construction.

## Alternatives Considered

| Approach | Decision | Reason |
|---|---|---|
| Adopt `redis.KeyBuilder` | Reject | Its structural validation would reject existing supported namespace values and cannot place the `lock`/`result` segment before an arbitrary caller key without changing key bytes. |
| Parse envelope tokens as `redis.OwnerToken` | Reject | Envelope compatibility intentionally accepts opaque historical owner values. |
| Replace the `lock/redis` lease with a new local shared lease | Reject | The package already depends on the migrated owner-safe lock boundary; duplicating it increases correctness risk. |
| Migrate provider errors only | Accept | It standardizes diagnostics while preserving all visible storage and coordination behavior. |

## Invariants And Acceptance Criteria

1. `lockKey` and `resultKey` keep their current bytes for caller keys and
   namespaces, including whitespace, colons, and brace characters.
2. `LockTTL`, `ResultTTL`, and `PollInterval` retain their current zero/default
   and any-positive-duration behavior.
3. Result envelopes retain opaque token equality; no token normalization or
   schema version change is introduced.
4. `redis.Nil`, `redislock.ErrNotAcquired`, cache sentinel errors, and preflight
   `ctx.Err()` retain their existing control-flow behavior.
5. Direct provider failures use a `*redis.OpError` with family
   `cache coordination`, the operation label for the dispatched command, a
   deterministic redacted key ID, and the original cause.
6. Error text reveals neither raw key nor owner token nor provider error text.
7. If a command completes after cancellation/deadline, `errors.Is` matches both
   the provider cause and `ctx.Err()`.
8. No algorithm, command count, polling cadence, or benchmark claim changes.

## Test Plan

- First add closed-client regression tests for result read, owner read,
  owner check, and result write. Each asserts `errors.Is(err, redis.ErrClosed)`,
  `errors.As(err, *redis.OpError)`, operation/family/key-ID values, and marker
  redaction.
- Add a post-dispatch canceled-context test to prove both the provider cause
  and `context.DeadlineExceeded` remain discoverable.
- Keep existing Testcontainers tests serial. Run focused normal and race
  package tests, then `make ci` before PR publication.

## Benchmark Decision

No benchmark is run for this behavior-preserving error-boundary migration. The
existing README snapshot remains historical evidence only. If this work later
runs a measurement, its result table, chart, and written analysis must be
updated together. Issue #560 owns the ecosystem/provider benchmark matrix.

## Non-Goals

- No public API, Redis key, namespace, TTL, envelope, polling, or cache
  invalidation behavior change.
- No new Redis primitive, script, retry loop, background worker, or benchmark
  conclusion.
