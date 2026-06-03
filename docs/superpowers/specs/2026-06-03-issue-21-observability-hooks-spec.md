# Issue 21 Observability Hooks Spec

## Goal

Stabilize the first-party `resilience` event contract so retry, timeout,
circuit breaker, and bulkhead policies can be connected to logging, counters, or
future telemetry bridges without changing policy composition.

## Constraints

- Do not add OpenTelemetry, logging, metrics, or external observability runtime
  dependencies.
- Keep `EventHandler func(context.Context, Event)` synchronous and predictable.
- Preserve source compatibility for existing `Event` struct literals and
  existing option structs.
- Keep policy behavior first-party and Go-native.
- Event handlers must not run while a circuit breaker lock is held.

## Current Behavior

- Retry emits `EventRetry` before a retry sleep and `EventSuccess` on eventual
  success.
- Timeout emits `EventTimeout` only for the timeout policy's own deadline, and
  `EventSuccess` on success.
- Circuit breaker emits state transition and rejection events.
- Bulkhead emits admission, rejection, and success events.
- Existing event payloads do not include stable policy type constants, event
  categories, timeout duration, or normalized error category data.

## Approach Comparison

### Approach A - Extend Event with Stable Constants and Small Payload Fields

- Add policy type constants for retry, timeout, circuit breaker, and bulkhead.
- Add event category constants for success, retry, timeout, rejection,
  transition, and failure/exhaustion where relevant.
- Add `Category`, `ErrorCategory`, and `Timeout` fields to `Event`.
- Continue using the existing per-policy `OnEvent` option.
- Add event ordering tests for representative flows.

Decision: choose this approach.

Why: it keeps the existing public shape, gives #20 stable labels, avoids vendor
dependencies, and does not require a global registry.

### Approach B - Add Observer Interface per Policy

- Replace or supplement `EventHandler` with an interface such as
  `OnRetry`, `OnTimeout`, `OnReject`, and `OnStateTransition`.

Rejected: this provides stronger typing per policy, but it is heavier for Go
users, requires more option fields, and makes simple logging/metric bridging
more verbose.

### Approach C - Add Global Event Bus

- Add a package-level registry or bus that receives all policy events.

Rejected: global state is harder to test, harder to isolate across services,
and unnecessary for #20 HTTP middleware examples. Callers can compose their own
fan-out handler when needed.

## Event Contract

- `EventHandler` remains synchronous.
- `emitEvent` calls the handler only when it is non-nil.
- Policy implementations may call handlers after internal state is updated, but
  must not hold internal locks while invoking handler code.
- `Event.PolicyType` uses stable string constants:
  - `PolicyTypeRetry`
  - `PolicyTypeTimeout`
  - `PolicyTypeCircuitBreaker`
  - `PolicyTypeBulkhead`
- `Event.Category` describes the event's broad observability meaning:
  - success
  - retry
  - admission
  - timeout
  - rejection
  - transition
  - failure
- `Event.ErrorCategory` is set when `Err` is set and should be stable enough for
  logs/metric labels:
  - timeout
  - circuit_open
  - bulkhead_rejected
  - retry_exhausted
  - context_canceled
  - context_deadline
  - failure
- `Event.Timeout` is set for timeout events.
- Existing fields remain valid:
  - `Attempt` for retry attempt data
  - `Delay` for retry backoff
  - `State` and `PreviousState` for circuit events
  - `InFlight` for bulkhead events
  - `Err` for retry, timeout, rejection, and failure/exhaustion events

## Policy Event Expectations

### Retry

- On retryable failure before the final attempt:
  - emit `EventRetry`
  - set policy type, category retry, attempt, delay, err, error category
- On success:
  - emit `EventSuccess`
  - set policy type, category success, attempt
- On max-attempt exhaustion:
  - emit a failure event with `RetryError`
  - keep `errors.Is(err, ErrRetryExhausted)` behavior unchanged
- On predicate-rejected error:
  - emit a failure event with the operation error

### Timeout

- On success:
  - emit `EventSuccess` with category success
- On policy-owned deadline:
  - emit `EventTimeout`
  - set timeout duration, err, and error category timeout
- Parent cancellation/deadline must not be mislabeled as this policy's timeout.

### Circuit Breaker

- On state transition:
  - emit `EventCircuitStateTransition`
  - set category transition, previous state, and next state
- On circuit rejection:
  - emit `EventCircuitRejected`
  - set category rejection, state, in-flight when relevant, err, and error
    category circuit_open

### Bulkhead

- On admission:
  - emit `EventBulkheadAccepted`
  - set category admission and in-flight count
- On rejection:
  - emit `EventBulkheadRejected`
  - set category rejection, in-flight count, err, and error category
    bulkhead_rejected
- On success:
  - emit `EventSuccess`
  - set policy type, category success, in-flight count

## Test Contract

- Retry ordering:
  - retry event for attempt 1
  - success event for attempt 2
  - payload includes policy name/type, category, attempt, delay, and error
    category
- Retry exhaustion:
  - failure event is emitted before returning `RetryError`
  - `errors.Is(err, ErrRetryExhausted)` still works
- Timeout:
  - timeout event includes timeout duration and timeout error category
  - parent cancellation still emits no timeout event
- Circuit breaker:
  - failed call emits closed -> open transition
  - immediate rejected call emits circuit rejection
  - after open timeout, next call emits open -> half-open transition, then
    half-open -> closed transition on success
- Bulkhead:
  - admitted first call emits accepted
  - rejected concurrent call emits bulkhead rejection
  - successful admitted call emits success

## Documentation Contract

- Package docs explain how to bridge `EventHandler` to logging or metrics.
- README locale pair mentions that resilience policies expose synchronous events
  for logging/metrics/telemetry bridges.
- Docs must state that #21 does not add a telemetry vendor dependency.

## DoD

- Research/spec/plan/review/lesson artifacts are committed.
- Event contract is extended without adding runtime dependencies.
- Tests cover event ordering and payloads for retry, timeout, circuit breaker,
  and bulkhead.
- `go test -count=1 ./resilience`, `go test -race -count=1 ./resilience`,
  `go test -count=1 ./...`, `go vet ./...`, raw
  `golangci-lint run ./...`, `make fmt-check`, tidy diff check, and
  `git diff --check` pass.
- PR is created with milestone `0.2.0`, assignee `debop`, linked issue #21, and
  DoD as the final PR body section.
