# Lessons Learned - Retrospective P1 Fixes

Date: 2026-07-07 KST
Related issue: #425
Affected packages: `cache`, `ratelimit/redis`

## L1: Same-key load collapse must not borrow one caller's cancellation

### Problem

`cache.Memory.GetOrLoad` used the first same-key caller's context for the shared
`singleflight` load. When that owner caller canceled while another waiter still
had a live context, the waiter received the owner's `context.Canceled`.

### Lesson

For same-key stampede protection, cancellation is caller-owned. A live waiter may
observe that the shared owner call failed, but it must be able to retry under its
own context instead of inheriting another caller's cancellation.

### Evidence

- Added `TestMemorySameKeyCanceledOwnerDoesNotCancelLiveWaiter`.
- The test failed before the fix with `live waiter should retry after owner
  cancellation, got context canceled`.
- `go test -count=1 ./cache ./ratelimit/redis`: PASS.
- `go test -race -count=1 ./cache ./ratelimit/redis`: PASS.

## L2: Redis logical key validation must preserve caller-owned storage bytes

### Problem

`ratelimit/redis` trimmed logical keys before composing the Redis bucket key.
That collapsed `"tenant:blue"` and `" tenant:blue "` into the same bucket even
though the README documented `<key>` as the caller-provided logical key.

### Lesson

Redis packages may inspect `strings.TrimSpace(key)` to reject blank input, but
storage and collision behavior must use the exact caller-provided key unless the
public API explicitly documents and tests canonicalization.

### Evidence

- Added `TestLimiterPreservesCallerOwnedKeys`.
- The test failed before the fix because the spaced key reused the trimmed
  bucket and was rejected.
- `go test -count=1 ./cache ./ratelimit/redis`: PASS.
- `go test -race -count=1 ./cache ./ratelimit/redis`: PASS.
