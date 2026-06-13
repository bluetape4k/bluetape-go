# Issue #175 Step 2-R Spec Review

Issue: #175
Date: 2026-06-14
Spec: `docs/superpowers/specs/2026-06-14-issue-175-jwt-provider-cache-adapters-design.md`
Gate: 7-Tier = 6 independent lanes + main integration review
Wait SLA: subagent wait max 10 minutes; no lane timed out.

## Initial 7-Tier Results

| Tier | Perspective | Verdict | P0 | P1 | P2 | P3 |
|---|---|---:|---:|---:|---:|---:|
| 1 | Performance | REQUEST_CHANGES | 0 | 1 | 2 | 0 |
| 2 | Stability | REQUEST_CHANGES | 0 | 3 | 1 | 0 |
| 3 | Security | REQUEST_CHANGES | 0 | 2 | 3 | 2 |
| 4 | Operator/Ops | COMMENT | 0 | 0 | 2 | 1 |
| 5 | Developer/API | REQUEST_CHANGES | 0 | 1 | 2 | 2 |
| 6 | User/Caller | COMMENT | 0 | 0 | 3 | 1 |
| Main | Integration | REQUEST_CHANGES | 0 | 7 | 13 | 6 |

## Blocking Findings Fixed

| Finding | Resolution |
|---|---|
| Cold same-token miss stampede after rejecting `LoadingCache.GetOrLoad` | Required per-adapter `singleflight.Group`, dynamic TTL inside loader, and concurrent same-token tests. |
| Custom parse-clock cache bypass not implementable | Required `parseConfig.customClock` plus cache-profile normalizer returning `cacheable=false`. |
| Cache key missing provider/trust identity | Added key prefix, trust scope, provider algorithm, default per-adapter random scope, and explicit scope rules. |
| `*Reader` cache backend trust boundary unclear | Restricted first slice to trusted application-process cache backends; untrusted/shared external cache is a non-goal. |
| Cache operation error policy unclear | Required only `cache.ErrCacheMiss` to fall through, non-miss errors visible, stale delete failure blocks stale reader, clear failures visible. |
| Nil/canceled context behavior ambiguous | Required nil context rejection, cancellation/deadline preservation, and no cache mutation/delegation after already-done contexts. |
| Constructor validation unclear | Added nil/typed-nil provider/cache checks, nil option rejection, TTL/prefix/scope/clock validation, and `OptionError` option names. |

## Affected-Lane Rerun

| Tier | Rerun Scope | Verdict | P0 | P1 |
|---|---|---:|---:|---:|
| 1 | Performance blocking fixes | APPROVE | 0 | 0 |
| 2 | Stability blocking fixes | APPROVE | 0 | 0 |
| 3 | Security blocking fixes | APPROVE | 0 | 0 |
| 5 | Developer/API blocking fixes | APPROVE | 0 | 0 |

## Main Integration Verdict

APPROVE.

Final gate:

- P0 = 0
- P1 = 0

The updated spec now constrains cache identity, backend trust, hit
revalidation, miss coalescing, custom-clock bypass, context behavior, and cache
error propagation tightly enough for Step 3 implementation planning.

## Verification Evidence

- `node scripts/generate-jwt-provider-cache-adapter-diagram.mjs`
  - `badEndpointAngle=0`
  - `badBends=0`
  - `interiorCrossings=0`
  - `nodeOverlaps=0`
  - `marginImbalance=0`
  - `margins=L48/R48/T48/B48`
- `go test -count=1 ./jwt ./cache ./testing/concurrency`
  - PASS
- `git diff --check`
  - PASS

## DoD

| Item | Status |
|---|---|
| Six independent lanes run | Done |
| Main integration review run | Done |
| Completed agents closed | Done |
| Timeout fallback recorded if needed | N/A |
| P0/P1 normalized | Done |
| Blocking findings fixed in spec | Done |
| Affected lanes rerun | Done |
| Convergence P0=0 P1=0 | Done |
