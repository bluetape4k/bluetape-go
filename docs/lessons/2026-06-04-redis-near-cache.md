# Lessons Learned — Redis NearCache (2026-06-04)

**Related issue**: #23
**Impact modules**: `cache/redisnear`, `cache`, `testcontainers/redis`

## L1: Testcontainers log readiness is not always connection readiness

### Problem

The first Redis NearCache package test failed once with `connect: connection refused`
immediately after the Redis container helper returned an address.

### Lesson

For Redis Pub/Sub integration tests, add a lightweight `PING` readiness check in
the package-level test helper before constructing subscribers. This keeps the
shared container fixture unchanged while protecting timing-sensitive subscriber
startup tests.

### Evidence

- Initial `go test -count=1 ./cache ./cache/redisnear` failed in
  `TestNearCacheInvalidatesPeerEntries`.
- After adding `waitForRedis`, `go test -count=1 ./cache ./cache/redisnear`
  passed.

## L2: Failure-mode behavior needs a direct test, not only a design note

### Problem

The spec required receive errors to clear the local cache and call `OnError`, but
the first test pass covered malformed messages and close semantics without
forcing a receive error.

### Lesson

Near-cache invalidation tests must include both data-path proof and failure-path
proof: peer invalidation, malformed payload reporting, receive-error local clear,
close idempotency, stress, and cancellation.

### Evidence

- Added `TestNearCacheClearsLocalOnReceiveError`.
- `go test -count=1 ./cache/redisnear` passed.
- `go test -race -count=1 ./cache/redisnear` passed.

## L3: Examples must satisfy the same errcheck gate as production code

### Problem

The compile-only `ExampleNewPubSub` initially used bare `defer client.Close()`
and `defer near.Close()`. `make ci` failed at lint with two errcheck findings.

### Lesson

Even when examples are intentionally minimal, close calls should use a deferred
function and explicitly discard the error when cleanup cannot affect the example
result.

### Evidence

- Initial `make ci` failed in `cache/redisnear/example_test.go`.
- After wrapping deferred closes, `make ci` passed with `0 issues`.
