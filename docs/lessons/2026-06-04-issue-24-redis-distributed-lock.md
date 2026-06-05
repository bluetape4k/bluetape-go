# Lessons Learned - Issue 24 Redis Distributed Lock

Date: 2026-06-04 KST
Related issue: #24
Affected package: `lock/redis`

## L1: Preserve Caller-Owned Redis Keys

### Problem

The first implementation used `strings.TrimSpace` for both validation and
storage. The spec intended trim-based blank validation only, so a caller key
like `" locks:billing-rollup "` would have been silently changed.

### Lesson

For Redis keys, validation may inspect a normalized value, but the package
should store and use the exact caller-provided key unless the API explicitly
documents canonicalization.

### Evidence

- Added `TestNewPreservesRedisKeyVerbatim`.
- `go test -count=1 ./lock/redis`: PASS, 15 tests.
- `make ci`: PASS.

## L2: Stress Probes Must Not Create Their Own Race Window

### Problem

The same-key contention stress test originally decremented its active-owner
counter after releasing the Redis key. A new owner could acquire the key during
that small gap, causing a false positive even though Redis ownership was safe.

### Lesson

When a stress test measures critical-section overlap, end the test-level
critical section before releasing the external lock. Otherwise the probe can
report a test instrumentation race as a product race.

### Evidence

- `go test -count=5 ./lock/redis -run 'TestMutexSameKeyContentionStress|TestMutexAsyncCancellationDoesNotLeakKey'`: PASS, 10 runs.
- `make ci`: PASS.
