# Issue 19 Circuit Breaker and Bulkhead Spec

## Goal

Add first-party circuit breaker and bulkhead policies that plug into the #18
`resilience` policy composition model.

## Constraints

- Do not wrap or depend on `sony/gobreaker`, `golang.org/x/sync/semaphore`, or
  resilience4j for runtime behavior.
- Public API must stay Go-native and context-aware.
- Circuit breaker and bulkhead must be generic `Policy[T]` implementations.
- Event hooks must reuse the existing `EventHandler` skeleton and remain
  compatible with #21 observability expansion.
- Tests must avoid flaky timing.

## Circuit Breaker Contract

- Public states: closed, open, half-open.
- A new circuit breaker starts closed.
- Closed state:
  - admits calls
  - records success/failure counters
  - opens when a configurable failure predicate and threshold decide the circuit
    is unhealthy
- Open state:
  - rejects calls without executing the operation
  - returns a circuit-open error compatible with `errors.Is`
  - moves to half-open only after the configured open interval has elapsed
- Half-open state:
  - admits only a bounded number of probe calls
  - rejects excess probes
  - closes after enough successful probes
  - reopens immediately on a probe failure
- Failure classification is configurable by `FailureIf`.
- State transition and rejection events are emitted through `EventHandler`.
- State reads are concurrency-safe.

## Bulkhead Contract

- A bulkhead limits concurrent operation execution.
- `MaxConcurrent` must be positive.
- When no permit is available:
  - reject immediately by default
  - optionally wait when configured to do so
- Waiting for a permit must respect `context.Context` cancellation.
- Permits must always be released after operation completion, panic-free paths,
  context cancellation, and operation errors.
- Rejection returns a bulkhead-rejected error compatible with `errors.Is`.
- Admission, rejection, and success/failure events are emitted through
  `EventHandler`.

## Error Contract

- Add sentinel errors for circuit-open and bulkhead-rejected behavior.
- Typed errors must preserve policy name and cause where applicable.
- `errors.Is` must work for the new sentinel errors.

## Event Contract

Add event kinds for:

- circuit breaker state transition
- circuit breaker rejection
- bulkhead admission
- bulkhead rejection

Existing event fields remain valid. New fields may be added only when needed and
must not break #18 callers.

## Test Contract

- Circuit breaker starts closed and records success.
- Circuit breaker opens after the configured failure threshold.
- Open circuit rejects calls without invoking the operation.
- Fake clock or injected `Now` function controls open-to-half-open transition.
- Half-open success closes the circuit.
- Half-open failure reopens the circuit.
- Half-open permits are bounded under concurrent calls.
- Bulkhead rejects calls beyond `MaxConcurrent` by default.
- Bulkhead can wait for a permit and obeys context cancellation.
- Bulkhead releases permits after success and failure.
- Circuit breaker and bulkhead compose with retry/timeout through `Run`.
