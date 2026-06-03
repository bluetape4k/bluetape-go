# LeaderGroupElector

## Context

Issue #85 adds a Redis-backed group elector to `bluetape-go` so at most
`MaxLeaders` workers can run a coordinated task concurrently.

## Decision

- Keep the Go Redis group key separate from Kotlin/JVM Redis group keys.
- Use a Redis ZSET with `memberID:random` tokens and server-time expiry scores.
- Make `Campaign` context-bounded instead of returning immediate `ErrNotLeader`
  on full slots, because group election is semaphore-like.

## Outcome

The implementation follows the existing single-elector lifecycle shape while
adding count/status methods and expiry reclamation.

## Verification

Planned gates: targeted `go test` for `leader` and `leader/redis`, then
`make ci`, diff check, and local 7-tier review.

## Future Guard

When adding Redis coordination primitives, keep ownership tokens opaque, use
server-side time for expiry decisions, and document Kotlin/Go non-interop unless
an explicit compatibility adapter exists.
