# Issue 23 Redis Near Cache Spec Review

Issue: #23
Gate: Step 2-R
Date: 2026-06-04
Reviewed spec: `docs/superpowers/specs/2026-06-04-issue-23-redis-near-cache-spec.md`

## Scope

Review the public API, invalidation semantics, Redis Pub/Sub lifecycle,
stress/benchmark requirements, and future RESP3 strategy boundary for the first
Redis NearCache implementation.

## Perspective Findings

| Perspective | P0 | P1 | P2 | P3 | Notes |
|---|---:|---:|---:|---:|---|
| Developer/API | 0 | 0 | 0 | 0 | `cache/redisnear` and `NewPubSub` keep the first strategy explicit and Go-native. |
| Security | 0 | 0 | 0 | 0 | JSON payload has no code execution path; malformed payloads are ignored and reported. |
| Ops/SRE | 0 | 0 | 1 | 0 | Receive errors must not create a tight loop. Spec now requires bounded backoff. |
| User/Caller | 0 | 0 | 0 | 0 | Non-goals and consistency model explain stale-read boundaries and bypassed writes. |

## Local 7-Tier Review

| Tier | P0 | P1 | P2 | P3 | Evidence |
|---|---:|---:|---:|---:|---|
| 1 Security | 0 | 0 | 0 | 0 | Message fields are strings/ints; unknown/malformed payloads are not executed. |
| 2 Ops/SRE reliability | 0 | 0 | 1 | 0 | Subscriber receive error handling requires local clear and bounded retry. |
| 3 Structural impact | 0 | 0 | 0 | 0 | New public package matches package layout policy and isolates Redis strategy. |
| 4 API quality | 0 | 0 | 0 | 0 | API uses constructor-per-strategy and implements existing cache contracts for string keys. |
| 5 Tests/types/silent failure | 0 | 0 | 0 | 0 | Spec requires Testcontainers, stress, cancellation, and close behavior tests. |
| 6 Performance/stability | 0 | 0 | 0 | 0 | Benchmarks are linked to #107; stress remains in #23. |
| 7 Docs/release/evidence | 0 | 0 | 0 | 0 | README/CHANGELOG/package docs/lesson requirements are recorded. |

## Integrated Findings

| Severity | Finding | Resolution |
|---|---|---|
| P1 | `Close` behavior was initially unspecified, allowing stale local reads after invalidation stopped. | Resolved in spec by adding public `ErrClosed` and requiring all cache operations after `Close` to return it. |
| P2 | Receive errors could spin without a retry policy. | Resolved in spec by requiring bounded backoff after receive errors. |

## Rejected Alternatives

- Hide Pub/Sub and RESP3 behind a single runtime enum: rejected because the
  lifecycle and consistency guarantees differ materially.
- Add Ristretto/BigCache in #23: rejected because #107 owns benchmark-driven
  local storage decisions.

## Gate Verdict

P0 = 0
P1 = 0

Step 2-R is closed. The plan may proceed if it preserves `ErrClosed`, bounded
receive retry, Testcontainers peer invalidation, stress tests, and #107
benchmark linkage.
