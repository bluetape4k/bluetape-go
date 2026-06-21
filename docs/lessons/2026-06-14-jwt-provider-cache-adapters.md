# Lessons Learned - JWT Provider Cache Adapters (2026-06-14)

**Related issue**: #175
**Related PR**: #230
**Affected modules**: `jwt`, `jwt/redis`, `testing/concurrency`

## L1: Cached JWT readers need stale-hit revalidation tests

### Problem

A cached `jwt.Reader` can become invalid after key rotation, algorithm changes,
or removed key ids. Warm-hit tests alone do not prove that a stale cached reader
is deleted and reparsed through the live provider path.

### Lesson

For JWT cache adapters, test both local and distributed stale-hit branches. Seed
the cache with invalid reader states, verify delete/reparse behavior, and cover
nil reader, wrong algorithm, unknown key id, and expired-key or expired-token
no-recache cases.

### Evidence

- `jwt/cached_provider_test.go`
- `jwt/cached_distributed_provider_test.go`
- Step 6-R Security lane reached `P0=0 P1=0` after stale-hit and TTL proof tests
  were added.

## L2: Cancellation tests must prove no cache write completed

### Problem

An async cancellation test can pass by observing a canceled caller, while an
in-flight cache owner still completes `Set` and leaves a stale entry behind.
That is a correctness gap for context-aware cache APIs.

### Lesson

Cancellation tests for cache adapters should assert both the caller-visible
error and the storage side effect. Use the repository concurrency helpers where
they fit, then assert no completed `Set` and no retained entry after the helper
returns.

### Evidence

- `jwt/cache_failure_test.go` uses `AsyncJobTester`.
- Security rerun attempt 2 found a P2 proof gap; main integration added
  assertions for `sets == 0` and `entries == 0`.
- Fresh targeted test, race test, and `make ci` passed after the assertion fix.

## L3: Review lane timeout is recoverable work, not immediate failure

### Problem

Native subagent lanes can exceed the 10-minute SLA even when the review
perspective is still valuable. Leaving the lane as a final timeout too early
turns a runtime delay into weaker review evidence.

### Lesson

For Step 2-R, Step 3-R, Step 6-R, and Step 7-R, close timed-out lanes and rerun
the same perspective up to 3 times with a fresh gate-scoped agent before final
main-session fallback. The main session must continue local verification while
subagents run.

### Evidence

- Step 6-R Security attempt 1 timed out after the 10-minute SLA.
- Security attempt 2 completed with `P0=0 P1=0 P2=1 P3=0`.
- `bluetape4k-workflow` and `bluetape4k-full-feature` were updated in live
  skill files and chezmoi source with matching retry-then-fallback rules.
