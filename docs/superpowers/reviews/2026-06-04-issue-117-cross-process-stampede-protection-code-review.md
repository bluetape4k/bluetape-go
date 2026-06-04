# Issue #117 Cross-Process Stampede Protection Code Review

Date: 2026-06-04
Scope: `cache/rediscoord`, docs, benchmark target
Review type: 7-Tier implementation review

## Verdict

PASS. `P0 = 0`, `P1 = 0`, `P2 = 0`.

## 7-Tier Findings

| Tier | Focus | P0 | P1 | P2 | P3 | Finding |
|---|---|---:|---:|---:|---:|---|
| 1 Requirements | #117 acceptance | 0 | 0 | 0 | 0 | Research, API decision, Redis TTL lock, multi-near-cache collapse, cancellation, expiry, docs, and benchmark boundary are implemented. |
| 2 API/UX | Public surface | 0 | 0 | 0 | 1 | `NewStampedeCache`, `Options`, `Codec`, and `JSONCodec` are explicit. Users must opt in and choose serialization. |
| 3 Integration | cache/redisnear/lock/redis | 0 | 0 | 0 | 0 | Waiters fill local state through wrapped `GetOrLoad`, avoiding accidental NearCache invalidation publish. |
| 4 Security/data | Redis payload exposure | 0 | 0 | 0 | 1 | Payload exposure is documented in README; caller controls codec and Redis isolation. |
| 5 Tests/types | Silent failures and races | 0 | 0 | 0 | 0 | Unit, Testcontainers, stress, cancellation, expiry, race, and benchmark smoke coverage pass. |
| 6 Performance/stability | Lock TTL, polling, benchmark | 0 | 0 | 0 | 1 | Polling is context-bound; benchmark target is opt-in. Loader over-lease behavior is documented. |
| 7 Docs/ops | Release notes and WIP | 0 | 0 | 0 | 0 | README pair, CHANGELOG, WIP, research index, verifier, and lessons are updated. |

## Review Notes

- Token-bound envelopes prevent waiters from accepting stale result data from a
  different owner attempt.
- `ensureOwner` reads the lease's Redis key directly, so user cache keys do not
  need parsing or escaping for owner checks.
- Unlock uses a short background context so caller cancellation does not skip
  cleanup.
- The package intentionally returns an error when result publication fails after
  a local fill; this surfaces a failed cross-process guarantee instead of hiding
  coordination loss.

## Residual Risk

- A durable Redis L2 cache may still be useful later for applications that want
  shared values outside a cold burst. That is intentionally outside #117.
- Workloads with loaders longer than `LockTTL` must tune the lease or accept
  possible overlapping loads after expiry.

Gate verdict: PASS. No blocker for PR.
