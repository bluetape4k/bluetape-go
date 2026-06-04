# Cross-Process Stampede Protection

Issue: #117
Date: 2026-06-04

## Lesson

Redis lock-only coordination is not enough for NearCache load collapse because
NearCache peers do not share values. It only serializes loaders; it does not
let waiters reuse the winner's result.

## Decision

Use an explicit `cache/rediscoord` wrapper with a caller-provided codec and a
short-lived token-bound result envelope. Keep `cache/redisnear` invalidation-only
by default.

## Implementation Note

Waiters must fill local state through wrapped `GetOrLoad`, not `Set`, because a
NearCache `Set` publishes peer invalidation. The coordinator result is a local
fill, not a write/invalidation command.

## Operational Note

`LockTTL` bounds both progress and mutual exclusion. If a loader runs past the
lease, another process may acquire and load. Tune the lease for expected loader
duration and treat Redis payload exposure as part of the deployment security
boundary.
