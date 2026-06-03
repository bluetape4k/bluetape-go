# Issue 19 Circuit Breaker and Bulkhead Inventory

## Scope

Issue #19 adds circuit breaker and bulkhead policies to the existing
`github.com/bluetape4k/bluetape-go/resilience` package.

The package already has the #18 composition core:

- `Operation[T] func(context.Context) (T, error)`
- `Policy[T]` and `PolicyFunc[T]`
- `Compose[T]` with the first policy as the outermost wrapper
- `Run[T]`
- retry, timeout, errors, and a small synchronous `EventHandler`

## GitHub Evidence

- Epic #2: 0.2.0 resilience primitives for Go services.
- Issue #19: implement circuit breaker and bulkhead primitives.
- Milestone: 0.2.0.
- Assignee: `debop`.

## Local Graph Evidence

- CodeGraph initialized in this worktree: 102 files, 843 nodes, 1,471 edges.
- code-review-graph initialized in this worktree: 99 files, 448 nodes, 3,057 edges.
- CodeGraph context confirmed the integration points are `Run`, `Compose`,
  `Policy`, `PolicyFunc`, `Operation`, `EventHandler`, and `emitEvent`.

## Reference-Only Inputs

- `sony/gobreaker`: state names, counters, transition callbacks, and half-open
  admission concepts.
- `golang.org/x/sync/semaphore`: weighted semaphore behavior as a reference for
  bulkhead admission, not as a runtime dependency for this package.
- resilience4j: service-level circuit breaker and bulkhead concepts, not JVM API
  shape.

## Existing Package Constraints

- No new runtime dependency for circuit breaker or bulkhead.
- Keep generic typed operations.
- Preserve synchronous event hook shape so #21 can add observability without API
  reshaping.
- Context cancellation must not leak permits or half-open slots.
- Tests must be deterministic. Avoid sleep-dependent state transitions where a
  fake clock or explicit function can control behavior.

## Deferred Scope

- HTTP middleware remains #20.
- Full metrics/OpenTelemetry remains #21.
- Rate limiting remains a later milestone unless another 0.2.0 issue adds it.
