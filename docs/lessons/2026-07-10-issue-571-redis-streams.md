# Redis Streams Primitive: Command Sharing Without Delivery Policy

## Context

Issue #571 added `redis/stream` and migrated only the `XADD` dispatch inside
`audit/sqloutbox/redisstreams`. The provider already owned a stable audit
envelope and duplicate-attempt behavior; the shared package needed to remove
only repeated command safety behavior.

## Lesson

For a provider-backed Redis Streams feature, share command mechanics but keep
delivery policy and domain data at the provider or application boundary.

- The primitive owns narrow command interfaces, argument validation, nil/typed
  nil detection, context preflight, and redacted typed errors.
- The provider owns record fields, payload encoding, default stream selection,
  idempotency identity, relay retry/dead-letter policy, and its public API.
- The application owns consumer groups, idle thresholds, `XAUTOCLAIM` cursor
  persistence, replay, retention, and consumer shutdown.

This avoids a generic message-bus abstraction while still giving every caller a
consistent cancellation and error contract.

## Cancellation Rule

Redis blocking reads can complete with a Redis nil-style result after a caller
deadline is already done. Do not replace that provider cause with the context
cause. Join both before wrapping in the redacted operational error so callers
can use `errors.Is` for either the provider result or
`context.DeadlineExceeded`/`context.Canceled`.

## Test Isolation Rule

Consumer groups are stored under the stream key. A static test stream name can
cross-contaminate normal and race test processes even when cleanup is present.
Use a unique suffix per Testcontainers fixture invocation and clean only that
test-owned stream key.

## Benchmark Boundary

This issue adds command behavior and provider reuse, not a throughput claim.
No benchmark is appropriate here. Issue #560 owns any provider comparison and
must publish the required result table, chart, and written analysis when it
runs measurements.
