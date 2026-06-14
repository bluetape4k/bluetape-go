# Issue #179 CI Fix Review

## Trigger

PR #234 CI failed in workflow `27486100283` during `make coverage`.

Failing tests:

- `TestCachedProviderAsyncCancellationDoesNotCache`
- `TestCachedProviderSetFailureIsVisible`
- `TestCachedProviderLiveWaiterRetriesAfterCanceledOwner`

## Root Cause

`CachedProvider` and `CachedDistributedProvider` defaulted cache TTL/revalidation time to `time.Now`, even when the wrapped provider used a deterministic `WithClock`.

The cached provider tests compose tokens with provider clock `2026-06-14T01:00:00Z`. Once the real wall clock moved past the token's one-hour expiration, the cache layer saw the freshly parsed token as expired, computed `ttl=0`, skipped `cache.Set`, and caused:

- set failures to be hidden because `cache.Set` was never called,
- async cancellation tests to wait for a cache set signal that never happened,
- live waiter retry tests to block at the same set boundary.

## Fix

- Track whether `WithCacheClock` was explicitly supplied.
- If no explicit cache clock is supplied, inherit the wrapped provider clock.
- Preserve explicit `WithCacheClock` behavior.
- Add deterministic regression tests for both in-memory and distributed cached providers using a provider clock in year 2000 and a wall-clock-backed spy cache.

## Review Mode

7-Tier gate executed as six independent review lanes plus main integration review.

Native subagents were not used for this gate because this session showed unstable child-agent waits and the operator instruction was to continue with main-session role switching. Main integration fallback performed.

## Lane 1: Performance

Verdict: PASS.

The fix adds one boolean field and one constructor-time assignment. There is no new per-parse allocation, lock, goroutine, I/O, or cache call.

## Lane 2: Stability And Concurrency

Verdict: PASS.

The root cause was a clock-domain mismatch, not a singleflight race. Regression tests now force the mismatch deterministically. `go test -race -count=1 -timeout=120s ./jwt` passed.

## Lane 3: Security

Verdict: PASS.

The change does not alter token validation, signing keys, trust scopes, cache keys, or cryptographic behavior. It only aligns cache TTL/revalidation time with the provider clock unless callers explicitly override the cache clock.

## Lane 4: Operator And Operations

Verdict: PASS.

The CI failure mode is documented with workflow and test names. Local `make coverage` and `make race` now pass on the branch before pushing the fix.

## Lane 5: Developer And API

Verdict: PASS.

No public API change. Existing `WithCacheClock` semantics are preserved. The new internal `customNow` flag records whether the option was set.

## Lane 6: User And Caller

Verdict: PASS.

Default cache behavior is now less surprising for callers that configure a provider clock for deterministic tests or controlled runtime clocks. Explicit cache clock users keep full control.

## Evidence

- `go test -count=1 -timeout=60s ./jwt -run 'TestCachedProvider(SetFailureIsVisible|AsyncCancellationDoesNotCache|LiveWaiterRetriesAfterCanceledOwner)'`: PASS
- `go test -count=1 -timeout=120s ./jwt`: PASS
- `go test -race -count=1 -timeout=120s ./jwt`: PASS
- `go test -count=1 ./money ./testing/concurrency`: PASS
- `go test -race -count=1 ./money ./testing/concurrency`: PASS
- `make fmt-check`: PASS
- `make tidy-check`: PASS
- `make vet`: PASS
- `make lint`: PASS
- `make coverage`: PASS
- `make race`: PASS
- `git diff --check`: PASS

## Main Integration Review

P0 findings: 0.

P1 findings: 0.

P2 findings: 0.

P3 findings: 0.

Integrated verdict: PASS. Re-run GitHub CI after pushing the fix.
