# Issue 18 Resilience Core Inventory

## Context

Issue #18 starts milestone `0.2.0` by creating the first-party
`resilience` package. The goal is not to wrap an existing Go library. The goal
is to own a Go-native policy model that can grow across retry, timeout, circuit
breaker, bulkhead, observability hooks, and HTTP examples.

## Source Inventory

Observed source and reference inputs:

- `bluetape4k-projects/infra/resilience4j`: service-level resilience concepts.
- `bluetape4k-projects/ktor/resilience4j`: framework integration examples that
  should later map to `net/http`.
- `failsafe-go`: composable policy shape and executor-oriented API ideas.
- `cenkalti/backoff`: exponential backoff and jitter design ideas.
- `sony/gobreaker`: circuit breaker state and half-open behavior ideas for #19.
- `golang.org/x/sync/semaphore`: bulkhead/concurrency limiter ideas for #19.
- `golang.org/x/time/rate`: future token-bucket rate limiter reference.

## Implement Now

- `Operation[T]`: `context.Context`-aware unit of work.
- `Policy[T]`: composable operation wrapper.
- `Compose` and `Run`: policy application and execution helpers.
- Retry policy:
  - max attempts
  - retryable error predicate
  - pluggable backoff
  - pluggable sleeper for deterministic tests
- Backoff policies:
  - no delay
  - constant delay
  - exponential delay with optional jitter
- Timeout policy:
  - `context.WithTimeout` composition
  - cooperative timeout semantics
- Common errors:
  - retry exhaustion
  - policy-owned timeout
  - wrapped cause preservation through `errors.Is` and `errors.As`
- Event skeleton:
  - success
  - retry
  - timeout

## Defer

- Circuit breaker implementation: #19.
- Bulkhead implementation: #19.
- Full event payload matrix and ordering coverage: #21.
- HTTP middleware and README copy-paste examples: #20.
- OpenTelemetry or metrics exporter dependencies: #21 or later.
- Rate limiter implementation: later milestone unless a `0.2.0` child issue
  explicitly pulls it in.

## Decision

Create a small public `resilience` package with first-party implementation.
External libraries are reference material only. Keep behavior deterministic,
composition explicit, and errors inspectable with standard Go error handling.
