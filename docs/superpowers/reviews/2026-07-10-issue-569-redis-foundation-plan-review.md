# Issue 569 Step 3-R Plan Review

Date: 2026-07-10 KST
Branch: `feat/issue-569-redis-foundation`
Baseline commit: `6a3a261`
Scope:

- `docs/superpowers/specs/2026-07-10-issue-569-redis-foundation-spec.md`
- `docs/superpowers/plans/2026-07-10-issue-569-redis-foundation-plan.md`

## Initial Findings

The first Step 3-R pass found P1 blockers in the plan/spec contract before
implementation:

| Lane | P0 | P1 | Blocking theme |
|---|---:|---:|---|
| Performance | 0 | 0 | P2 nil-client coverage and stress specificity. |
| Stability | 0 | 2 | Nil Redis client behavior; missing real-client cancellation/deadline proof. |
| Security | 0 | 2 | Owner token debug/structured redaction; public redacted-key error bypass. |
| Operator/Ops | 0 | 1 | Error/test artifacts could expose raw keys or tokens. |
| Developer/API | 0 | 3 | Prefix validation ambiguity; nil client boundary; adjacent string error API. |
| Caller/User | 0 | 1 | Prefix/structural-segment contradiction. |

## Corrections Applied

- Split colon-delimited package prefixes from single structural segments.
- Allowed colon-bearing hash tags to preserve existing `probabilistic/redis`
  namespace/key compatibility for #570.
- Added richer verbatim logical-key preservation cases.
- Required nil `redis.Scripter` and detectable typed nil validation before Redis
  dispatch.
- Added real Redis cancellation/deadline and first-run `Script.Run` integration
  checks.
- Required serial Testcontainers tests and no `t.Parallel` in Redis integration
  tests.
- Added bounded interleaved-owner Redis stress invariants.
- Added `OwnerToken.GoString` and `slog.LogValuer` redaction requirements.
- Replaced adjacent string error constructors with `OpLabels`.
- Required strict `redis-key:<24 lowercase hex>` validation for pre-redacted key
  IDs.
- Required `OpError.Error()` to omit raw keys, owner tokens, and wrapped cause
  strings while preserving causal inspection via `Unwrap`, `errors.Is`, and
  `errors.As`.
- Sanitized planned token test failure messages so CI artifacts do not print raw
  or potentially token-bearing values.
- Extended README/runbook requirements for ownership drift, cancellation,
  post-dispatch indeterminate commit state, Redis script/client errors, cleanup,
  rollback, and no-migration behavior.

## Final Rerun Verdict

| Lane | P0 | P1 | Notes |
|---|---:|---:|---|
| Performance/runtime | 0 | 0 | Nil-client coverage, stress acceptance, serial race commands, and benchmark deferral are explicit. |
| Stability/reliability | 0 | 0 | Nil client, typed nil, real cancellation/deadline, Testcontainers serial behavior, and go-redis fallback proof are covered. |
| Security | 0 | 0 | Token formatting, structured logging, redacted ID validation, sanitized OpError output, and `OpLabels` are covered. |
| Operator/Ops | 0 | 0 | P2 test-artifact concern was patched after rerun; no P0/P1 remained. |
| Developer/API | 0 | 0 | Hash-tag colon compatibility was patched and rerun cleanly. |
| Caller/User | 0 | 0 | Prefix semantics, logical-key preservation, examples, and README parity are covered. |

## Main-Session Integration Verdict

`P0=0 P1=0`

Implementation may proceed through Step 4 TDD. The implementation must preserve
the reviewed boundaries:

- #569 creates the new `redis` package only and does not migrate existing Redis
  package behavior.
- #570 must add old/new key parity tests before migrating existing Redis
  packages.
- Benchmark/provider comparison remains deferred to #560/#570; this issue must
  not claim migrated hot-path performance.
- Redis Testcontainers verification must run serially with `-p 1`.
