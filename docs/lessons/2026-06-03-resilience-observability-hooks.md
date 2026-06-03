# Lessons Learned - Resilience Observability Hooks (2026-06-03)

Related issue: #21
Affected module: `resilience`

## L1: Stabilize low-cardinality labels before telemetry adapters

### Problem

Retry, timeout, circuit breaker, and bulkhead already emitted some events, but
callers had to treat event kind strings and policy type strings as ad hoc data.
That makes HTTP middleware or future telemetry bridges brittle because they need
stable labels before deciding how to map events to logs, counters, or spans.

### Lesson

For first-party resilience policies, define the policy type, event category, and
error category contract before adding exporters or middleware. Keep exporter
dependencies out of the core package and let `OnEvent` bridge to the service's
existing logging/metrics/tracing stack.

### Evidence

- `resilience/events.go` adds stable policy type, event category, and error
  category constants without changing `EventHandler`.
- `README.md`, `README.ko.md`, and `resilience/doc.go` document synchronous
  handler behavior and the no-exporter boundary.
- `go mod tidy && git diff --exit-code -- go.mod go.sum` confirms no runtime
  dependency was added.

## L2: Review every emission path against a dedicated regression test

### Problem

The first implementation added retry predicate-rejected failure emission, but
the initial test set covered retry ordering and retry exhaustion only. That
would have let a future change remove or mislabel predicate-rejected failures
without failing a focused event test.

### Lesson

When a policy has multiple event-producing branches, Step 6-R should map each
branch to a named regression test. If a branch is implemented but untested, fix
the test gap before PR creation even when the branch is not the headline issue
acceptance case.

### Evidence

- Step 6-R recorded the missing predicate-rejected retry test as a P2 finding.
- `TestRetryPredicateRejectedFailureEmitsFailureEvent` now locks that behavior.
- `go test -count=1 ./resilience`, `go test -race -count=1 ./resilience`, and
  `go test -count=1 ./...` pass after the fix.
